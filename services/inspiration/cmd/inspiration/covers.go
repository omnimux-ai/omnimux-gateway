package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func coversCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "covers", Short: "只读检查 R2 封面引用"}
	cmd.AddCommand(coversInspectCmd())
	return cmd
}

func writeSecureJSON(path string, value any) error {
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
	// Link publishes atomically and fails if any file or symlink already exists.
	if err := os.Link(tmpPath, path); err != nil {
		return err
	}
	dirFile, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer dirFile.Close()
	return dirFile.Sync()
}

func requireFlags(cmd *cobra.Command, names ...string) {
	for _, name := range names {
		_ = cmd.MarkFlagRequired(name)
	}
}
