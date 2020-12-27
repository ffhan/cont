package container

import (
	"cont/tty"
	"context"
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"os/exec"
	"syscall"
)

func Start(ctx context.Context, config *Config) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, "/proc/self/exe", append([]string{"init", config.Cmd}, config.Args...)...)
	cmd.Stdin = config.Stdin
	cmd.Stdout = config.Stdout
	cmd.Stderr = config.Stderr

	if err := setupLogging(cmd, config); err != nil {
		return nil, fmt.Errorf("cannot setup logging: %w", err)
	}

	cmd.Env = os.Environ() // todo: shouldn't share environ?
	if err := setupEnv(cmd, config); err != nil {
		return nil, err
	}

	if config.Interactive {
		pty, err := tty.OpenPTY() // todo: close PTY on container exit
		if err != nil {
			return nil, err
		}
		stdout := cmd.Stdout // setupLogging set up stdout

		// attach to PTY
		cmd.Stdin = pty.Slave
		cmd.Stdout = pty.Slave
		cmd.Stderr = pty.Slave

		go io.Copy(pty.Master, config.Stdin)
		go io.Copy(stdout, pty.Master) // write output to previously set up logging
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS,
		UidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getuid(), Size: 1},
		},
		GidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getgid(), Size: 1},
		},
	}

	if config.SharedNamespaceConfig.Flags == 0 { // we're not sharing anything
		cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET | syscall.CLONE_NEWUSER | syscall.CLONE_NEWIPC | unix.CLONE_NEWCGROUP
	} else {
		if err := setupSharedNSes(cmd, config); err != nil {
			return nil, err
		}
	}

	return cmd, cmd.Start()
}

func setupSharedNSes(cmd *exec.Cmd, config *Config) error {
	nses, err := getNses(config.SharedNamespaceConfig)
	if err != nil {
		return err
	}
	nsStartFd := 3 + len(cmd.ExtraFiles)
	nsEndFd := nsStartFd + len(nses)

	cmd.Env = append(cmd.Env,
		fmt.Sprintf(nsStartEnv+"=%d", nsStartFd),
		fmt.Sprintf(nsEndEnv+"=%d", nsEndFd),
	)
	if cmd.ExtraFiles == nil {
		cmd.ExtraFiles = make([]*os.File, 0, len(nses))
	}
	cmd.ExtraFiles = append(cmd.ExtraFiles, nses...)
	return nil
}

func Run(ctx context.Context, config *Config) (*exec.Cmd, error) {
	cmd, err := Start(ctx, config)
	if err != nil {
		return cmd, err
	}
	return cmd, cmd.Wait()
}
