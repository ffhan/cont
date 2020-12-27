package container

import (
	"encoding/gob"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

func setupEnv(cmd *exec.Cmd, config *Config) error {
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	defer w.Close()
	if cmd.ExtraFiles == nil {
		cmd.ExtraFiles = make([]*os.File, 0, 1)
	}
	cmd.ExtraFiles = append(cmd.ExtraFiles, r)
	cmd.Env = append(cmd.Env, fmt.Sprintf(initPipeEnv+"=%d", 2+len(cmd.ExtraFiles)))

	return gob.NewEncoder(w).Encode(initPipeConfig{
		Hostname:              config.Hostname,
		Workdir:               config.Workdir,
		Interactive:           config.Interactive,
		SharedNamespaceConfig: config.SharedNamespaceConfig,
	})
}

func getEnv() (result initPipeConfig, err error) {
	fdString, ok := os.LookupEnv(initPipeEnv)
	if !ok {
		return result, fmt.Errorf("cannot get init pipe FD from environment")
	}
	fd, err := strconv.Atoi(fdString)
	if err != nil {
		return result, err
	}
	file := os.NewFile(uintptr(fd), "pipe")
	if file == nil {
		return result, fmt.Errorf("cannot use an init pipe fd %d: opened file is nil", fd)
	}
	defer file.Close()
	if err = gob.NewDecoder(file).Decode(&result); err != nil {
		return result, fmt.Errorf("cannot decode from init pipe: %w", err)
	}
	return result, nil
}

func isNSSelected(ns string, flags int) bool {
	switch ns {
	case "cgroup":
		return flags&unix.CLONE_NEWCGROUP != 0
	case "ipc":
		return flags&syscall.CLONE_NEWIPC != 0
	case "mnt":
		return flags&syscall.CLONE_NEWNS != 0
	case "net":
		return flags&syscall.CLONE_NEWNET != 0
	case "pid":
		return flags&syscall.CLONE_NEWPID != 0
	case "pid_for_children":
		log.Println("pid_for_children not implemented")
		return false
	case "user":
		return flags&syscall.CLONE_NEWUSER != 0
	case "uts":
		return flags&syscall.CLONE_NEWUTS != 0
	default:
		panic(errors.New("invalid ns " + ns))
	}
}

func getNses(config SharedNamespaceConfig) ([]*os.File, error) {
	nses := make([]*os.File, 0, 4)

	nsPath := fmt.Sprintf("/proc/%d/ns", config.PID)
	dir, err := ioutil.ReadDir(nsPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read ns-es: %w", err)
	}
	for _, f := range dir {
		if !isNSSelected(f.Name(), config.Flags) {
			continue // NS not selected
		}
		log.Printf("will try to share %s namespace", f.Name())
		ns, err := os.Open(filepath.Join(nsPath, f.Name()))
		if err != nil {
			for _, n := range nses {
				n.Close()
			}
			return nil, fmt.Errorf("cannot open ns: %w", err)
		}
		nses = append(nses, ns)
	}
	for i, ns := range nses {
		if ns.Name() == "user" { // make sure user NS is the first available NS in the list (if user is shared)
			tmp := nses[0]
			nses[0] = nses[i]
			nses[i] = tmp
			break
		}
	}
	return nses, nil
}

func setupSharedNSes(cmd *exec.Cmd, config *Config) error {
	cmd.SysProcAttr.Cloneflags ^= uintptr(config.SharedNamespaceConfig.Flags) // unset cloning NS-es we share
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
