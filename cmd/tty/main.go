package main

import (
	"cont/tty"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func main() {
	pty, err := tty.OpenPTY()
	if err != nil {
		panic(err)
	}

	backupTerm, _ := tty.Attr(os.Stdin)
	// Copy attributes
	myTerm := backupTerm
	// Change the Stdin term to RAW so we get everything
	//myTerm.Raw()

	myTerm.Set(os.Stdin)
	// Set the backup attributes on our PTY slave
	backupTerm.Set(pty.Slave)
	// Make sure we'll get the attributes back when exiting
	defer backupTerm.Set(os.Stdin)
	// Get the snooping going
	go Snoop(pty)

	sig := make(chan os.Signal, 2)
	signal.Notify(sig, syscall.SIGWINCH, syscall.SIGCLD)

	command := exec.Command("bash")
	command.Stdin = pty.Slave
	command.Stdout = pty.Slave
	command.Stderr = pty.Slave
	command.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
	}
	if err = command.Start(); err != nil {
		panic(err)
	}
	myTerm.Winsz(os.Stdin)
	myTerm.Winsz(pty.Slave)
	for {
		switch <-sig {
		case syscall.SIGWINCH:
			myTerm.Winsz(os.Stdin)
			myTerm.Setwinsz(pty.Slave)
		default:
			return
		}
	}
}

func Snoop(pty *tty.PTY) {
	// Just something that might be a bit uniqe
	//pid := os.Getpid()
	//pidcol, _ := tty.NewColor256(strconv.Itoa(pid), strconv.Itoa(pid%256), "")
	//greet := fmt.Sprintln("\n", term.Green(Welcome), " pid:", pidcol,
	//	" file:", term.Yellow(file+strconv.Itoa(pid)+"\n"))
	// Our logfile
	//file, _ := os.Create(file + strconv.Itoa(pid))
	//os.Stdout.Write([]byte(greet))
	go reader(pty.Master)
	go writer(pty.Master)
}

// reader reads from master and writes to file and stdout
func reader(master *os.File) {
	var buf = make([]byte, 2048)
	for {
		nr, _ := master.Read(buf)
		os.Stdout.Write(buf[:nr])
	}
}

// writer reads from stdin and writes to master
func writer(master *os.File) {
	var buf = make([]byte, 2048)
	for {
		nr, _ := os.Stdin.Read(buf)
		master.Write(buf[:nr])
	}
}
