package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"omnimux-inspiration/internal/config"
	"omnimux-inspiration/internal/media"
	"omnimux-inspiration/internal/service"
	"omnimux-inspiration/internal/store"
)

type mediaCoverReader struct{ reader *media.CoverReader }

func (r mediaCoverReader) Read(ctx context.Context, ref string) (*service.CoverObject, error) {
	image, err := r.reader.Read(ctx, ref)
	if err != nil {
		return nil, err
	}
	return &service.CoverObject{
		Data: image.Data, ContentType: image.ContentType, ETag: image.ETag, LastModified: image.LastModified,
	}, nil
}

func coversCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "covers", Short: "审阅 R2 封面并事务修复固定封面引用"}
	cmd.AddCommand(coversInspectCmd(), coversPlanCmd(), coversVerifyCmd(), coversApplyCmd(), coversStatusCmd(), coversRollbackCmd())
	return cmd
}

func coversPlanCmd() *cobra.Command {
	var manifestPath, outputPath, dsn, operationID string
	cmd := &cobra.Command{
		Use: "plan", Short: "生成只读审阅计划",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var manifest service.CoverRepairManifest
			if err := readJSONFile(manifestPath, &manifest); err != nil {
				return err
			}
			db, err := openRepairDB(dsn)
			if err != nil {
				return err
			}
			plan, err := service.BuildCoverRepairPlan(cmd.Context(), db, operationID, manifest.Items)
			if err != nil {
				return err
			}
			if err := writeSecureJSON(outputPath, plan); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "plan=%s digest=%s\n", outputPath, plan.Digest)
			return nil
		},
	}
	cmd.Flags().StringVar(&manifestPath, "manifest", "", "固定10条配对清单 JSON")
	cmd.Flags().StringVar(&outputPath, "out", "", "0600计划输出路径")
	cmd.Flags().StringVar(&dsn, "dsn", "", "显式MySQL DSN")
	cmd.Flags().StringVar(&operationID, "operation-id", "", "唯一操作ID")
	requireFlags(cmd, "manifest", "out", "dsn", "operation-id")
	return cmd
}

func coversVerifyCmd() *cobra.Command {
	var planPath, digest, outputPath, envFile string
	cmd := &cobra.Command{
		Use: "verify", Short: "在事务外验证固定R2对象",
		RunE: func(cmd *cobra.Command, _ []string) error {
			plan, err := loadPlan(planPath, digest)
			if err != nil {
				return err
			}
			reader, err := newMediaCoverReader(envFile)
			if err != nil {
				return err
			}
			verification, err := service.VerifyCoverRepairPlan(cmd.Context(), plan, reader)
			if err != nil {
				return err
			}
			if err := writeSecureJSON(outputPath, verification); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "verified=%d evidence=%s\n", len(verification.Items), outputPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&planPath, "plan", "", "正式计划文件")
	cmd.Flags().StringVar(&digest, "digest", "", "计划SHA-256摘要")
	cmd.Flags().StringVar(&outputPath, "out", "", "0600验证证据输出路径")
	cmd.Flags().StringVar(&envFile, "env-file", "", "R2只读配置文件")
	requireFlags(cmd, "plan", "digest", "out")
	return cmd
}

func coversApplyCmd() *cobra.Command {
	var planPath, digest, dsn, receiptPath, verificationPath, envFile string
	cmd := &cobra.Command{
		Use: "apply", Short: "验证对象并原子提交10条封面引用",
		RunE: func(cmd *cobra.Command, _ []string) error {
			plan, err := loadPlan(planPath, digest)
			if err != nil {
				return err
			}
			reader, err := newMediaCoverReader(envFile)
			if err != nil {
				return err
			}
			verification, err := service.VerifyCoverRepairPlan(cmd.Context(), plan, reader)
			if err != nil {
				return err
			}
			if err := writeSecureJSON(verificationPath, verification); err != nil {
				return err
			}
			db, err := openRepairDB(dsn)
			if err != nil {
				return err
			}
			output, err := reserveReceiptOutput(receiptPath, plan.OperationID, plan.Digest)
			if err != nil {
				return err
			}
			receipt, applyErr := service.ApplyCoverRepair(cmd.Context(), db, plan)
			if receipt != nil {
				if err := output.save(receipt); err != nil {
					return errors.Join(applyErr, fmt.Errorf("database state=%s; receipt not saved: %w; run covers status with the same plan and target before any retry", receipt.State, err))
				}
			}
			if applyErr != nil {
				return applyErr
			}
			fmt.Fprintf(cmd.OutOrStdout(), "state=%s receipt=%s digest=%s\n", receipt.State, receiptPath, receipt.Digest)
			return nil
		},
	}
	cmd.Flags().StringVar(&planPath, "plan", "", "正式计划文件")
	cmd.Flags().StringVar(&digest, "digest", "", "计划SHA-256摘要")
	cmd.Flags().StringVar(&dsn, "dsn", "", "显式MySQL DSN")
	cmd.Flags().StringVar(&receiptPath, "receipt", "", "0600 receipt输出路径")
	cmd.Flags().StringVar(&verificationPath, "verification", "", "0600对象验证证据路径")
	cmd.Flags().StringVar(&envFile, "env-file", "", "R2只读配置文件")
	requireFlags(cmd, "plan", "digest", "dsn", "receipt", "verification")
	return cmd
}

func coversStatusCmd() *cobra.Command {
	var planPath, digest, dsn, outputPath string
	cmd := &cobra.Command{
		Use: "status", Short: "判定完整before、完整after或冲突状态",
		RunE: func(cmd *cobra.Command, _ []string) error {
			plan, err := loadPlan(planPath, digest)
			if err != nil {
				return err
			}
			db, err := openRepairDB(dsn)
			if err != nil {
				return err
			}
			status, err := service.StatusCoverRepair(cmd.Context(), db, plan)
			if err != nil {
				return err
			}
			if outputPath != "" {
				return writeSecureJSON(outputPath, status)
			}
			return writeJSON(cmd.OutOrStdout(), status)
		},
	}
	cmd.Flags().StringVar(&planPath, "plan", "", "正式计划文件")
	cmd.Flags().StringVar(&digest, "digest", "", "计划SHA-256摘要")
	cmd.Flags().StringVar(&dsn, "dsn", "", "显式MySQL DSN")
	cmd.Flags().StringVar(&outputPath, "out", "", "可选0600状态输出路径")
	requireFlags(cmd, "plan", "digest", "dsn")
	return cmd
}

func coversRollbackCmd() *cobra.Command {
	var planPath, planDigest, receiptPath, suppliedReceiptDigest, dsn, outputPath string
	cmd := &cobra.Command{
		Use: "rollback", Short: "条件回滚完整after状态",
		RunE: func(cmd *cobra.Command, _ []string) error {
			plan, err := loadPlan(planPath, planDigest)
			if err != nil {
				return err
			}
			var receipt service.CoverRepairReceipt
			if err := readJSONFile(receiptPath, &receipt); err != nil {
				return err
			}
			if err := service.CheckReceiptDigest(&receipt, plan, suppliedReceiptDigest); err != nil {
				return err
			}
			db, err := openRepairDB(dsn)
			if err != nil {
				return err
			}
			output, err := reserveReceiptOutput(outputPath, plan.OperationID, plan.Digest)
			if err != nil {
				return err
			}
			rollbackReceipt, rollbackErr := service.RollbackCoverRepair(cmd.Context(), db, plan, &receipt)
			if rollbackReceipt != nil {
				if err := output.save(rollbackReceipt); err != nil {
					return errors.Join(rollbackErr, fmt.Errorf("database state=%s; receipt not saved: %w; run covers status with the same plan and target before any retry", rollbackReceipt.State, err))
				}
			}
			if rollbackErr != nil {
				return rollbackErr
			}
			fmt.Fprintf(cmd.OutOrStdout(), "state=%s receipt=%s digest=%s\n", rollbackReceipt.State, outputPath, rollbackReceipt.Digest)
			return nil
		},
	}
	cmd.Flags().StringVar(&planPath, "plan", "", "正式计划文件")
	cmd.Flags().StringVar(&planDigest, "digest", "", "计划SHA-256摘要")
	cmd.Flags().StringVar(&receiptPath, "receipt", "", "apply receipt文件")
	cmd.Flags().StringVar(&suppliedReceiptDigest, "receipt-digest", "", "receipt SHA-256摘要")
	cmd.Flags().StringVar(&dsn, "dsn", "", "显式MySQL DSN")
	cmd.Flags().StringVar(&outputPath, "out", "", "0600 rollback receipt输出路径")
	requireFlags(cmd, "plan", "digest", "receipt", "receipt-digest", "dsn", "out")
	return cmd
}

func newMediaCoverReader(envFile string) (service.CoverObjectReader, error) {
	cfg := config.Load("", envFile)
	reader, err := media.NewCoverReader(cfg)
	if err != nil {
		return nil, err
	}
	return mediaCoverReader{reader: reader}, nil
}

func openRepairDB(dsn string) (*store.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("explicit --dsn is required")
	}
	return store.Open(dsn)
}

func loadPlan(path, digest string) (*service.CoverRepairPlan, error) {
	var plan service.CoverRepairPlan
	if err := readJSONFile(path, &plan); err != nil {
		return nil, err
	}
	if err := service.CheckPlanDigest(&plan, digest); err != nil {
		return nil, err
	}
	return &plan, nil
}

func readJSONFile(path string, target any) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("JSON file path is required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

type receiptOutput struct {
	path        string
	reservation os.FileInfo
}

// Reserve the receipt before starting a transaction. PREPARED is never proof of a commit.
func reserveReceiptOutput(path, operationID, planDigest string) (*receiptOutput, error) {
	if err := writeSecureJSON(path, map[string]string{
		"state": "PREPARED", "operationId": operationID, "planDigest": planDigest,
		"recovery": "Run covers status with the same plan and target; this file is not a commit receipt.",
	}); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	return &receiptOutput{path: path, reservation: info}, nil
}

func (o *receiptOutput) save(value any) error {
	return publishSecureJSON(o.path, value, o.reservation)
}

func writeSecureJSON(path string, value any) error {
	return publishSecureJSON(path, value, nil)
}

func publishSecureJSON(path string, value any, reservation os.FileInfo) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("output path is required")
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err == nil {
		raw = append(raw, '\n')
		_, err = tmp.Write(raw)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if reservation == nil {
		// Link publishes atomically and fails if any file or symlink already exists.
		if err := os.Link(tmpPath, path); err != nil {
			return err
		}
	} else {
		current, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !os.SameFile(current, reservation) {
			return fmt.Errorf("receipt reservation changed: %s", path)
		}
		if err := os.Rename(tmpPath, path); err != nil {
			return err
		}
	}
	dirFile, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer dirFile.Close()
	return dirFile.Sync()
}

func writeJSON(out interface{ Write([]byte) (int, error) }, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func requireFlags(cmd *cobra.Command, names ...string) {
	for _, name := range names {
		_ = cmd.MarkFlagRequired(name)
	}
}
