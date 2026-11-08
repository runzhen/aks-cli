package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aks-cli/internal/config"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func clickRemove(t *testing.T, a *App, index int) {
	t.Helper()
	x, y, width, height := a.list.GetInnerRect()
	offset, _ := a.list.GetOffset()
	row := 2 * (index - offset)
	if row < 0 || row >= height {
		t.Fatalf("item %d is not visible (offset %d, height %d)", index, offset, height)
	}
	consumed, _ := a.pages.MouseHandler()(tview.MouseLeftClick,
		tcell.NewEventMouse(x+width+1, y+row, tcell.Button1, tcell.ModNone),
		func(p tview.Primitive) { a.application.SetFocus(p) })
	if !consumed {
		t.Fatal("remove click did not request a redraw")
	}
	a.application.ForceDraw()
}

func TestRemoveConfigurationPersistsAndRefreshes(t *testing.T) {
	store := config.Store{Path: filepath.Join(t.TempDir(), "aks-cli.json")}
	first, second, third := fixture("shared", "dev"), fixture("shared", "prod"), fixture("third", "prod")
	doc := config.Document{Version: config.Version, Clusters: []config.Cluster{first, second, third}}
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}
	a, screen := simulate(t, doc, store.Save)
	x, y, width, _ := a.list.GetInnerRect()
	for i := range doc.Clusters {
		mark, _, _, _ := screen.GetContent(x+width+1, y+2*i)
		if mark != 'x' {
			t.Fatalf("item %d missing remove control", i)
		}
	}

	press(a, tcell.KeyDown, 0)
	clickRemove(t, a, 0)
	want := config.Document{Version: config.Version, Clusters: []config.Cluster{second, third}}
	reloaded, err := store.Load()
	if err != nil || !reflect.DeepEqual(reloaded, want) || !reflect.DeepEqual(a.doc, want) {
		t.Fatalf("wrong config removed: %+v, %v", reloaded, err)
	}
	if a.list.GetCurrentItem() != 0 || !strings.Contains(previewText(a), "SUBSCRIPTION=prod") {
		t.Fatal("deleting preceding item changed the selected cluster")
	}
	clickRemove(t, a, 0)
	if a.list.GetItemCount() != 1 || !strings.Contains(previewText(a), "third-gpu") {
		t.Fatal("deleting selected item did not select its successor")
	}
	reloaded, err = store.Load()
	if err != nil || len(reloaded.Clusters) != 1 || reloaded.Clusters[0] != third {
		t.Fatalf("selected item removal not persisted: %+v, %v", reloaded, err)
	}
	restarted, _ := simulate(t, reloaded, store.Save)
	if restarted.list.GetItemCount() != 1 || !strings.Contains(previewText(restarted), "third-gpu") {
		t.Fatal("removed configuration returned on restart")
	}
	clickRemove(t, a, 0)
	reloaded, err = store.Load()
	if err != nil || !reflect.DeepEqual(reloaded, config.Empty()) {
		t.Fatalf("last removal did not save an empty array: %+v, %v", reloaded, err)
	}
	if a.list.GetItemCount() != 0 || len(a.preview.sections) != 0 ||
		a.application.GetFocus() != a.newButton || !strings.Contains(screenText(screen), "No saved configurations") {
		t.Fatal("last removal did not restore empty state and New focus")
	}
	press(a, tcell.KeyRune, 'n')
	a.saveDraft(first)
	if a.editing || len(a.doc.Clusters) != 1 {
		t.Fatal("could not add a configuration after deleting all entries")
	}
}

func TestRemoveConfigurationFailureKeepsState(t *testing.T) {
	store := config.Store{Path: filepath.Join(t.TempDir(), "aks-cli.json")}
	doc := config.Document{Version: config.Version, Clusters: []config.Cluster{fixture("first", "dev"), fixture("second", "prod")}}
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := simulate(t, doc, func(config.Document) error { return errors.New("disk is read-only") })
	press(a, tcell.KeyDown, 0)
	previewBefore := previewText(a)
	clickRemove(t, a, 0)
	after, err := os.ReadFile(store.Path)
	if err != nil || string(before) != string(after) || !reflect.DeepEqual(a.doc, doc) ||
		a.list.GetItemCount() != 2 || a.list.GetCurrentItem() != 1 || previewText(a) != previewBefore {
		t.Fatal("failed removal changed persisted or visible state")
	}
	if !strings.Contains(a.status.GetText(false), "Cannot remove configuration: disk is read-only") {
		t.Fatal("removal error was not displayed")
	}
	a.save = store.Save
	clickRemove(t, a, 1)
	if len(a.doc.Clusters) != 1 || a.list.GetCurrentItem() != 0 || a.doc.Clusters[0] != doc.Clusters[0] {
		t.Fatal("retry deleting last selected row did not select preceding survivor")
	}
}

func TestRemoveControlTracksScrolledResizedList(t *testing.T) {
	doc := config.Empty()
	for i := 0; i < 30; i++ {
		doc.Clusters = append(doc.Clusters, fixture(fmt.Sprintf("cluster-%02d", i), "dev"))
	}
	var saved config.Document
	a, screen := simulate(t, doc, func(next config.Document) error { saved = next; return nil })
	press(a, tcell.KeyEnd, 0)
	screen.SetSize(minWidth, minHeight)
	a.application.ForceDraw()
	offset, _ := a.list.GetOffset()
	if offset == 0 {
		t.Fatal("test list did not scroll")
	}
	x, y, width, height := a.list.GetInnerRect()
	for i := offset; i < len(doc.Clusters) && 2*(i-offset) < height; i++ {
		mark, _, _, _ := screen.GetContent(x+width+1, y+2*(i-offset))
		if mark != 'x' {
			t.Fatalf("scrolled item %d missing remove control", i)
		}
	}
	clickRemove(t, a, offset)
	want := append([]config.Cluster{}, doc.Clusters[:offset]...)
	want = append(want, doc.Clusters[offset+1:]...)
	if !reflect.DeepEqual(saved.Clusters, want) || !reflect.DeepEqual(a.doc.Clusters, want) {
		t.Fatal("scrolled click removed the wrong item")
	}
	if a.doc.Clusters[a.list.GetCurrentItem()] != doc.Clusters[len(doc.Clusters)-1] {
		t.Fatal("deleting a nonselected visible item changed selection")
	}
}

func TestRemoveControlIgnoresOtherClicks(t *testing.T) {
	saves := 0
	a, _ := simulate(t, config.Document{Version: config.Version, Clusters: []config.Cluster{fixture("alice", "dev"), fixture("bob", "prod")}},
		func(config.Document) error { saves++; return nil })
	x, y, width, _ := a.list.GetInnerRect()
	for _, pos := range [][2]int{
		{x, y + 2},             // Regular cluster name.
		{x + width + 1, y + 1}, // Secondary metadata, not the x.
		{x + width + 2, y},     // Border.
		{x + width + 1, y + 8}, // Empty row.
		{x + width + 1, y - 1}, // Title border.
	} {
		click(a, pos[0], pos[1])
	}
	if saves != 0 || len(a.doc.Clusters) != 2 {
		t.Fatal("a click outside an x removed an item")
	}
	click(a, x, y+2)
	if a.application.GetFocus() != a.list || a.list.GetCurrentItem() != 1 {
		t.Fatal("regular list clicks no longer select and focus")
	}
	press(a, tcell.KeyTab, 0)
	if a.application.GetFocus() != a.preview {
		t.Fatal("Tab after list click did not focus right pane")
	}
}
