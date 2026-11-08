package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aks-cli/internal/commands"
	"aks-cli/internal/config"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestCommandEditorsUseCurrentText(t *testing.T) {
	a, _ := simulate(t, config.Document{Version: config.Version, Clusters: []config.Cluster{fixture("alice", "dev")}},
		func(config.Document) error { t.Fatal("editing a command must not save configuration"); return nil })
	var copied string
	var executed []commands.Section
	a.copyText = func(text string) error { copied = text; return nil }
	a.runSection = func(section commands.Section) { executed = append(executed, section) }
	for i, section := range a.preview.sections {
		a.application.SetFocus(section.text)
		a.application.ForceDraw()
		press(a, tcell.KeyCtrlA, 0)
		text := "printf '%s\\n' 'edited command'\nprintf '%s\\n' 'second line'"
		a.pages.PasteHandler()(text, func(p tview.Primitive) { a.application.SetFocus(p) })
		if section.text.GetText() != text || a.editing {
			t.Fatalf("editor %d failed to replace pasted content", i)
		}
		if !strings.HasSuffix(section.panel.GetTitle(), "* ") {
			t.Fatal("edited section was not marked")
		}
		a.application.SetFocus(section.copy)
		press(a, tcell.KeyEnter, 0)
		if copied != text {
			t.Fatal("Copy used stale generated content")
		}
		before := len(executed)
		a.application.SetFocus(section.execute)
		press(a, tcell.KeyEnter, 0)
		if !a.confirming || len(executed) != before {
			t.Fatal("edited execution bypassed review")
		}
		press(a, tcell.KeyTab, 0)
		press(a, tcell.KeyEnter, 0)
		if len(executed) != before+1 {
			t.Fatal("confirmed editor did not execute")
		}
		run := executed[len(executed)-1]
		if run.Content != text || !run.Edited || run.Command.Executable != "/bin/sh" ||
			!reflect.DeepEqual(run.Command.Args, []string{"-c", text}) {
			t.Fatalf("wrong edited command dispatched: %+v", run)
		}
	}
}

func TestEditorTypingAndNavigation(t *testing.T) {
	a, _ := simulate(t, config.Document{Version: config.Version, Clusters: []config.Cluster{fixture("alice", "dev")}},
		func(config.Document) error { return nil })
	editor := a.preview.sections[0].text
	a.application.SetFocus(editor)
	a.application.ForceDraw()
	press(a, tcell.KeyCtrlA, 0)
	press(a, tcell.KeyRune, 'n')
	press(a, tcell.KeyEnter, 0)
	press(a, tcell.KeyRune, 'x')
	if editor.GetText() != "n\nx" || a.editing {
		t.Fatal("editor input triggered a New form or failed to insert newline")
	}
	press(a, tcell.KeyUp, 0)
	_, _, row, _ := editor.GetCursor()
	if row != 0 || a.application.GetFocus() != editor {
		t.Fatal("Up did not move the editing cursor")
	}
	press(a, tcell.KeyTab, 0)
	if a.application.GetFocus() != a.preview.sections[0].execute {
		t.Fatal("Tab did not focus Execute")
	}
	press(a, tcell.KeyBacktab, 0)
	press(a, tcell.KeyEscape, 0)
	if a.application.GetFocus() != a.preview {
		t.Fatal("Escape did not leave the editor")
	}
}

func TestEditorDraftsStayWithClusterUntilRestart(t *testing.T) {
	store := config.Store{Path: filepath.Join(t.TempDir(), "aks-cli.json")}
	doc := config.Document{Version: config.Version, Clusters: []config.Cluster{fixture("same", "dev"), fixture("same", "prod")}}
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := simulate(t, doc, store.Save)
	a.preview.sections[0].text.SetText("printf first", false)
	a.application.SetFocus(a.list)
	press(a, tcell.KeyDown, 0)
	defaultSecond := a.preview.sections[0].data.Content
	if a.preview.sections[0].text.GetText() != defaultSecond {
		t.Fatal("draft leaked to another cluster")
	}
	a.preview.sections[0].text.SetText("printf second", false)
	press(a, tcell.KeyUp, 0)
	if a.preview.sections[0].text.GetText() != "printf first" {
		t.Fatal("switching clusters discarded draft")
	}
	a.refresh()
	if a.preview.sections[0].text.GetText() != "printf first" {
		t.Fatal("refresh discarded draft")
	}
	after, err := os.ReadFile(store.Path)
	if err != nil || string(before) != string(after) {
		t.Fatal("command edits changed aks-cli.json")
	}
	restarted, _ := simulate(t, doc, store.Save)
	if restarted.preview.sections[0].text.GetText() != restarted.preview.sections[0].data.Content {
		t.Fatal("session-only draft survived restart")
	}
	a.removeCluster(0)
	if _, exists := a.drafts[doc.Clusters[0]]; exists {
		t.Fatal("deleting a configuration retained its draft")
	}
}

func TestEditedCommandExecutesAfterConfirmation(t *testing.T) {
	a, _ := simulate(t, config.Document{Version: config.Version, Clusters: []config.Cluster{fixture("alice", "dev")}},
		func(config.Document) error { return nil })
	a.runSection = a.startSection
	section := a.preview.sections[0]
	section.text.SetText("printf 'EDITED_RESULT\\n'; exit 9", false)
	a.application.SetFocus(section.execute)
	press(a, tcell.KeyEnter, 0)
	if !a.confirming {
		t.Fatal("edited shell command was not reviewed")
	}
	press(a, tcell.KeyTab, 0)
	press(a, tcell.KeyEnter, 0)
	if result := waitTerminal(t, a.terminal); !strings.Contains(result, "exit status 9") ||
		!strings.Contains(a.terminal.String(), "EDITED_RESULT") {
		t.Fatalf("edited command did not run: %s\n%s", result, a.terminal.String())
	}
}

func TestEditorEmptyCancelAndRestoredDefaults(t *testing.T) {
	a, _ := simulate(t, config.Document{Version: config.Version, Clusters: []config.Cluster{fixture("alice", "dev")}},
		func(config.Document) error { return nil })
	var executed []commands.Section
	a.runSection = func(section commands.Section) { executed = append(executed, section) }
	section := a.preview.sections[0]
	section.text.SetText(" \n\t", false)
	a.application.SetFocus(section.execute)
	press(a, tcell.KeyEnter, 0)
	if a.confirming || len(executed) != 0 || !strings.Contains(a.status.GetText(false), "empty command") {
		t.Fatal("empty command was not rejected")
	}
	section.text.SetText("az aks delete --name different", false)
	a.application.SetFocus(section.execute)
	press(a, tcell.KeyEnter, 0)
	press(a, tcell.KeyEnter, 0)
	if a.confirming || len(executed) != 0 {
		t.Fatal("default Cancel ran edited command")
	}
	section.text.SetText(section.data.Content, false)
	a.application.SetFocus(section.execute)
	press(a, tcell.KeyEnter, 0)
	if a.confirming || len(executed) != 1 || executed[0].Edited ||
		!reflect.DeepEqual(executed[0].Command, section.data.Command) {
		t.Fatal("restoring generated text did not restore structured execution")
	}
}
