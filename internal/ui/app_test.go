package ui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aks-cli/internal/commands"
	"aks-cli/internal/config"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func fixture(name, subscription string) config.Cluster {
	c := config.Defaults(name)
	c.Subscription = subscription
	return c
}

func simulate(t *testing.T, doc config.Document, save func(config.Document) error) (*App, tcell.SimulationScreen) {
	t.Helper()
	a := New(doc, save, config.Defaults("alice"))
	a.runSection = func(section commands.Section) {
		t.Errorf("unexpected command execution in UI test: %s", section.Title)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	a.application.SetScreen(screen)
	screen.SetSize(120, 32)
	a.application.ForceDraw()
	t.Cleanup(func() {
		a.application.Stop()
		if err := a.terminal.close(); err != nil {
			t.Error(err)
		}
	})
	return a, screen
}

func press(a *App, key tcell.Key, r rune) {
	event := a.handleKey(tcell.NewEventKey(key, r, tcell.ModNone))
	if event != nil {
		a.pages.InputHandler()(event, func(p tview.Primitive) { a.application.SetFocus(p) })
	}
	a.application.ForceDraw()
}

func click(a *App, x, y int) {
	a.pages.MouseHandler()(tview.MouseLeftClick, tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone),
		func(p tview.Primitive) { a.application.SetFocus(p) })
	a.application.ForceDraw()
}

func screenText(screen tcell.SimulationScreen) string {
	cells, width, _ := screen.GetContents()
	var text strings.Builder
	for i, cell := range cells {
		for _, r := range cell.Runes {
			text.WriteRune(r)
		}
		if (i+1)%width == 0 {
			text.WriteByte('\n')
		}
	}
	return text.String()
}

func previewText(a *App) string {
	var parts []string
	for _, section := range a.preview.sections {
		parts = append(parts, section.text.GetText(false))
	}
	return strings.Join(parts, "\n")
}

func TestSelectionFocusMouseAndScrolling(t *testing.T) {
	doc := config.Document{Version: config.Version, Clusters: []config.Cluster{
		fixture("first", "[red]dev"+strings.Repeat("-long-name", 20)), fixture("second", "prod"),
	}}
	a, screen := simulate(t, doc, func(config.Document) error { return nil })
	if !strings.Contains(previewText(a), "first-gpu") {
		t.Fatal("first cluster not selected")
	}
	if !strings.Contains(screenText(screen), "[red]dev") {
		t.Fatal("subscription markup was not displayed literally")
	}
	press(a, tcell.KeyDown, 0)
	if a.list.GetCurrentItem() != 1 || !strings.Contains(previewText(a), "second-gpu") {
		t.Fatal("keyboard selection did not refresh preview")
	}
	x, y, _, _ := a.list.GetInnerRect()
	click(a, x, y)
	if a.list.GetCurrentItem() != 0 || !strings.Contains(previewText(a), "first-gpu") {
		t.Fatal("mouse selection did not refresh preview")
	}
	press(a, tcell.KeyTab, 0)
	if a.application.GetFocus() != a.preview {
		t.Fatal("Tab did not focus preview")
	}
	press(a, tcell.KeyPgDn, 0)
	row, _ := a.preview.GetOffset()
	if row == 0 {
		t.Fatal("preview did not scroll vertically")
	}
	press(a, tcell.KeyTab, 0)
	press(a, tcell.KeyRight, 0)
	_, column := a.preview.sections[0].text.GetScrollOffset()
	if column == 0 {
		t.Fatal("preview did not scroll horizontally")
	}
	press(a, tcell.KeyBacktab, 0)
	press(a, tcell.KeyBacktab, 0)
	if a.application.GetFocus() != a.list {
		t.Fatal("Shift-Tab did not focus list")
	}
	press(a, tcell.KeyDown, 0)
	row, column = a.preview.GetOffset()
	if row != 0 || column != 0 {
		t.Fatal("switching clusters did not reset scroll")
	}
}

func TestFormSaveCancelAndRestart(t *testing.T) {
	store := config.Store{Path: filepath.Join(t.TempDir(), ".config", "aks-cli.json")}
	a, screen := simulate(t, config.Empty(), store.Save)
	if !strings.Contains(screenText(screen), "No saved configurations") {
		t.Fatal("empty state not visible")
	}
	x, y, _, _ := a.newButton.GetInnerRect()
	click(a, x, y)
	if !a.editing || a.form.GetFormItemCount() != 6 {
		t.Fatal("New button did not open six-field form")
	}
	a.form.GetFormItem(0).(*tview.InputField).SetText("Development")
	a.form.SetFocus(6)
	a.application.SetFocus(a.form)
	press(a, tcell.KeyEnter, 0)
	if a.editing || a.list.GetItemCount() != 1 {
		t.Fatalf("Save did not close and add entry: %s", a.formError.GetText(false))
	}
	press(a, tcell.KeyRune, 'n')
	originalForm := a.form
	press(a, tcell.KeyRune, 'n')
	if a.form != originalForm || a.form.GetFormItem(0).(*tview.InputField).GetText() != "n" {
		t.Fatal("New shortcut intercepted typing in a form")
	}
	a.form.GetFormItem(0).(*tview.InputField).SetText("Production")
	a.form.SetFocus(6)
	a.application.SetFocus(a.form)
	press(a, tcell.KeyEnter, 0)
	if a.editing || a.list.GetItemCount() != 2 || a.list.GetCurrentItem() != 1 ||
		!strings.Contains(previewText(a), "Production") {
		t.Fatal("second save did not add/select/preview the new configuration")
	}
	press(a, tcell.KeyRune, 'n')
	press(a, tcell.KeyEscape, 0)
	if a.editing || a.list.GetItemCount() != 2 {
		t.Fatal("Escape did not cancel without saving")
	}
	press(a, tcell.KeyRune, 'n')
	a.form.SetFocus(7)
	a.application.SetFocus(a.form)
	press(a, tcell.KeyEnter, 0)
	if a.editing || a.list.GetItemCount() != 2 {
		t.Fatal("Cancel button modified configurations")
	}
	reloaded, err := store.Load()
	if err != nil || len(reloaded.Clusters) != 2 {
		t.Fatalf("reload failed: %+v, %v", reloaded, err)
	}
	restarted, _ := simulate(t, reloaded, store.Save)
	if restarted.list.GetItemCount() != 2 || !strings.Contains(previewText(restarted), "Development") {
		t.Fatal("restart did not restore configurations")
	}
	press(restarted, tcell.KeyDown, 0)
	if !strings.Contains(previewText(restarted), "Production") {
		t.Fatal("reloaded configurations not selectable")
	}
}

func TestFormErrorsRetainInputAndSavedState(t *testing.T) {
	calls := 0
	a, _ := simulate(t, config.Empty(), func(config.Document) error {
		calls++
		return errors.New("disk is read-only")
	})
	press(a, tcell.KeyRune, 'n')
	a.saveDraft(config.Defaults("alice"))
	if calls != 0 || !a.editing || !strings.Contains(a.formError.GetText(false), "SUBSCRIPTION") {
		t.Fatal("validation error did not retain form or attempted save")
	}
	a.form.GetFormItem(0).(*tview.InputField).SetText("Development")
	a.form.SetFocus(6)
	a.application.SetFocus(a.form)
	press(a, tcell.KeyEnter, 0)
	if calls != 1 || !a.editing || a.list.GetItemCount() != 0 || len(a.doc.Clusters) != 0 ||
		!strings.Contains(a.formError.GetText(false), "disk is read-only") ||
		a.form.GetFormItem(0).(*tview.InputField).GetText() != "Development" {
		t.Fatal("save failure lost input, changed saved state, or hid the error")
	}
	a.save = func(config.Document) error { return nil }
	press(a, tcell.KeyEnter, 0)
	if a.editing || len(a.doc.Clusters) != 1 {
		t.Fatal("could not retry failed save")
	}
	press(a, tcell.KeyRune, 'n')
	a.saveDraft(fixture("alice", "development"))
	if !a.editing || !strings.Contains(a.formError.GetText(false), "duplicates") {
		t.Fatal("duplicate did not produce an error")
	}
}

func TestResizeAndFormVisibility(t *testing.T) {
	a, screen := simulate(t, config.Empty(), func(config.Document) error { return nil })
	screen.SetSize(minWidth, minHeight)
	press(a, tcell.KeyRune, 'n')
	for _, label := range []string{"SUBSCRIPTION", "LOCATION", "RESOURCE_GROUP", "GPU_POOL", "CLUSTER_NAME", "GPU_VM_SIZE", "Save", "Cancel"} {
		if !strings.Contains(screenText(screen), label) {
			t.Errorf("%s not visible at minimum terminal size", label)
		}

	}
	screen.SetSize(40, 10)
	a.application.ForceDraw()
	if !a.compact || !strings.Contains(screenText(screen), "Terminal too small") {
		t.Fatal("small-terminal message missing")
	}
	press(a, tcell.KeyEscape, 0)
	if !a.editing {
		t.Fatal("hidden form accepted input while terminal was too small")
	}
	screen.SetSize(120, 32)
	a.application.ForceDraw()
	if a.compact || !strings.Contains(screenText(screen), "GPU_VM_SIZE") {
		t.Fatal("form did not recover after resize")
	}
	press(a, tcell.KeyEscape, 0)
	if a.editing {
		t.Fatal("form could not be cancelled after resize")
	}
}

func TestSectionButtonsCopyAndExecuteDispatch(t *testing.T) {
	first, second := fixture("first", "Dev's [red] subscription"), fixture("second", "prod")
	a, screen := simulate(t, config.Document{Version: config.Version, Clusters: []config.Cluster{first, second}},
		func(config.Document) error { t.Fatal("section buttons must not save"); return nil })
	var copied []string
	var executed []commands.Section
	a.runSection = func(section commands.Section) { executed = append(executed, section) }
	a.copyText = func(text string) error {
		copied = append(copied, text)
		return nil
	}
	for _, size := range [][2]int{{120, 32}, {minWidth, minHeight}} {
		screen.SetSize(size[0], size[1])
		for i, section := range a.preview.sections {
			a.application.SetFocus(section.copy)
			a.application.ForceDraw()
			visible := screenText(screen)
			for _, label := range []string{section.data.Title, "Execute", "Copy"} {
				if !strings.Contains(visible, label) {
					t.Fatalf("section %d at %v missing %q: %s", i, size, label, visible)
				}
			}
			x, y, _, _ := section.copy.GetInnerRect()
			before := len(copied)
			click(a, x, y)
			if len(copied) != before+1 || copied[len(copied)-1] != section.data.Content {
				t.Fatalf("section %d copied wrong text: %v", i, copied)
			}
			if !strings.Contains(a.status.GetText(false), "Copied "+section.data.Title) {
				t.Fatal("copy confirmation missing")
			}
			runsBefore := len(executed)
			x, y, _, _ = section.execute.GetInnerRect()
			click(a, x, y)
			if section.data.Command.Destructive() {
				if !a.confirming || len(executed) != runsBefore {
					t.Fatal("delete command did not require confirmation")
				}
				press(a, tcell.KeyEscape, 0)
			} else if len(executed) != runsBefore+1 || executed[len(executed)-1].Content != section.data.Content {
				t.Fatal("Execute did not dispatch the displayed section")
			}
			runsBefore = len(executed)
			a.application.SetFocus(section.execute)
			press(a, tcell.KeyEnter, 0)
			if section.data.Command.Destructive() {
				if !a.confirming || len(executed) != runsBefore {
					t.Fatal("keyboard delete bypassed confirmation")
				}
				press(a, tcell.KeyEscape, 0)
			} else if len(executed) != runsBefore+1 {
				t.Fatal("keyboard Execute did not dispatch")
			}
			if len(copied) != before+1 {
				t.Fatal("Execute modified the clipboard")
			}
		}
	}
	a.application.SetFocus(a.list)
	press(a, tcell.KeyDown, 0)
	want := commands.Sections(second)
	for i, section := range a.preview.sections {
		a.application.SetFocus(section.copy)
		press(a, tcell.KeyEnter, 0)
		if copied[len(copied)-1] != want[i].Content {
			t.Fatalf("section %d, selected %d: copied %q, want %q", i, a.list.GetCurrentItem(), copied[len(copied)-1], want[i].Content)
		}
	}
	a.copyText = func(string) error { return errors.New("clipboard unavailable") }
	press(a, tcell.KeyEnter, 0)
	if !strings.Contains(a.status.GetText(false), "Copy failed: clipboard unavailable") {
		t.Fatal("clipboard failure not surfaced")
	}
}

func TestPreviewMouseScrollAndKeyboardButtons(t *testing.T) {
	a, _ := simulate(t, config.Document{Version: config.Version, Clusters: []config.Cluster{fixture("alice", "dev")}},
		func(config.Document) error { return nil })
	press(a, tcell.KeyTab, 0)
	for _, section := range a.preview.sections {
		for _, target := range []tview.Primitive{section.text, section.execute, section.copy} {
			press(a, tcell.KeyTab, 0)
			if a.application.GetFocus() != target {
				t.Fatal("Tab skipped a section control")
			}
		}
	}
	press(a, tcell.KeyTab, 0)
	if a.application.GetFocus() != a.newButton {
		t.Fatal("focus did not wrap to New")
	}
	a.application.SetFocus(a.preview.sections[0].copy)
	a.application.ForceDraw()
	x, y, _, _ := a.preview.GetInnerRect()
	for i := 0; i < 3; i++ {
		consumed, _ := a.pages.MouseHandler()(tview.MouseScrollDown, tcell.NewEventMouse(x, y, tcell.WheelDown, tcell.ModNone),
			func(p tview.Primitive) { a.application.SetFocus(p) })
		if !consumed {
			t.Fatal("mouse scroll must be consumed to trigger a redraw")
		}
		a.application.ForceDraw()
	}
	row, _ := a.preview.GetOffset()
	if row == 0 {
		t.Fatal("mouse wheel did not scroll past focused section")
	}
	copied := ""
	a.copyText = func(text string) error { copied = text; return nil }
	var visible *previewSection
	for i := range a.preview.sections {
		section := &a.preview.sections[i]
		_, _, width, height := section.panel.GetRect()
		if width > 0 && height > 0 {
			visible = section
			break
		}
	}
	if visible == nil {
		t.Fatal("no visible section after scrolling")
	}
	x, y, _, _ = visible.copy.GetInnerRect()
	click(a, x, y)
	if copied != visible.data.Content {
		t.Fatal("mouse click hit a hidden section's stale button")
	}
}

func TestPreviewArrowKeysFromEveryControl(t *testing.T) {
	a, _ := simulate(t, config.Document{Version: config.Version, Clusters: []config.Cluster{fixture("alice", "dev")}},
		func(config.Document) error { return nil })
	for _, target := range a.preview.focusables() {
		a.application.SetFocus(target)
		a.application.ForceDraw()
		before, _ := a.preview.GetOffset()
		press(a, tcell.KeyDown, 0)
		down, _ := a.preview.GetOffset()
		if down != min(before+1, len(a.preview.sections)-1) {
			t.Fatalf("Down from %T: offset %d -> %d", target, before, down)
		}
		if a.application.GetFocus() != a.preview {
			t.Fatalf("Down from %T did not transfer focus to preview", target)
		}
		press(a, tcell.KeyUp, 0)
		up, _ := a.preview.GetOffset()
		if up != max(down-1, 0) {
			t.Fatalf("Up from %T: offset %d -> %d", target, down, up)
		}
	}
}

func TestPreviewScrollRedrawsInEventLoop(t *testing.T) {
	a, screen := simulate(t, config.Document{Version: config.Version, Clusters: []config.Cluster{fixture("alice", "dev")}},
		func(config.Document) error { return nil })
	a.application.SetFocus(a.preview.sections[0].copy)
	type frame struct {
		offset int
		text   string
	}
	frames := make(chan frame, 16)
	a.application.SetAfterDrawFunc(func(tcell.Screen) {
		offset, _ := a.preview.GetOffset()
		var text strings.Builder
		width, height := screen.Size()
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				r, _, _, _ := screen.GetContent(x, y)
				text.WriteRune(r)
			}
			text.WriteByte('\n')
		}
		frames <- frame{offset, text.String()}
	})
	done := make(chan error, 1)
	go func() { done <- a.Run() }()
	defer func() {
		a.application.Stop()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("application event loop did not stop")
		}
	}()
	nextFrame := func(want int, title string) {
		t.Helper()
		select {
		case got := <-frames:
			if got.offset != want || !strings.Contains(got.text, title) {
				t.Fatalf("expected offset %d and %q, got offset %d:\n%s", want, title, got.offset, got.text)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("scroll event did not redraw")
		}
	}
	nextFrame(0, "Variables")
	a.application.QueueEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	nextFrame(1, "az group create")
	a.application.QueueEvent(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	nextFrame(0, "Variables")
	a.application.QueueEvent(tcell.NewEventMouse(60, 5, tcell.WheelDown, tcell.ModNone))
	nextFrame(1, "az group create")
	a.application.QueueEvent(tcell.NewEventMouse(60, 5, tcell.WheelUp, tcell.ModNone))
	nextFrame(0, "Variables")
}
