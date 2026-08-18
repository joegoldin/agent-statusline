// Package layout composes widgets into rows, applies the reactive drop
// priority on overflow, and renders flex spacers / right-aligned content.
package layout

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const defaultWidth = 80

// widthSafetyReserve trims a few columns off the auto-detected terminal width
// so content never butts against the right edge or gets clipped by things the
// detected width can't see (Claude Code's own padding, IDE integration text,
// the auto-compact banner). Not applied to an explicit CLAUDE_STATUSLINE_WIDTH.
const widthSafetyReserve = 10

// DetectWidth returns the effective terminal width. Precedence:
//  1. CLAUDE_STATUSLINE_WIDTH env var (must be positive int).
//  2. TIOCGWINSZ ioctl on our own stdio (only if attached to a tty).
//  3. The controlling terminal of an ancestor process (Claude Code pipes our
//     stdio, so this is the case that normally wins).
//  4. defaultWidth (80).
func DetectWidth() int {
	if s := os.Getenv("CLAUDE_STATUSLINE_WIDTH"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	if w := ttyWidth(); w > 0 {
		if w -= widthSafetyReserve; w < 1 {
			w = 1
		}
		return w
	}
	return defaultWidth
}

type winsize struct {
	Row, Col       uint16
	XPixel, YPixel uint16
}

func ttyWidth() int {
	for _, fd := range []uintptr{os.Stdout.Fd(), os.Stderr.Fd(), os.Stdin.Fd()} {
		if w := ioctlCols(fd); w > 0 {
			return w
		}
	}
	// Claude Code invokes us with piped stdio, so our own fds aren't a tty and
	// /dev/tty often isn't usable. Fall back to the controlling terminal of an
	// ancestor process (ccstatusline's trick): find an ancestor's tty name and
	// read its column count.
	return ancestorTTYWidth()
}

func ioctlCols(fd uintptr) int {
	ws := &winsize{}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		fd, uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(ws)))
	if errno == 0 && ws.Col > 0 {
		return int(ws.Col)
	}
	return 0
}

// ancestorTTYWidth walks up the process tree from our parent and, for the first
// ancestor attached to a real terminal, reads that terminal's column count.
func ancestorTTYWidth() int {
	pid := os.Getppid()
	for i := 0; i < 8 && pid > 1; i++ {
		tty, ppid := procTTYAndParent(pid)
		if tty != "" && tty != "??" && tty != "?" {
			if f, err := os.Open("/dev/" + tty); err == nil {
				w := ioctlCols(f.Fd())
				_ = f.Close()
				if w > 0 {
					return w
				}
			}
		}
		if ppid <= 1 {
			break
		}
		pid = ppid
	}
	return 0
}

// procTTYAndParent returns pid's tty name (without the /dev/ prefix) and its
// parent pid via `ps`. The `-o name=` form suppresses the header on both
// macOS/BSD and Linux; a process with no controlling terminal reports "??".
func procTTYAndParent(pid int) (tty string, ppid int) {
	out, err := exec.Command("ps", "-o", "tty=,ppid=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return "", 0
	}
	fields := strings.Fields(string(out))
	if len(fields) < 2 {
		return "", 0
	}
	ppid, _ = strconv.Atoi(fields[len(fields)-1])
	return fields[0], ppid
}
