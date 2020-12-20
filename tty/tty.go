package tty

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// IOCTL terminal stuff.
const (
	TCGETS     = 0x5401     // TCGETS get terminal attributes
	TCSETS     = 0x5402     // TCSETS set terminal attributes
	TIOCGWINSZ = 0x5413     // TIOCGWINSZ used to get the terminal window size
	TIOCSWINSZ = 0x5414     // TIOCSWINSZ used to set the terminal window size
	TIOCGPTN   = 0x80045430 // TIOCGPTN IOCTL used to get the PTY number
	TIOCSPTLCK = 0x40045431 // TIOCSPTLCK IOCT used to lock/unlock PTY
	CBAUD      = 0010017    // CBAUD Serial speed settings
	CBAUDEX    = 0010000    // CBAUDX Serial speed settings
)

// INPUT handling terminal flags
// see 'man stty' for further info about most of the constants
const (
	IGNBRK  = 0000001 // IGNBRK ignore break characters
	BRKINT  = 0000002 // BRKINT Break genereates an interrupt signal
	IGNPAR  = 0000004 // IGNPAR Ignore characters with parity errors
	PARMRK  = 0000010 // PARMRK Mark parity errors byte{ff,0}
	INPCK   = 0000020 // INPCK enable parity checking
	ISTRIP  = 0000040 // ISTRIP Clear 8th bit of input characters
	INLCR   = 0000100 // INLCR Translate LF => CR
	IGNCR   = 0000200 // IGNCR Ignore Carriage Return
	ICRNL   = 0000400 // ICRNL Translate CR => NL
	IUCLC   = 0001000 // IUCLC Translate uppercase to lowercase
	IXON    = 0002000 // IXON Enable flow control
	IXANY   = 0004000 // IXANY let any char restart input
	IXOFF   = 0010000 // IXOFF start sending start/stop chars
	IMAXBEL = 0020000 // IMAXBEL Sound the bell and skip flushing input buffer
	IUTF8   = 0040000 // IUTF8 assume input being utf-8
)

// OUTPUT treatment terminal flags
const (
	OPOST  = 0000001 // OPOST post process output
	OLCUC  = 0000002 // OLCUC translate lower case to upper case
	ONLCR  = 0000004 // ONLCR Map NL -> CR-NL
	OCRNL  = 0000010 // OCRNL Map CR -> NL
	ONOCR  = 0000020 // ONOCR No CR at col 0
	ONLRET = 0000040 // ONLRET NL also do CR
	OFILL  = 0000100 // OFILL Fillchar for delay
	OFDEL  = 0000200 // OFDEL use delete instead of null
)

// TERM control modes.
const (
	CSIZE  = 0000060 // CSIZE used as mask when setting character size
	CS5    = 0000000 // CS5 char size 5bits
	CS6    = 0000020 // CS6 char size 6bits
	CS7    = 0000040 // CS7 char size 7bits
	CS8    = 0000060 // CS8 char size 8bits
	CSTOPB = 0000100 // CSTOPB two stop bits
	CREAD  = 0000200 // CREAD enable input
	PARENB = 0000400 // PARENB generate and expect parity bit
	PARODD = 0001000 // PARODD set odd parity
	HUPCL  = 0002000 // HUPCL send HUP when last process closes term
	CLOCAL = 0004000 // CLOCAL no modem control signals
)

// TERM modes
const (
	ISIG    = 0000001 // ISIG enable Interrupt,quit and suspend chars
	ICANON  = 0000002 // ICANON enable erase,kill ,werase and rprnt chars
	XCASE   = 0000004 // XCASE preceedes all uppercase chars with '\'
	ECHO    = 0000010 // ECHO echo input characters
	ECHOE   = 0000020 // ECHOE erase => BS - SPACE - BS
	ECHOK   = 0000040 // ECHOK add newline after kill char
	ECHONL  = 0000100 // ECHONL echo NL even without other characters
	NOFLSH  = 0000200 // NOFLSH no flush after interrupt and kill characters
	TOSTOP  = 0000400 // TOSTOP stop BG jobs trying to write to term
	ECHOCTL = 0001000 // ECHOCTL will echo control characters as ^c
	ECHOPRT = 0002000 // ECHOPRT will print erased characters between \ /
	ECHOKE  = 0004000 // ECHOKE kill all line considering ECHOPRT and ECHOE flags
	IEXTEN  = 0100000 // IEXTEN enable non POSIX special characters
)

// Control characters
const (
	VINTR    = 0  // VINTR 		char will send an interrupt signal
	VQUIT    = 1  // VQUIT 		char will send a quit signal
	VERASE   = 2  // VEREASE 	char will erase last typed char
	VKILL    = 3  // VKILL 		char will erase current line
	VEOF     = 4  // VEOF 		char will send EOF
	VTIME    = 5  // VTIME 		set read timeout in tenths of seconds
	VMIN     = 6  // VMIN 		set min characters for a complete read
	VSWTC    = 7  // VSWTC 		char will switch to a different shell layer
	VSTART   = 8  // VSTART 	char will restart output after stopping it
	VSTOP    = 9  // VSTOP 		char will stop output
	VSUSP    = 10 // VSUSP 		char will send a stop signal
	VEOL     = 11 // VEOL 		char will end the line
	VREPRINT = 12 // VREPRINT will redraw the current line
	VDISCARD = 13 // VDISCARD
	VWERASE  = 14 // VWERASE 	char will erase last word typed
	VLNEXT   = 15 // VLNEXT 	char will enter the next char quoted
	VEOL2    = 16 // VEOL2 		char alternate to end line
	tNCCS    = 32 // tNCCS    Termios CC size
)

type PTY struct {
	Master *os.File
	Slave  *os.File
}

func OpenPTY() (*PTY, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("cannot open ptmx: %w", err)
	}
	var unlock int
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), uintptr(TIOCSPTLCK), uintptr(unsafe.Pointer(&unlock))); errno != 0 {
		_ = master.Close()
		return nil, fmt.Errorf("cannot unlock PTY: %w", err)
	}
	pty := &PTY{Master: master}
	slaveStr, err := pty.PTSName()
	if err != nil {
		master.Close()
		return nil, fmt.Errorf("cannot get PTS name: %w", err)
	}
	pty.Slave, err = os.OpenFile(slaveStr, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		master.Close()
		return nil, fmt.Errorf("cannot open PTS: %w", err)
	}
	return pty, nil
}

// return the PTY number
func (p *PTY) PTSNumber() (uint, error) {
	var ptyno uint
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, p.Master.Fd(), uintptr(TIOCGPTN), uintptr(unsafe.Pointer(&ptyno)))
	if errno != 0 {
		return 0, errno
	}
	return ptyno, nil
}

func (p *PTY) PTSName() (string, error) {
	n, err := p.PTSNumber()
	if err != nil {
		return "", err
	}
	return filepath.Join("/dev/pts/", strconv.Itoa(int(n))), nil
}

func (p *PTY) Close() error {
	slaveErr := errors.New("Slave FD nil")
	if p.Slave != nil {
		slaveErr = p.Slave.Close()
	}
	masterErr := errors.New("Master FD nil")
	if p.Master != nil {
		masterErr = p.Master.Close()
	}
	if slaveErr != nil || masterErr != nil {
		var errs []string
		if slaveErr != nil {
			errs = append(errs, "Slave: "+slaveErr.Error())
		}
		if masterErr != nil {
			errs = append(errs, "Master: "+masterErr.Error())
		}
		return errors.New(strings.Join(errs, " "))
	}
	return nil
}

func (p *PTY) ReadByte() (byte, error) {
	bs := make([]byte, 1, 1)
	_, err := p.Master.Read(bs)
	return bs[0], err
}
