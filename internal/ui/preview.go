package ui

import (
	"strings"

	"aks-cli/internal/commands"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type previewSection struct {
	data    commands.Section
	panel   *tview.Flex
	text    *tview.TextView
	execute *tview.Button
	copy    *tview.Button
}

type preview struct {
	*tview.Grid
	sections []previewSection
}

func newPreview(focus func(tview.Primitive)) *preview {
	p := &preview{Grid: tview.NewGrid().SetColumns(0).SetGap(1, 0)}
	p.SetBorder(true).SetTitle(" Sections - Execute is inactive ")
	p.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if !p.InRect(event.Position()) {
			return action, event
		}
		if action == tview.MouseScrollDown || action == tview.MouseScrollUp {
			focus(p)
			offset, _ := p.GetOffset()
			if action == tview.MouseScrollDown {
				offset++
			} else {
				offset--
			}
			p.scrollTo(offset)
			return action, nil
		}
		return action, event
	})
	return p
}

func (p *preview) Draw(screen tcell.Screen) {
	// Hidden grid items retain old rectangles; clear them before mouse routing.
	for _, section := range p.sections {
		section.panel.SetRect(0, 0, 0, 0)
	}
	p.Grid.Draw(screen)
}

func (p *preview) HasFocus() bool {
	for _, section := range p.sections {
		if section.panel.HasFocus() {
			return true
		}
	}
	return p.Grid.HasFocus()
}

func (p *preview) scrollTo(index int) {
	p.SetOffset(max(0, min(index, len(p.sections)-1)), 0)
}

func (p *preview) setSections(sections []commands.Section, copyText func(commands.Section)) {
	p.Clear()
	p.sections = nil
	p.SetOffset(0, 0)
	heights := make([]int, 0, len(sections))
	for i, section := range sections {
		text := tview.NewTextView().SetDynamicColors(false).SetWrap(false).SetText(section.Content)
		execute := tview.NewButton("Execute").SetSelectedFunc(func() {})
		copyButton := tview.NewButton("Copy").SetSelectedFunc(func() { copyText(section) })
		buttons := tview.NewFlex().
			AddItem(execute, 11, 0, false).
			AddItem(nil, 1, 0, false).
			AddItem(copyButton, 8, 0, false)
		panel := tview.NewFlex().SetDirection(tview.FlexRow)
		panel.SetBorder(true).SetTitle(" " + section.Title + " ")
		warningHeight := 0
		if section.Warning != "" {
			warningHeight = 3
			panel.AddItem(tview.NewTextView().SetText(section.Warning).SetWordWrap(true), warningHeight, 0, false)
		}
		panel.AddItem(text, 0, 1, false).AddItem(buttons, 1, 0, false)
		heights = append(heights, strings.Count(section.Content, "\n")+1+3+warningHeight)
		p.AddItem(panel, i, 0, 1, 1, 0, 0, false)
		p.sections = append(p.sections, previewSection{section, panel, text, execute, copyButton})
	}
	if len(sections) == 0 {
		empty := tview.NewTextView().SetText("No saved configurations.\n\nClick New or press n to add one.\n\nThis app does not discover clusters or execute Azure commands.")
		p.AddItem(empty, 0, 0, 1, 1, 0, 0, false)
		heights = []int{0}
	}
	p.SetRows(heights...)
}

func (p *preview) focusables() []tview.Primitive {
	items := []tview.Primitive{p}
	for _, section := range p.sections {
		items = append(items, section.text, section.execute, section.copy)
	}
	return items
}
