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
	TCGETS     = syscall.TCGETS     // TCGETS get terminal attributes
	TCSETS     = syscall.TCSETS     // TCSETS set terminal attributes
	TIOCGWINSZ = syscall.TIOCGWINSZ // TIOCGWINSZ used to get the terminal window size
	TIOCSWINSZ = syscall.TIOCSWINSZ // TIOCSWINSZ used to set the terminal window size
	TIOCGPTN   = syscall.TIOCGPTN   // TIOCGPTN IOCTL used to get the PTY number
	TIOCSPTLCK = syscall.TIOCSPTLCK // TIOCSPTLCK IOCT used to lock/unlock PTY
	CBAUD      = 0010017            // CBAUD Serial speed settings
	CBAUDEX    = 0010000            // CBAUDX Serial speed settings
)

// INPUT handling terminal flags
// see 'man stty' for further info about most of the constants
const (
	IGNBRK  = syscall.IGNBRK  // 0000001 // IGNBRK ignore break characters
	BRKINT  = syscall.BRKINT  // 0000002 // BRKINT Break genereates an interrupt signal
	IGNPAR  = syscall.IGNPAR  // 0000004 // IGNPAR Ignore characters with parity errors
	PARMRK  = syscall.PARMRK  // 0000010 // PARMRK Mark parity errors byte{ff,0}
	INPCK   = syscall.INPCK   // 0000020 // INPCK enable parity checking
	ISTRIP  = syscall.ISTRIP  // 0000040 // ISTRIP Clear 8th bit of input characters
	INLCR   = syscall.INLCR   // 0000100 // INLCR Translate LF => CR
	IGNCR   = syscall.IGNCR   // 0000200 // IGNCR Ignore Carriage Return
	ICRNL   = syscall.ICRNL   // 0000400 // ICRNL Translate CR => NL
	IUCLC   = syscall.IUCLC   // 0001000 // IUCLC Translate uppercase to lowercase
	IXON    = syscall.IXON    // 0002000 // IXON Enable flow control
	IXANY   = syscall.IXANY   // 0004000 // IXANY let any char restart input
	IXOFF   = syscall.IXOFF   // 0010000 // IXOFF start sending start/stop chars
	IMAXBEL = syscall.IMAXBEL // 0020000 // IMAXBEL Sound the bell and skip flushing input buffer
	IUTF8   = syscall.IUTF8   // 0040000 // IUTF8 assume input being utf-8
)

// OUTPUT treatment terminal flags
const (
	OPOST  = syscall.OPOST  // 0000001 // OPOST post process output
	OLCUC  = syscall.OLCUC  // 0000002 // OLCUC translate lower case to upper case
	ONLCR  = syscall.ONLCR  // 0000004 // ONLCR Map NL -> CR-NL
	OCRNL  = syscall.OCRNL  // 0000010 // OCRNL Map CR -> NL
	ONOCR  = syscall.ONOCR  // 0000020 // ONOCR No CR at col 0
	ONLRET = syscall.ONLRET // 0000040 // ONLRET NL also do CR
	OFILL  = syscall.OFILL  // 0000100 // OFILL Fillchar for delay
	OFDEL  = syscall.OFDEL  // 0000200 // OFDEL use delete instead of null
)

// TERM control modes.
const (
	CSIZE  = syscall.CSIZE  // 0000060 // CSIZE used as mask when setting character size
	CS5    = syscall.CS5    // 0000000 // CS5 char size 5bits
	CS6    = syscall.CS6    // 0000020 // CS6 char size 6bits
	CS7    = syscall.CS7    // 0000040 // CS7 char size 7bits
	CS8    = syscall.CS8    // 0000060 // CS8 char size 8bits
	CSTOPB = syscall.CSTOPB // 0000100 // CSTOPB two stop bits
	CREAD  = syscall.CREAD  // 0000200 // CREAD enable input
	PARENB = syscall.PARENB // 0000400 // PARENB generate and expect parity bit
	PARODD = syscall.PARODD // 0001000 // PARODD set odd parity
	HUPCL  = syscall.HUPCL  // 0002000 // HUPCL send HUP when last process closes term
	CLOCAL = syscall.CLOCAL // 0004000 // CLOCAL no modem control signals
)

// TERM modes
const (
	ISIG    = syscall.ISIG    // 0000001        // ISIG enable Interrupt,quit and suspend chars
	ICANON  = syscall.ICANON  // 0000002        // ICANON enable erase,kill ,werase and rprnt chars
	XCASE   = syscall.XCASE   // 0000004        // XCASE preceedes all uppercase chars with '\'
	ECHO    = syscall.ECHO    // 0000010        // ECHO echo input characters
	ECHOE   = syscall.ECHOE   // 0000020        // ECHOE erase => BS - SPACE - BS
	ECHOK   = syscall.ECHOK   // 0000040        // ECHOK add newline after kill char
	ECHONL  = syscall.ECHONL  // 0000100        // ECHONL echo NL even without other characters
	NOFLSH  = syscall.NOFLSH  // 0000200        // NOFLSH no flush after interrupt and kill characters
	TOSTOP  = syscall.TOSTOP  // 0000400        // TOSTOP stop BG jobs trying to write to term
	ECHOCTL = syscall.ECHOCTL // 0001000        // ECHOCTL will echo control characters as ^c
	ECHOPRT = syscall.ECHOPRT // 0002000        // ECHOPRT will print erased characters between \ /
	ECHOKE  = syscall.ECHOKE  // 0004000        // ECHOKE kill all line considering ECHOPRT and ECHOE flags
	IEXTEN  = syscall.IEXTEN  // IEXTEN enable non POSIX special characters
)

// Control characters
const (
	VINTR    = syscall.VINTR    // 0  // VINTR 		char will send an interrupt signal
	VQUIT    = syscall.VQUIT    // 1  // VQUIT 		char will send a quit signal
	VERASE   = syscall.VERASE   // 2  // VEREASE 	char will erase last typed char
	VKILL    = syscall.VKILL    // 3  // VKILL 		char will erase current line
	VEOF     = syscall.VEOF     // 4  // VEOF 		char will send EOF
	VTIME    = syscall.VTIME    // 5  // VTIME 		set read timeout in tenths of seconds
	VMIN     = syscall.VMIN     // 6  // VMIN 		set min characters for a complete read
	VSWTC    = syscall.VSWTC    // 7  // VSWTC 		char will switch to a different shell layer
	VSTART   = syscall.VSTART   // 8  // VSTART 	char will restart output after stopping it
	VSTOP    = syscall.VSTOP    // 9  // VSTOP 		char will stop output
	VSUSP    = syscall.VSUSP    // 10 // VSUSP 		char will send a stop signal
	VEOL     = syscall.VEOL     // 11 // VEOL 		char will end the line
	VREPRINT = syscall.VREPRINT // 12 // VREPRINT will redraw the current line
	VDISCARD = syscall.VDISCARD // 13 // VDISCARD
	VWERASE  = syscall.VWERASE  // 14 // VWERASE 	char will erase last word typed
	VLNEXT   = syscall.VLNEXT   // 15 // VLNEXT 	char will enter the next char quoted
	VEOL2    = syscall.VEOL2    // 16 // VEOL2 		char alternate to end line
	tNCCS    = 32               // tNCCS    Termios CC size
)

type PTY struct {
	Master         *os.File
	Slave          *os.File
	runningTermios *Termios
	backupTermios  *Termios
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
	if p.backupTermios != nil {
		p.backupTermios.Set(os.Stdin)
	}
	slaveErr := errors.New("slave FD nil")
	if p.Slave != nil {
		slaveErr = p.Slave.Close()
	}
	masterErr := errors.New("master FD nil")
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
