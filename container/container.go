package container

import (
	"cont/tty"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
)

const (
	hostEnv        = "HOST"
	workdirEnv     = "WORKDIR"
	interactiveEnv = "INTERACTIVE"
)

type Config struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	Hostname       string
	Workdir        string
	Cmd            string
	Args           []string
	Interactive    bool
}

func Start(ctx context.Context, config *Config) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, "/proc/self/exe", append([]string{config.Cmd}, config.Args...)...)
	cmd.Stdin = config.Stdin
	cmd.Stdout = config.Stdout
	cmd.Stderr = config.Stderr

	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env,
		fmt.Sprintf(hostEnv+"=%s", config.Hostname),
		fmt.Sprintf(workdirEnv+"=%s", config.Workdir))
	if config.Interactive {
		cmd.Env = append(cmd.Env, fmt.Sprintf(interactiveEnv+"=%t", config.Interactive))
		pty, err := tty.OpenPTY() // todo: close PTY on container exit
		if err != nil {
			return nil, err
		}
		cmd.Stdin = pty.Slave
		cmd.Stdout = pty.Slave
		cmd.Stderr = pty.Slave

		go io.Copy(pty.Master, config.Stdin)
		go io.Copy(config.Stdout, pty.Master)
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET | syscall.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getuid(), Size: 1},
		},
		GidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getgid(), Size: 1},
		},
	}
	return cmd, cmd.Start()
}

func Run(ctx context.Context, config *Config) (*exec.Cmd, error) {
	cmd, err := Start(ctx, config)
	if err != nil {
		return cmd, err
	}
	return cmd, cmd.Wait()
}

func RunChild() error {
	cmd := exec.Command(os.Args[1], os.Args[2:]...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := syscall.Sethostname([]byte(os.Getenv(hostEnv))); err != nil {
		return err
	}
	if err := os.Chdir("/"); err != nil {
		return err
	}
	if err := syscall.Mount("proc", "proc", "proc", 0, ""); err != nil {
		return err
	}

	if err := os.Chdir(os.Getenv(workdirEnv)); err != nil {
		return err
	}

	if os.Getenv(interactiveEnv) != "" {
		pty, err := tty.Start(cmd, os.Stdin, os.Stdout)
		if err != nil {
			return fmt.Errorf("cannot start TTY: %w", err)
		}
		defer pty.Close()
		return cmd.Wait()
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
