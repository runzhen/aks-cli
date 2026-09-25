package ui

import (
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
	application *tview.Application
	pages       *tview.Pages
	list        *clusterList
	preview     *preview
	status      *tview.TextView
	copyText    func(string) error
	newButton   *tview.Button
	form        *tview.Form
	formError   *tview.TextView
	doc         config.Document
	save        func(config.Document) error
	defaults    config.Cluster
	editing     bool
	compact     bool
}

func New(doc config.Document, save func(config.Document) error, defaults config.Cluster) *App {
	a := &App{
		application: tview.NewApplication(),
		pages:       tview.NewPages(),
		doc:         doc,
		save:        save,
		defaults:    defaults,
		copyText:    clipboard.Write,
	}
	a.preview = newPreview(func(p tview.Primitive) { a.application.SetFocus(p) })
	a.status = tview.NewTextView().SetDynamicColors(false).SetWordWrap(true)
	a.list = newClusterList(a.removeCluster)
	a.list.SetBorder(true).SetTitle(" Saved configurations ")
	a.list.SetChangedFunc(func(index int, _, _ string, _ rune) { a.selectCluster(index) })
	a.newButton = tview.NewButton("New").SetSelectedFunc(a.showForm)
	a.newButton.SetBorder(true)

	left := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.newButton, 3, 0, false).
		AddItem(a.list, 0, 1, true)
	right := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.preview, 0, 1, false).AddItem(a.status, 2, 0, false)
	body := tview.NewFlex().AddItem(left, 0, 1, true).AddItem(right, 0, 2, false)
	header := tview.NewTextView().SetText(" aks-cli | LOCAL CONFIGURATIONS | PREVIEW ONLY")
	footer := tview.NewTextView().SetText(" n: New | Tab: focus | Arrows/PgUp/PgDn: navigate/scroll | Ctrl-C: quit")
	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 1, 0, false).
		AddItem(body, 0, 1, true).
		AddItem(footer, 1, 0, false)
	a.pages.AddPage("main", root, true, true)
	a.application.SetRoot(a.pages, true).EnableMouse(true)
	a.application.SetInputCapture(a.handleKey)
	a.application.SetMouseCapture(func(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
		if a.compact {
			return nil, action
		}
		return event, action
	})

	small := tview.NewTextView().SetTextAlign(tview.AlignCenter).
		SetText(fmt.Sprintf("Terminal too small.\nResize to at least %dx%d.\nCtrl-C: quit", minWidth, minHeight))
	a.application.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		width, height := screen.Size()
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
	return a.application.Run()
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
		a.preview.setSections(nil, a.copySection)
		return
	}
	if index < 0 || index >= len(a.doc.Clusters) {
		return
	}
	a.preview.setSections(commands.Sections(a.doc.Clusters[index]), a.copySection)
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
	if event.Key() == tcell.KeyCtrlC {
		a.application.Stop()
		return nil
	}
	if a.compact {
		return nil
	}
	if a.editing {
		if event.Key() == tcell.KeyEscape {
			a.closeForm()
			return nil
		}
		return event
	}
	if event.Key() == tcell.KeyRune && event.Rune() == 'n' {
		a.showForm()
		return nil
	}
	if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyBacktab {
		focus := append([]tview.Primitive{a.newButton, a.list}, a.preview.focusables()...)
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
