package tty

import (
	"os"
	"syscall"
	"unsafe"
)

type Termios struct {
	syscall.Termios
	Wz Winsize
}

type Winsize struct {
	WsRow    uint16 // WsRow 		Terminal number of rows
	WsCol    uint16 // WsCol 		Terminal number of columns
	WsXpixel uint16 // WsXpixel Terminal width in pixels
	WsYpixel uint16 // WsYpixel Terminal height in pixels
}

func (t *Termios) Winsz(file *os.File) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), uintptr(TIOCGWINSZ), uintptr(unsafe.Pointer(&t.Wz)))
	if errno != 0 {
		return errno
	}
	return nil
}

func (t *Termios) Setwinsz(file *os.File) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), uintptr(TIOCSWINSZ), uintptr(unsafe.Pointer(&t.Wz)))
	if errno != 0 {
		return errno
	}
	return nil
}

func (t *Termios) Set(file *os.File) error {
	fd := file.Fd()
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(TCSETS), uintptr(unsafe.Pointer(t)))
	if errno != 0 {
		return errno
	}
	return nil
}

func (t *Termios) Raw() {
	t.Iflag &^= IGNBRK | BRKINT | PARMRK | ISTRIP | INLCR | IGNCR | ICRNL | IXON
	// t.Iflag &^= BRKINT | ISTRIP | ICRNL | IXON // Stevens RAW
	t.Oflag &^= OPOST
	t.Lflag &^= ECHO | ECHONL | ICANON | ISIG | IEXTEN
	t.Cflag &^= CSIZE | PARENB
	t.Cflag |= CS8
	t.Cc[VMIN] = 1
	t.Cc[VTIME] = 0
}

func Attr(file *os.File) (Termios, error) {
	var t Termios
	fd := file.Fd()
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(TCGETS), uintptr(unsafe.Pointer(&t)))
	if errno != 0 {
		return t, errno
	}
	t.Ispeed &= CBAUD | CBAUDEX
	t.Ospeed &= CBAUD | CBAUDEX
	return t, nil
}

func Isatty(file *os.File) bool {
	_, err := Attr(file)
	return err == nil
}
