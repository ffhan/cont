package container

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func setupLogging(cmd *exec.Cmd, config *Config) error {
	path := config.Logging.Path

	if err := os.MkdirAll(path, 0774); err != nil {
		return fmt.Errorf("cannot create logging directory: %w", err)
	}
	log, err := os.OpenFile(filepath.Join(path, "logs.log"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("cannot create log file: %w", err)
	}
	cmd.Stdout = io.MultiWriter(cmd.Stdout, log)
	cmd.Stderr = io.MultiWriter(cmd.Stderr, log)

	return nil
}
