package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"aks-cli/internal/commands"
	"aks-cli/internal/config"

	"github.com/creack/pty"
	"github.com/gdamore/tcell/v2"
)

func localCommand(script string, args ...string) commands.Section {
	return commands.Section{Title: "Local test", Content: "local fixture",
		Command: commands.Command{Executable: "/bin/sh", Args: append([]string{"-c", script, "fixture"}, args...)}}
}

func testTerminal(t *testing.T) *terminalPane {
	t.Helper()
	pane := newTerminalPane()
	t.Cleanup(func() {
		if err := pane.close(); err != nil {
			t.Error(err)
		}
	})
	return pane
}

func waitTerminal(t *testing.T, pane *terminalPane) string {
	t.Helper()
	pane.mu.Lock()
	done := pane.done
	pane.mu.Unlock()
	select {
	case <-done:
		running, result := pane.state()
		if running {
			t.Fatal("process finished but terminal still running")
		}
		return result
	case <-time.After(5 * time.Second):
		t.Fatal("terminal process did not finish")
		return ""
	}
}

func waitText(t *testing.T, pane *terminalPane, text string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(pane.String(), text) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("terminal never displayed %q:\n%s", text, pane.String())
}

func TestTerminalPTYOutputExitAndInput(t *testing.T) {
	pane := testTerminal(t)
	section := localCommand(`test -t 0 && test -t 1 && test -t 2 || exit 99
printf '\033[31mREADY\033[0m\n'
read answer
printf 'INPUT:%s\n' "$answer"
printf 'STDERR\n' >&2
exit 7`)
	if err := pane.start(section, func() {}); err != nil {
		t.Fatal(err)
	}
	waitText(t, pane, "READY")
	if err := pane.start(localCommand("exit 0"), func() {}); err == nil {
		t.Fatal("started concurrent process")
	}
	pane.send("hello terminal\r")
	if result := waitTerminal(t, pane); !strings.Contains(result, "exit status 7") {
		t.Fatalf("exit status not shown: %s", result)
	}
	for _, text := range []string{"READY", "INPUT:hello terminal", "STDERR", "[Failed:"} {
		if !strings.Contains(pane.String(), text) {
			t.Fatalf("missing %q:\n%s", text, pane.String())
		}
	}
	if strings.Contains(pane.String(), "\x1b[31m") {
		t.Fatal("ANSI sequence rendered as literal text")
	}
}

func TestTerminalArgumentsAreNotShellCommands(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "az")
	output := filepath.Join(dir, "args")
	sentinel := filepath.Join(dir, "must-not-exist")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$ARGS_FILE\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("ARGS_FILE", output)
	cluster := fixture("alice", "dev; touch "+sentinel+"; $(false) ' \"")
	section := commands.Sections(cluster)[1]
	pane := testTerminal(t)
	if err := pane.start(section, func() {}); err != nil {
		t.Fatal(err)
	}
	if result := waitTerminal(t, pane); result != "Exit 0" {
		t.Fatal(result)
	}
	data, err := os.ReadFile(output)
	if err != nil || string(data) != strings.Join(section.Command.Args, "\n")+"\n" {
		t.Fatalf("arguments changed: %q, %v", data, err)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("argument content was executed: %v", err)
	}
}

func TestVariablesExecuteOnlyInChildShell(t *testing.T) {
	cluster := fixture("alice", "Dev's team")
	section := commands.Sections(cluster)[3]
	t.Setenv("SUBSCRIPTION", "parent")
	pane := testTerminal(t)
	if err := pane.start(section, func() {}); err != nil {
		t.Fatal(err)
	}
	if result := waitTerminal(t, pane); result != "Exit 0" {
		t.Fatal(result)
	}
	if os.Getenv("SUBSCRIPTION") != "parent" || !strings.Contains(pane.String(), "SUBSCRIPTION=Dev's team") {
		t.Fatalf("variables were not applied only to child:\n%s", pane.String())
	}
}

func TestTerminalResizeInterruptAndRestart(t *testing.T) {
	pane := testTerminal(t)
	if err := pane.start(localCommand("printf 'READY\\n'; exec /bin/sleep 30"), func() {}); err != nil {
		t.Fatal(err)
	}
	waitText(t, pane, "READY")
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 20)
	pane.SetRect(0, 0, 100, 20)
	pane.Draw(screen)
	pane.mu.Lock()
	size, err := pty.GetsizeFull(pane.file)
	pane.mu.Unlock()
	if err != nil || size.Cols != 98 || size.Rows != 18 {
		t.Fatalf("PTY did not resize: %+v, %v", size, err)
	}
	pane.send("\x03")
	if result := waitTerminal(t, pane); result == "Exit 0" {
		t.Fatal("interrupted process reported success")
	}
	if err := pane.start(localCommand("printf 'RESTARTED\\n'"), func() {}); err != nil {
		t.Fatal(err)
	}
	if result := waitTerminal(t, pane); result != "Exit 0" || !strings.Contains(pane.String(), "RESTARTED") {
		t.Fatalf("restart failed: %s\n%s", result, pane.String())
	}
}

func TestTerminalMissingCommandAndCleanup(t *testing.T) {
	pane := testTerminal(t)
	err := pane.start(commands.Section{Title: "Missing", Command: commands.Command{
		Executable: filepath.Join(t.TempDir(), "does-not-exist")}}, func() {})
	if err == nil {
		t.Fatal("missing command reported success")
	}
	running, result := pane.state()
	if running || !strings.Contains(result, "Failed to start") {
		t.Fatal("startup error not visible")
	}
	if err := pane.start(localCommand("printf 'READY\\n'; exec /bin/sleep 30"), func() {}); err != nil {
		t.Fatal(err)
	}
	waitText(t, pane, "READY")
	if err := pane.close(); err != nil {
		t.Fatal(err)
	}
	if running, _ := pane.state(); running {
		t.Fatal("cleanup left process running")
	}
}

func TestTerminalKeyboardMappings(t *testing.T) {
	for key, want := range map[tcell.Key]string{
		tcell.KeyEnter: "\r", tcell.KeyTab: "\t", tcell.KeyCtrlC: "\x03",
		tcell.KeyUp: "\x1b[A", tcell.KeyDown: "\x1b[B",
		tcell.KeyBackspace2: "\x7f", tcell.KeyDelete: "\x1b[3~",
	} {
		if got := terminalKey(tcell.NewEventKey(key, 0, tcell.ModNone)); got != want {
			t.Errorf("key %v = %q, want %q", key, got, want)
		}
	}
	if got := terminalKey(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModAlt)); got != "\x1ba" {
		t.Fatalf("Alt mapping: %q", got)
	}
}

func TestExecutionConfirmationAndSnapshot(t *testing.T) {
	a, _ := simulate(t, config.Document{Version: config.Version, Clusters: []config.Cluster{fixture("alice", "dev")}},
		func(config.Document) error { return nil })
	var executed []commands.Section
	a.runSection = func(section commands.Section) { executed = append(executed, section) }
	for _, index := range []int{5, 6, 7} {
		section := a.preview.sections[index].data
		a.requestExecute(section)
		if !a.confirming {
			t.Fatal("delete did not open confirmation")
		}
		press(a, tcell.KeyEnter, 0)
		if a.confirming || len(executed) != 0 {
			t.Fatal("default confirmation action was not Cancel")
		}
		a.requestExecute(section)
		press(a, tcell.KeyTab, 0)
		press(a, tcell.KeyEnter, 0)
		if a.confirming || len(executed) != 1 || !reflect.DeepEqual(executed[0], section) {
			t.Fatal("confirmed command changed or failed to dispatch")
		}
		executed = nil
	}
}

func TestBottomTerminalUIAndFocus(t *testing.T) {
	a, screen := simulate(t, config.Document{Version: config.Version, Clusters: []config.Cluster{fixture("alice", "dev")}},
		func(config.Document) error { return nil })
	a.runSection = a.startSection
	a.requestExecute(localCommand("printf 'READY\\n'; read answer; printf 'ANSWER:%s\\n' \"$answer\""))
	waitText(t, a.terminal, "READY")
	if !a.terminalVisible || a.application.GetFocus() != a.terminal {
		t.Fatal("Execute did not open/focus bottom pane")
	}
	_, terminalY, terminalWidth, _ := a.terminal.GetRect()
	listX, _, _, listHeight := a.list.GetRect()
	screenWidth, _ := screen.Size()
	if terminalWidth != screenWidth || terminalY <= listHeight || listX != 0 {
		t.Fatalf("terminal is not below both panes: y=%d, width=%d", terminalY, terminalWidth)
	}
	press(a, tcell.KeyRune, 'n')
	press(a, tcell.KeyEnter, 0)
	if result := waitTerminal(t, a.terminal); result != "Exit 0" || a.editing {
		t.Fatalf("terminal typing intercepted by global shortcut: %s", result)
	}
	a.application.ForceDraw()
	if !strings.Contains(screenText(screen), "ANSWER:n") {
		t.Fatalf("output not rendered in pane:\n%s", screenText(screen))
	}
	press(a, tcell.KeyEscape, 0)
	if a.application.GetFocus() != a.preview {
		t.Fatal("Escape did not return to panes")
	}
	press(a, tcell.KeyF2, 0)
	if a.terminalVisible {
		t.Fatal("F2 did not hide pane")
	}
	press(a, tcell.KeyF2, 0)
	if !a.terminalVisible || !strings.Contains(a.terminal.String(), "ANSWER:n") {
		t.Fatal("F2 did not restore terminal output")
	}
	for _, size := range [][2]int{{80, 24}, {140, 40}} {
		screen.SetSize(size[0], size[1])
		a.application.ForceDraw()
		_, y, width, height := a.terminal.GetRect()
		if width != size[0] || height != max(8, size[1]/3) || y+height != size[1]-1 {
			t.Fatalf("terminal geometry after resize: %d,%d,%d at %v", y, width, height, size)
		}
	}
}
