package tty

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func AttachPTSToTerminal(stdin io.Reader, stdout io.Writer) (*PTY, error) {
	pty, err := OpenPTY()
	if err != nil {
		return nil, err
	}

	backupTerm, err := Attr(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("cannot get term attr: %w", err)
	}
	// Copy attributes
	myTerm := backupTerm
	// Change the Stdin term to RAW so we get everything
	myTerm.Raw()

	pty.runningTermios = &myTerm
	pty.backupTermios = &backupTerm

	if err = myTerm.Set(os.Stdin); err != nil {
		return nil, fmt.Errorf("cannot set stdin termios: %w", err)
	}
	// Set the backup attributes on our PTY slave
	//if err = backupTerm.Set(pty.Slave); err != nil {
	//	return nil, fmt.Errorf("cannot set slave termios: %w", err)
	//}

	sig := make(chan os.Signal, 2)
	signal.Notify(sig, syscall.SIGWINCH, syscall.SIGCLD)

	go Snoop(pty, stdin, stdout)

	if err := myTerm.Winsz(os.Stdin); err != nil {
		return nil, err
	}
	if err := myTerm.Winsz(pty.Slave); err != nil {
		return nil, err
	}

	go func() {
		// Make sure we'll get the attributes back when exiting
		for {
			switch <-sig {
			case syscall.SIGWINCH:
				myTerm.Winsz(os.Stdin)
				myTerm.Setwinsz(pty.Slave)
			default:
				return
			}
		}
	}()
	return pty, nil
}

func Start(cmd *exec.Cmd, stdin io.Reader, stdout io.Writer) (*PTY, error) {
	if stdin == nil {
		return nil, errors.New("stdin is nil")
	}
	if stdout == nil {
		return nil, errors.New("stdout is nil")
	}

	pty, err := AttachPTSToTerminal(stdin, stdout)
	if err != nil {
		return nil, err
	}

	cmd.Stdin = pty.Slave
	cmd.Stdout = pty.Slave
	cmd.Stderr = pty.Slave
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
	cmd.SysProcAttr.Setctty = true

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return pty, nil
}

func Snoop(pty *PTY, stdin io.Reader, stdout io.Writer) {
	if stdin != nil {
		go writer(pty.Master, stdin)
	}
	if stdout != nil {
		go reader(pty.Master, stdout)
	}
}

// reader reads from master and writes to file and stdout
func reader(master *os.File, stdout io.Writer) {
	var buf = make([]byte, 2048)
	for {
		nr, _ := master.Read(buf)
		read := buf[:nr]
		if _, err := stdout.Write(read); err != nil {
			os.Exit(0)
		}
		//log.Printf("written %s", string(read))
	}
}

// writer reads from stdin and writes to master
func writer(master *os.File, stdin io.Reader) {
	var buf = make([]byte, 2048)
	for {
		nr, _ := stdin.Read(buf)
		read := buf[:nr]
		//log.Printf("read %s", string(read))
		if _, err := master.Write(read); err != nil {
			os.Exit(0)
		}
	}
}
