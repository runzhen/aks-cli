package ui

import (
	"testing"
	"time"

	"aks-cli/internal/config"

	"github.com/gdamore/tcell/v2"
)

func TestInputScreenCRLFAndUnmodifiedTerminalKeys(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	input := &inputScreen{Screen: screen}
	if err := input.Init(); err != nil {
		t.Fatal(err)
	}
	defer input.Fini()
	events := []tcell.Event{
		tcell.NewEventPaste(true),
		tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyCtrlJ, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone),
		tcell.NewEventPaste(false),
		tcell.NewEventKey(tcell.KeyCtrlJ, 0, tcell.ModNone),
	}
	for _, event := range events {
		if err := screen.PostEvent(event); err != nil {
			t.Fatal(err)
		}
	}
	input.PollEvent()
	if event := input.PollEvent().(*tcell.EventKey); event.Key() != tcell.KeyEnter {
		t.Fatal("paste lost CR")
	}
	if event := input.PollEvent().(*tcell.EventKey); event.Rune() != 'x' {
		t.Fatal("CRLF paste inserted a duplicate newline")
	}
	input.PollEvent()
	if event := input.PollEvent().(*tcell.EventKey); event.Key() != tcell.KeyCtrlJ {
		t.Fatal("normal terminal key was remapped outside paste")
	}
}

func TestNativePastePreservesLineFeeds(t *testing.T) {
	a, screen := simulate(t, config.Document{Version: config.Version, Clusters: []config.Cluster{fixture("alice", "dev")}},
		func(config.Document) error { return nil })
	editor := a.preview.sections[0].text
	editor.SetText("", false)
	a.application.SetFocus(editor)
	want := "printf first\nprintf second"
	result := make(chan struct{}, 1)
	a.application.SetAfterDrawFunc(func(tcell.Screen) {
		if editor.GetText() == want {
			select {
			case result <- struct{}{}:
			default:
			}
		}
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
			t.Error("event loop did not stop")
		}
	}()
	events := []tcell.Event{tcell.NewEventPaste(true)}
	for _, r := range want {
		if r == '\n' {
			events = append(events, tcell.NewEventKey(tcell.KeyCtrlJ, 0, tcell.ModNone))
		} else {
			events = append(events, tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
		}
	}
	events = append(events, tcell.NewEventPaste(false))
	for _, event := range events {
		screen.PostEventWait(event)
	}
	select {
	case <-result:
	case <-time.After(3 * time.Second):
		t.Fatal("native bracketed paste lost line feeds")
	}
}
