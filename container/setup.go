package container

import (
	"encoding/gob"
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

func getNses(pid int) ([]*os.File, error) {
	nses := make([]*os.File, 0, 4)

	nsPath := fmt.Sprintf("/proc/%d/ns", pid)
	dir, err := ioutil.ReadDir(nsPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read ns-es: %w", err)
	}
	for _, f := range dir {
		if f.Name() != "net" { // don't try to share mounts
			continue
		}
		ns, err := os.Open(filepath.Join(nsPath, f.Name()))
		if err != nil {
			for _, n := range nses {
				n.Close()
			}
			return nil, fmt.Errorf("cannot open ns: %w", err)
		}
		nses = append(nses, ns)
	}
	return nses, nil
}

// call *only* at process startup - extremely bad design
func attachToNSes() {
	for fd := 3; ; fd++ {
		_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFD, 0)
		if errno != 0 {
			return
		}
		err := unix.Setns(fd, 0)
		if err != nil {
			log.Printf("cannot set ns %d: %v", fd, err)
			return
		}
		log.Printf("set ns %d", fd)
	}
}
