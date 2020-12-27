package container

import (
	"cont/tty"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func countOpenFiles() int64 {
	out, err := exec.Command("/bin/sh", "-c", fmt.Sprintf("lsof -p %v", os.Getpid())).Output()
	if err != nil {
		fmt.Println(err.Error())
	}
	lines := strings.Split(string(out), "\n")
	return int64(len(lines) - 1)
}

func RunChild() error {
	cmd := exec.Command(os.Args[2], os.Args[3:]...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	env, err := getEnv()
	if err != nil {
		return fmt.Errorf("cannot get environment from init pipe: %w", err)
	}

	//if env.SharedNamespaceConfig.Share {
	//	attachToNSes()
	//}

	if err := syscall.Sethostname([]byte(env.Hostname)); err != nil {
		return fmt.Errorf("cannot set hostname \"%s\": %w", env.Hostname, err)
	}
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("cannot chdir to root: %w", err)
	}
	if err := syscall.Mount("proc", "proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("cannot mount new proc fs: %w", err)
	}

	if err := os.Chdir(env.Workdir); err != nil {
		return fmt.Errorf("cannot chdir to \"%s\": %w", env.Workdir, err)
	}

	isInteractive := env.Interactive

	if isInteractive {
		pty, err := tty.Start(cmd, os.Stdin, os.Stdout)
		if err != nil {
			return fmt.Errorf("cannot start TTY: %w", err)
		}
		defer pty.Close()
		err = cmd.Wait()
		if err != nil {
			return fmt.Errorf("wait failed: %w", err)
		}
		return nil
	} else {
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	if err := os.Chdir("/"); err != nil {
		return err
	}
	if err := syscall.Unmount("proc", 0); err != nil {
		return err
	}
	return nil
}
