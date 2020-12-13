package container

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
)

const (
	hostEnv    = "HOST"
	workdirEnv = "WORKDIR"
)

type Config struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	Hostname       string
	Workdir        string
	Args           []string
}

// todo: output should be protected data
func Run(config *Config) error {
	cmd := exec.Command("/proc/self/exe", config.Args...)
	cmd.Stdin = config.Stdin
	cmd.Stdout = config.Stdout
	cmd.Stderr = config.Stderr

	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env,
		fmt.Sprintf(hostEnv+"=%s", config.Hostname),
		fmt.Sprintf(workdirEnv+"=%s", config.Workdir))

	cmd.SysProcAttr = &syscall.SysProcAttr{ // todo: isolate users, rootless container
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET | syscall.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getuid(), Size: 1},
		},
		GidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getgid(), Size: 1},
		},
	}
	return cmd.Run()
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

	if err := cmd.Run(); err != nil {
		return err
	}

	if err := os.Chdir("/"); err != nil {
		return err
	}
	if err := syscall.Unmount("proc", 0); err != nil {
		return err
	}
	return nil
}
