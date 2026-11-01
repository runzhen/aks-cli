package ui

import (
	"errors"
	"fmt"

	"aks-cli/internal/clipboard"
	"aks-cli/internal/commands"
	"aks-cli/internal/config"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	minWidth  = 80
	minHeight = 24
)

type App struct {
	application     *tview.Application
	pages           *tview.Pages
	list            *clusterList
	preview         *preview
	status          *tview.TextView
	copyText        func(string) error
	newButton       *tview.Button
	form            *tview.Form
	formError       *tview.TextView
	doc             config.Document
	save            func(config.Document) error
	defaults        config.Cluster
	editing         bool
	compact         bool
	terminal        *terminalPane
	split           *tview.Flex
	terminalVisible bool
	confirming      bool
	screen          tcell.Screen
	runSection      func(commands.Section)
	drafts          map[config.Cluster]map[string]string
}

func New(doc config.Document, save func(config.Document) error, defaults config.Cluster) *App {
	a := &App{
		application: tview.NewApplication(),
		pages:       tview.NewPages(),
		doc:         doc,
		save:        save,
		defaults:    defaults,
		copyText:    clipboard.Write,
		drafts:      make(map[config.Cluster]map[string]string),
	}
	a.preview = newPreview(func(p tview.Primitive) { a.application.SetFocus(p) })
	a.terminal = newTerminalPane()
	a.runSection = a.startSection
	a.status = tview.NewTextView().SetDynamicColors(false).SetWordWrap(true)
	a.list = newClusterList(a.removeCluster)
	a.list.SetBorder(true).SetTitle(" Saved configurations ")
	a.list.SetChangedFunc(func(index int, _, _ string, _ rune) { a.selectCluster(index) })
	a.newButton = tview.NewButton("New").SetSelectedFunc(a.showForm)
	a.newButton.SetBorder(true)

	left := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.newButton, 3, 0, false).
		AddItem(a.list, 0, 1, true)
	commandHeading := tview.NewTextView().SetText(" ")
	right := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(commandHeading, 1, 0, false).
		AddItem(a.preview, 0, 1, false).AddItem(a.status, 2, 0, false)
	body := tview.NewFlex().AddItem(left, 0, 1, true).AddItem(right, 0, 2, false)
	a.split = tview.NewFlex().SetDirection(tview.FlexRow).AddItem(body, 0, 1, true)
	header := tview.NewTextView().SetText(" aks-cli | Execute runs real commands | x removes local config only")
	footer := tview.NewTextView().SetText(" n: New | Tab: focus | F2: terminal | Esc: panes | Ctrl-C: interrupt/quit | Ctrl-Q: quit")
	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 1, 0, false).
		AddItem(a.split, 0, 1, true).
		AddItem(footer, 1, 0, false)
	a.pages.AddPage("main", root, true, true)
	a.application.SetRoot(a.pages, true).EnableMouse(true).EnablePaste(true)
	a.application.SetInputCapture(a.handleKey)
	a.application.SetMouseCapture(a.handleMouse)

	small := tview.NewTextView().SetTextAlign(tview.AlignCenter).
		SetText(fmt.Sprintf("Terminal too small.\nResize to at least %dx%d.\nCtrl-C: quit", minWidth, minHeight))
	a.application.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		a.screen = screen
		width, height := screen.Size()
		if a.terminalVisible {
			a.split.ResizeItem(a.terminal, max(8, height/3), 0)
		}
		a.compact = width < minWidth || height < minHeight
		if !a.compact {
			return false
		}
		screen.Clear()
		small.SetRect(0, 0, width, height)
		small.Draw(screen)
		return true
	})
	a.refresh()
	if len(doc.Clusters) == 0 {
		a.application.SetFocus(a.newButton)
	} else {
		a.application.SetFocus(a.list)
	}
	return a
}

func (a *App) Run() error {
	if a.screen == nil {
		screen, err := tcell.NewScreen()
		if err != nil {
			return err
		}
		input := &inputScreen{Screen: screen}
		if err := input.Init(); err != nil {
			return err
		}
		a.application.SetScreen(input)
		input.EnableMouse()
		input.EnablePaste()
	}
	err := a.application.Run()
	return errors.Join(err, a.terminal.close())
}

func (a *App) refresh() {
	a.list.Clear()
	for _, c := range a.doc.Clusters {
		a.list.AddItem(tview.Escape(c.ClusterName),
			tview.Escape(c.Subscription+" / "+c.ResourceGroup), 0, nil)
	}
	a.selectCluster(0)
}

func (a *App) selectCluster(index int) {
	a.status.SetText("")
	if len(a.doc.Clusters) == 0 {
		a.preview.setSections(nil, nil, a.copySection, a.requestExecute)
		return
	}
	if index < 0 || index >= len(a.doc.Clusters) {
		return
	}
	cluster := a.doc.Clusters[index]
	if a.drafts[cluster] == nil {
		a.drafts[cluster] = make(map[string]string)
	}
	a.preview.setSections(commands.Sections(cluster), a.drafts[cluster], a.copySection, a.requestExecute)
}

func (a *App) copySection(section commands.Section) {
	if err := a.copyText(section.Content); err != nil {
		a.status.SetText("Copy failed: " + err.Error())
		return
	}
	a.status.SetText("Copied " + section.Title + " to system clipboard.")
}

func (a *App) removeCluster(index int) {
	if index < 0 || index >= len(a.doc.Clusters) {
		a.status.SetText("Cannot remove configuration: item is no longer available.")
		return
	}
	next := config.Document{Version: a.doc.Version, Clusters: make([]config.Cluster, 0, len(a.doc.Clusters)-1)}
	next.Clusters = append(next.Clusters, a.doc.Clusters[:index]...)
	next.Clusters = append(next.Clusters, a.doc.Clusters[index+1:]...)
	if err := a.save(next); err != nil {
		a.status.SetText("Cannot remove configuration: " + err.Error())
		return
	}
	selected := a.list.GetCurrentItem()
	if index < selected {
		selected--
	}
	delete(a.drafts, a.doc.Clusters[index])
	a.doc = next
	a.refresh()
	if len(next.Clusters) == 0 {
		a.application.SetFocus(a.newButton)
	} else {
		selected = min(selected, len(next.Clusters)-1)
		a.list.SetCurrentItem(selected)
		a.selectCluster(selected)
		a.application.SetFocus(a.list)
	}
	a.status.SetText("Removed saved configuration. Azure resources were not changed.")
}

func (a *App) handleKey(event *tcell.EventKey) *tcell.EventKey {
	// PTY updates use a nonblocking screen event so output cannot strand a
	// QueueUpdate callback when the UI exits.
	if event.Key() == tcell.KeyF64 {
		return nil
	}
	if event.Key() == tcell.KeyCtrlQ {
		a.application.Stop()
		return nil
	}
	if event.Key() == tcell.KeyCtrlC {
		if running, _ := a.terminal.state(); running {
			a.terminal.send("\x03")
		} else {
			a.application.Stop()
		}
		return nil
	}
	if a.compact {
		return nil
	}
	if a.confirming {
		if event.Key() == tcell.KeyEscape {
			a.dismissConfirmation()
			return nil
		}
		return event
	}
	if a.editing {
		if event.Key() == tcell.KeyEscape {
			a.closeForm()
			return nil
		}
		if isSelectAll(event) {
			for i := 0; i < a.form.GetFormItemCount(); i++ {
				if field, ok := a.form.GetFormItem(i).(*tview.InputField); ok && field.HasFocus() {
					// tview's native select-all binding preserves selection,
					// replacement, paste, and undo without changing the text.
					return tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModNone)
				}
			}
		}
		return event
	}
	if event.Key() == tcell.KeyF2 {
		a.toggleTerminal()
		return nil
	}
	if a.application.GetFocus() == a.terminal {
		if event.Key() == tcell.KeyEscape {
			a.application.SetFocus(a.preview)
			return nil
		}
		return event
	}

	if a.preview.editorFocused() {
		switch {
		case event.Key() == tcell.KeyEscape:
			a.application.SetFocus(a.preview)
			return nil
		case isSelectAll(event):
			return tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModNone)
		case event.Key() != tcell.KeyTab && event.Key() != tcell.KeyBacktab &&
			event.Key() != tcell.KeyPgUp && event.Key() != tcell.KeyPgDn:
			return event
		}
	}
	if event.Key() == tcell.KeyRune && event.Rune() == 'n' {
		a.showForm()
		return nil
	}
	if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyBacktab {
		focus := append([]tview.Primitive{a.newButton, a.list}, a.preview.focusables()...)
		if a.terminalVisible {
			focus = append(focus, a.terminal)
		}
		current := 0
		for i, item := range focus {
			if a.application.GetFocus() == item {
				current = i
				break
			}
		}
		delta := 1
		if event.Key() == tcell.KeyBacktab {
			delta = -1
		}
		a.application.SetFocus(focus[(current+delta+len(focus))%len(focus)])
		return nil
	}
	if a.preview.HasFocus() && (event.Key() == tcell.KeyDown || event.Key() == tcell.KeyUp ||
		event.Key() == tcell.KeyPgDn || event.Key() == tcell.KeyPgUp) {
		offset, _ := a.preview.GetOffset()
		if event.Key() == tcell.KeyDown || event.Key() == tcell.KeyPgDn {
			offset++
		} else {
			offset--
		}
		a.application.SetFocus(a.preview)
		a.preview.scrollTo(offset)
		return nil
	}
	return event
}

func (a *App) handleMouse(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
	if a.compact {
		return nil, action
	}
	if a.editing && action == tview.MouseLeftDoubleClick {
		for i := 0; i < a.form.GetFormItemCount(); i++ {
			field, ok := a.form.GetFormItem(i).(*tview.InputField)
			if ok && field.InRect(event.Position()) {
				a.application.SetFocus(field)
				field.InputHandler()(tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModNone),
					func(p tview.Primitive) { a.application.SetFocus(p) })
				return nil, tview.MouseConsumed
			}
		}
	}
	return event, action
}
