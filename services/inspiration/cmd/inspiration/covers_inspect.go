package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"omnimux-inspiration/internal/config"
	"omnimux-inspiration/internal/errs"
	"omnimux-inspiration/internal/media"
)

const coverInspectSchemaVersion = 1

type coverInspectManifestItem struct {
	ID  string `json:"id"`
	Ref string `json:"ref"`
}

type coverInspectManifest struct {
	Items []coverInspectManifestItem `json:"items"`
}

type coverInspectItem struct {
	ID          string `json:"id"`
	Ref         string `json:"ref"`
	Status      string `json:"status"`
	SHA256      string `json:"sha256,omitempty"`
	Bytes       int64  `json:"bytes,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	ErrorCode   string `json:"errorCode,omitempty"`
}

type coverInspection struct {
	SchemaVersion int                `json:"schemaVersion"`
	Items         []coverInspectItem `json:"items"`
}

type coverInspectReader interface {
	Read(context.Context, string) (*media.CoverImage, error)
}

type coverInspectReaderFactory func(string) (coverInspectReader, error)

func coversInspectCmd() *cobra.Command {
	return coversInspectCmdWithReader(newCoverInspectReader)
}

func coversInspectCmdWithReader(newReader coverInspectReaderFactory) *cobra.Command {
	var manifestPath, outputPath, envFile string
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "只读检查任意 R2 封面引用",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			manifest, err := readCoverInspectManifest(manifestPath)
			if err != nil {
				return err
			}

			reader, readerErr := newReader(envFile)
			if readerErr != nil {
				reader = unavailableCoverInspectReader{}
			}
			inspection, inspectErr := inspectCovers(cmd.Context(), manifest, reader)
			if err := writeSecureJSON(outputPath, inspection); err != nil {
				return fmt.Errorf("save inspection output: %w", err)
			}
			if readerErr != nil {
				return errors.New("cover reader configuration unavailable")
			}
			if inspectErr != nil {
				return inspectErr
			}
			fmt.Fprintf(cmd.OutOrStdout(), "inspected=%d output=%s\n", len(inspection.Items), outputPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&manifestPath, "manifest", "", "1至200条来源ID与R2引用的JSON清单")
	cmd.Flags().StringVar(&outputPath, "out", "", "0600检查结果输出路径")
	cmd.Flags().StringVar(&envFile, "env-file", "", "可选R2只读配置文件")
	requireFlags(cmd, "manifest", "out")
	return cmd
}

func newCoverInspectReader(envFile string) (coverInspectReader, error) {
	if strings.TrimSpace(envFile) != "" {
		if _, err := godotenv.Read(envFile); err != nil {
			return nil, errors.New("cover reader env file is missing or invalid")
		}
	}
	cfg := config.Load("", envFile)
	return media.NewCoverReader(cfg)
}

type unavailableCoverInspectReader struct{}

func (unavailableCoverInspectReader) Read(context.Context, string) (*media.CoverImage, error) {
	return nil, media.ErrCoverUnavailable
}

func inspectCovers(ctx context.Context, manifest *coverInspectManifest, reader coverInspectReader) (*coverInspection, error) {
	if err := validateCoverInspectManifest(manifest); err != nil {
		return nil, err
	}

	out := &coverInspection{SchemaVersion: coverInspectSchemaVersion, Items: make([]coverInspectItem, 0, len(manifest.Items))}
	for _, source := range manifest.Items {
		item := coverInspectItem{ID: source.ID, Ref: source.Ref}
		if _, err := media.ParseCoverRef(source.Ref); err != nil {
			item.Status = "invalid"
			item.ErrorCode = errs.ErrInvalidParam.Code
			out.Items = append(out.Items, item)
			continue
		}

		var image *media.CoverImage
		var err error
		if reader == nil {
			err = media.ErrCoverUnavailable
		} else {
			image, err = reader.Read(ctx, source.Ref)
		}
		if err != nil {
			status, code, fatal := classifyCoverInspectError(err)
			item.Status = status
			item.ErrorCode = code
			out.Items = append(out.Items, item)
			if fatal {
				return out, fmt.Errorf("cover inspection stopped at id %q: %s", source.ID, code)
			}
			continue
		}
		if image == nil {
			item.Status = "unavailable"
			item.ErrorCode = media.ErrCoverUnavailable.Code
			out.Items = append(out.Items, item)
			return out, fmt.Errorf("cover inspection stopped at id %q: %s", source.ID, item.ErrorCode)
		}

		digest := sha256.Sum256(image.Data)
		item.Status = "ok"
		item.SHA256 = hex.EncodeToString(digest[:])
		item.Bytes = int64(len(image.Data))
		item.ContentType = image.ContentType
		out.Items = append(out.Items, item)
	}
	return out, nil
}

func classifyCoverInspectError(err error) (status, code string, fatal bool) {
	switch {
	case errors.Is(err, errs.ErrInvalidParam):
		return "invalid", errs.ErrInvalidParam.Code, false
	case errors.Is(err, media.ErrCoverNotFound):
		return "invalid", media.ErrCoverNotFound.Code, false
	case errors.Is(err, media.ErrCoverInvalid):
		return "invalid", media.ErrCoverInvalid.Code, false
	case errors.Is(err, media.ErrCoverTimeout):
		return "unavailable", media.ErrCoverTimeout.Code, true
	default:
		return "unavailable", media.ErrCoverUnavailable.Code, true
	}
}

func validateCoverInspectManifest(manifest *coverInspectManifest) error {
	if manifest == nil || len(manifest.Items) < 1 || len(manifest.Items) > 200 {
		return errors.New("cover inspection manifest must contain 1 to 200 items")
	}
	seen := make(map[string]struct{}, len(manifest.Items))
	for i, item := range manifest.Items {
		if strings.TrimSpace(item.ID) == "" {
			return fmt.Errorf("cover inspection item %d has an empty id", i)
		}
		if _, exists := seen[item.ID]; exists {
			return fmt.Errorf("duplicate cover inspection id %q", item.ID)
		}
		seen[item.ID] = struct{}{}
	}
	return nil
}

func readCoverInspectManifest(path string) (*coverInspectManifest, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("manifest path is required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	type wireItem struct {
		ID  *string `json:"id"`
		Ref *string `json:"ref"`
	}
	type wireManifest struct {
		Items *[]wireItem `json:"items"`
	}
	var wire wireManifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if wire.Items == nil {
		return nil, errors.New("cover inspection manifest items must be an array")
	}
	manifest := &coverInspectManifest{Items: make([]coverInspectManifestItem, len(*wire.Items))}
	for i, item := range *wire.Items {
		if item.ID == nil || item.Ref == nil {
			return nil, fmt.Errorf("cover inspection item %d requires string id and ref", i)
		}
		manifest.Items[i] = coverInspectManifestItem{ID: *item.ID, Ref: *item.Ref}
	}
	if err := validateCoverInspectManifest(manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("multiple JSON values are not allowed")
}
