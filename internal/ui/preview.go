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
	text    *tview.TextArea
	execute *tview.Button
	copy    *tview.Button
}

type preview struct {
	*tview.Grid
	sections []previewSection
}

func newPreview(focus func(tview.Primitive)) *preview {
	p := &preview{Grid: tview.NewGrid().SetColumns(0).SetGap(1, 0)}
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
			return tview.MouseConsumed, nil
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

func (p *preview) setSections(sections []commands.Section, drafts map[string]string, copyText, executeSection func(commands.Section)) {
	p.Clear()
	p.sections = nil
	p.SetOffset(0, 0)
	heights := make([]int, 0, len(sections))
	for i, section := range sections {
		content := section.Content
		if draft, ok := drafts[section.Title]; ok {
			content = draft
		}
		text := tview.NewTextArea().SetWrap(false).SetText(content, false)
		execute := tview.NewButton("Execute").SetSelectedFunc(func() { executeSection(section.WithContent(text.GetText())) })
		copyButton := tview.NewButton("Copy").SetSelectedFunc(func() { copyText(section.WithContent(text.GetText())) })
		buttons := tview.NewFlex().
			AddItem(execute, 11, 0, false).
			AddItem(nil, 1, 0, false).
			AddItem(copyButton, 8, 0, false)
		panel := tview.NewFlex().SetDirection(tview.FlexRow)
		panel.SetBorder(true)
		updateDraft := func() {
			title := section.Title
			if current := text.GetText(); current != section.Content {
				drafts[section.Title] = current
				title += " *"
			} else {
				delete(drafts, section.Title)
			}
			panel.SetTitle(" " + title + " ")
		}
		text.SetChangedFunc(updateDraft)
		updateDraft()
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
		empty := tview.NewTextView().SetText("No saved configurations.\n\nClick New or press n to add one.\n\nSaved configurations are not discovered Azure resources.")
		p.AddItem(empty, 0, 0, 1, 1, 0, 0, false)
		heights = []int{0}
	}
	p.SetRows(heights...)
}

func (p *preview) editorFocused() bool {
	for _, section := range p.sections {
		if section.text.HasFocus() {
			return true
		}
	}
	return false
}

func (p *preview) focusables() []tview.Primitive {
	items := []tview.Primitive{p}
	for _, section := range p.sections {
		items = append(items, section.text, section.execute, section.copy)
	}
	return items
}
