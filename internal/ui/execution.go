package ui

import (
	"aks-cli/internal/commands"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (a *App) requestExecute(section commands.Section) {
	if strings.TrimSpace(section.Content) == "" {
		a.status.SetText("Cannot execute an empty command.")
		return
	}
	if running, _ := a.terminal.state(); running {
		a.status.SetText("A command is already running. Use Ctrl-C to interrupt it, or wait.")
		a.showTerminal()
		return
	}
	if !section.Edited && !section.Command.Destructive() {
		a.runSection(section)
		return
	}
	a.confirming = true
	title := "Run this destructive command?"
	if section.Edited {
		title = "Run edited text with /bin/sh? It may execute arbitrary commands."
	}
	text := section.Content
	if section.Warning != "" {
		text += "\n\n" + section.Warning
	}
	content := tview.NewTextView().SetDynamicColors(false).SetText(text)
	content.SetBorder(true).SetTitle(" Review command - PgUp/PgDn to scroll ")
	buttons := tview.NewForm().
		AddButton("Cancel", a.dismissConfirmation).
		AddButton("Execute", func() {
			a.dismissConfirmation()
			a.runSection(section)
		})
	page := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tview.NewTextView().SetText(title), 2, 0, false).
		AddItem(content, 0, 1, false).
		AddItem(buttons, 3, 0, true)
	page.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyPgUp || event.Key() == tcell.KeyPgDn {
			content.InputHandler()(event, func(p tview.Primitive) { a.application.SetFocus(p) })
			return nil
		}
		return event
	})
	a.pages.AddPage("confirm-execute", page, true, true)
	a.application.SetFocus(buttons)
}

func (a *App) dismissConfirmation() {
	a.confirming = false
	a.pages.RemovePage("confirm-execute")
	a.application.SetFocus(a.preview)
}

func (a *App) showTerminal() {
	if !a.terminalVisible {
		a.terminalVisible = true
		a.split.AddItem(a.terminal, 10, 0, false)
	}
	a.application.SetFocus(a.terminal)
}

func (a *App) toggleTerminal() {
	if a.terminalVisible {
		a.split.RemoveItem(a.terminal)
		a.terminalVisible = false
		a.application.SetFocus(a.preview)
	} else {
		a.showTerminal()
	}
}

func (a *App) startSection(section commands.Section) {
	a.showTerminal()
	// Lay out the newly opened pane before starting the child so its initial
	// PTY dimensions match the visible terminal.
	a.application.ForceDraw()
	screen := a.screen
	if screen == nil {
		a.status.SetText("Cannot execute: terminal screen is not available.")
		return
	}
	if err := a.terminal.start(section, func() {
		// A full event queue already guarantees a pending redraw.
		_ = screen.PostEvent(tcell.NewEventKey(tcell.KeyF64, 0, tcell.ModNone))
	}); err != nil {
		a.status.SetText("Cannot execute: " + err.Error())
		return
	}
	a.status.SetText("Started " + section.Title + ". Output and exit status appear in the terminal.")
}
