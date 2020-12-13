package cont

import (
	"github.com/google/uuid"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
)

func PipePath(id uuid.UUID) string {
	dir := os.TempDir()
	return filepath.Join(dir, id.String())
}

func pipeNames(path string) (inPath, outPath, errPath string) {
	inPath = path + ".in"
	outPath = path + ".out"
	errPath = path + ".err"
	return inPath, outPath, errPath
}

func CreatePipes(path string) error {
	inPath, outPath, errPath := pipeNames(path)
	if err := unix.Mkfifo(outPath, 0666); err != nil {
		return err
	}
	if err := unix.Mkfifo(inPath, 0666); err != nil {
		return err
	}
	if err := unix.Mkfifo(errPath, 0666); err != nil {
		return err
	}
	return nil
}

func RemovePipes(path string) error {
	inPath, outPath, errPath := pipeNames(path)
	if err := os.Remove(inPath); err != nil {
		return err
	}
	if err := os.Remove(outPath); err != nil {
		return err
	}
	if err := os.Remove(errPath); err != nil {
		return err
	}
	return nil
}

func OpenPipes(path string) (files [3]*os.File, err error) {
	inPath, outPath, errPath := pipeNames(path)
	inPipe, err := os.OpenFile(inPath, os.O_RDWR, os.ModeNamedPipe)
	if err != nil {
		return files, err
	}
	outPipe, err := os.OpenFile(outPath, os.O_RDWR, os.ModeNamedPipe)
	if err != nil {
		return files, err
	}
	errPipe, err := os.OpenFile(errPath, os.O_RDWR, os.ModeNamedPipe)
	if err != nil {
		return files, err
	}
	files[0] = inPipe
	files[1] = outPipe
	files[2] = errPipe
	return files, err
}
