package ui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"aks-cli/internal/commands"

	"github.com/creack/pty"
	"github.com/gdamore/tcell/v2"
	"github.com/hinshun/vt10x"
	"github.com/rivo/tview"
)

type terminalPane struct {
	*tview.Box
	mu      sync.Mutex
	view    vt10x.Terminal
	file    *os.File
	cmd     *exec.Cmd
	done    chan struct{}
	running bool
	result  string
	title   string
	cols    int
	rows    int
}

func newTerminalPane() *terminalPane {
	t := &terminalPane{Box: tview.NewBox(), cols: 80, rows: 10, result: "Ready"}
	t.SetBorder(true)
	t.view = vt10x.New(vt10x.WithSize(t.cols, t.rows))
	return t
}

func (t *terminalPane) state() (bool, string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.running, t.result
}

func (t *terminalPane) start(section commands.Section, notify func()) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.running {
		return errors.New("a command is already running; interrupt it or wait for it to finish")
	}
	t.title = section.Title
	t.view = vt10x.New(vt10x.WithSize(t.cols, t.rows))
	t.writeOutput("$ " + strings.ReplaceAll(section.Content, "\n", "\r\n") + "\r\n")
	cmd := exec.Command(section.Command.Executable, section.Command.Args...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	file, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(t.cols), Rows: uint16(t.rows)})
	if err != nil {
		t.result = "Failed to start: " + err.Error()
		t.writeOutput("\r\n" + t.result + "\r\n")
		return err
	}
	t.file, t.cmd, t.running = file, cmd, true
	t.done = make(chan struct{})
	t.result = "Running"
	// Terminal query responses are written back to the child, not the host terminal.
	t.view = vt10x.New(vt10x.WithSize(t.cols, t.rows), vt10x.WithWriter(file))
	t.writeOutput("$ " + strings.ReplaceAll(section.Content, "\n", "\r\n") + "\r\n")
	go t.run(file, cmd, t.done, notify)
	return nil
}

func (t *terminalPane) writeOutput(text string) {
	if _, err := t.view.Write([]byte(text)); err != nil {
		t.result = "Terminal rendering error: " + err.Error()
	}
}

func (t *terminalPane) run(file *os.File, cmd *exec.Cmd, done chan struct{}, notify func()) {
	readDone := make(chan error, 1)
	go func() {
		buffer := make([]byte, 4096)
		for {
			n, err := file.Read(buffer)
			if n > 0 {
				t.mu.Lock()
				t.writeOutput(string(buffer[:n]))
				t.mu.Unlock()
				notify()
			}
			if err != nil {
				readDone <- err
				return
			}
		}
	}()
	waitErr := cmd.Wait()
	var readErr, closeErr error
	cleanedDescendants := false
	select {
	case readErr = <-readDone:
	case <-time.After(250 * time.Millisecond):
		// A descendant may retain the PTY after its parent exits.
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			waitErr = errors.Join(waitErr, err)
		}
		cleanedDescendants = true
		closeErr = file.Close()
		readErr = <-readDone
	}
	if closeErr == nil {
		closeErr = file.Close()
	}
	t.mu.Lock()
	t.file = nil
	t.running = false
	t.result = "Exit 0"
	if waitErr != nil {
		t.result = "Failed: " + waitErr.Error()
	}
	if cleanedDescendants {
		t.result += "; closed lingering child processes"
	}
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, syscall.EIO) && !errors.Is(readErr, os.ErrClosed) {
		t.result += "; read error: " + readErr.Error()
	}
	if closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
		t.result += "; close error: " + closeErr.Error()
	}
	t.writeOutput("\r\n[" + t.result + "]\r\n")
	close(done)
	t.mu.Unlock()
	notify()
}

func (t *terminalPane) close() error {
	t.mu.Lock()
	if !t.running {
		t.mu.Unlock()
		return nil
	}
	pid, done := t.cmd.Process.Pid, t.done
	t.mu.Unlock()
	err := syscall.Kill(-pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		err = nil
	}
	<-done
	return err
}

func (t *terminalPane) send(data string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.running || t.file == nil {
		return
	}
	if _, err := io.WriteString(t.file, data); err != nil {
		t.result = "Terminal input failed: " + err.Error()
	}
}

func (t *terminalPane) Draw(screen tcell.Screen) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.SetTitle(" Terminal: " + tview.Escape(t.title+" | "+t.result) + " | Esc: panes | Ctrl-C: interrupt | F2: hide ")
	t.Box.DrawForSubclass(screen, t)
	x, y, width, height := t.GetInnerRect()
	if width <= 0 || height <= 0 {
		return
	}
	if t.cols != width || t.rows != height {
		t.cols, t.rows = width, height
		t.view.Resize(width, height)
		if t.file != nil {
			if err := pty.Setsize(t.file, &pty.Winsize{Cols: uint16(width), Rows: uint16(height)}); err != nil {
				t.result = "Terminal resize failed: " + err.Error()
			}
		}
	}
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			cell := t.view.Cell(col, row)
			style := tcell.StyleDefault.Foreground(terminalColor(cell.FG)).Background(terminalColor(cell.BG))
			screen.SetContent(x+col, y+row, cell.Char, nil, style)
		}
	}
	if t.HasFocus() && t.view.CursorVisible() {
		cursor := t.view.Cursor()
		screen.ShowCursor(x+cursor.X, y+cursor.Y)
	}
}

func terminalColor(color vt10x.Color) tcell.Color {
	if color == vt10x.DefaultFG || color == vt10x.DefaultBG || color == vt10x.DefaultCursor {
		return tcell.ColorDefault
	}
	if color < 256 {
		return tcell.PaletteColor(int(color))
	}
	return tcell.NewHexColor(int32(color))
}

func (t *terminalPane) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return t.WrapInputHandler(func(event *tcell.EventKey, _ func(tview.Primitive)) {
		data := terminalKey(event)
		if data != "" {
			t.send(data)
		}
	})
}

func (t *terminalPane) PasteHandler() func(string, func(tview.Primitive)) {
	return func(text string, _ func(tview.Primitive)) {
		t.send(text)
	}
}

func terminalKey(event *tcell.EventKey) string {
	var data string
	switch event.Key() {
	case tcell.KeyRune:
		data = string(event.Rune())
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		data = "\x7f"
	case tcell.KeyUp:
		data = "\x1b[A"
	case tcell.KeyDown:
		data = "\x1b[B"
	case tcell.KeyRight:
		data = "\x1b[C"
	case tcell.KeyLeft:
		data = "\x1b[D"
	case tcell.KeyHome:
		data = "\x1b[H"
	case tcell.KeyEnd:
		data = "\x1b[F"
	case tcell.KeyDelete:
		data = "\x1b[3~"
	case tcell.KeyPgUp:
		data = "\x1b[5~"
	case tcell.KeyPgDn:
		data = "\x1b[6~"
	default:
		if event.Key() >= 0 && event.Key() < 32 {
			data = string(rune(event.Key()))
		}
	}
	if event.Modifiers()&tcell.ModAlt != 0 && data != "" {
		data = "\x1b" + data
	}
	return data
}

func (t *terminalPane) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return t.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, focus func(tview.Primitive)) (bool, tview.Primitive) {
		if t.InRect(event.Position()) && action == tview.MouseLeftClick {
			focus(t)
			return true, nil
		}
		return false, nil
	})
}

func (t *terminalPane) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return fmt.Sprint(t.view)
}
