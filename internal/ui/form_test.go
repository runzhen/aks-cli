package ui

import (
	"strings"
	"testing"
	"time"

	"aks-cli/internal/config"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestFormSelectAllKeyboard(t *testing.T) {
	for _, event := range []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyCtrlA, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModCtrl),
		tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModMeta),
		tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModAlt),
	} {
		t.Run(event.Name(), func(t *testing.T) {
			a, _ := simulate(t, config.Empty(), func(config.Document) error { return nil })
			press(a, tcell.KeyRune, 'n')
			for i := 0; i < a.form.GetFormItemCount(); i++ {
				field := a.form.GetFormItem(i).(*tview.InputField)
				for _, original := range []string{"", "two words-and_punctuation", strings.Repeat("long-value-", 20), "équipe-日本"} {
					field.SetText(original)
					a.application.SetFocus(field)
					a.application.ForceDraw()
					mapped := a.handleKey(event)
					if mapped == nil {
						t.Fatal("select-all key was discarded")
					}
					a.pages.InputHandler()(mapped, func(p tview.Primitive) { a.application.SetFocus(p) })
					if field.GetText() != original {
						t.Fatal("select-all modified text before replacement")
					}
					press(a, tcell.KeyRune, 'x')
					if field.GetText() != "x" {
						t.Fatalf("field %d: selecting %q then typing produced %q", i, original, field.GetText())
					}
				}
			}
		})
	}
}

func TestFormDoubleClickSelectAllAndSave(t *testing.T) {
	var saved config.Document
	a, _ := simulate(t, config.Empty(), func(doc config.Document) error { saved = doc; return nil })
	press(a, tcell.KeyRune, 'n')
	for i := 0; i < a.form.GetFormItemCount(); i++ {
		field := a.form.GetFormItem(i).(*tview.InputField)
		original := "two words-and_punctuation"
		field.SetText(original)
		a.application.ForceDraw()
		x, y, width, _ := field.GetInnerRect()
		event, _ := a.handleMouse(tcell.NewEventMouse(x+width-1, y, tcell.ButtonNone, tcell.ModNone), tview.MouseLeftDoubleClick)
		if event != nil || !field.HasFocus() || field.GetText() != original {
			t.Fatal("double-click did not focus and select the field without editing it")
		}
		press(a, tcell.KeyBackspace2, 0)
		if field.GetText() != "" {
			t.Fatalf("field %d: selection did not include all text", i)
		}
		for _, r := range []string{"Development", "eastus2", "custom-rg", "gpu", "custom", "Standard_NC6s_v3"}[i] {
			press(a, tcell.KeyRune, r)
		}
	}
	a.form.SetFocus(6)
	a.application.SetFocus(a.form)
	press(a, tcell.KeyEnter, 0)
	if a.editing || len(saved.Clusters) != 1 || saved.Clusters[0].Subscription != "Development" ||
		saved.Clusters[0].ClusterName != "custom" || saved.Clusters[0].GPUVMSize != "Standard_NC6s_v3" {
		t.Fatalf("select-all edits did not reach saved data: %+v", saved)
	}
}

func TestSelectAllDoesNotInterceptTerminal(t *testing.T) {
	a, _ := simulate(t, config.Empty(), func(config.Document) error { return nil })
	a.showTerminal()
	event := tcell.NewEventKey(tcell.KeyCtrlA, 0, tcell.ModNone)
	if a.handleKey(event) != event {
		t.Fatal("Ctrl-A was remapped outside New form")
	}
}

func TestDoubleClickSelectAllInEventLoop(t *testing.T) {
	a, _ := simulate(t, config.Empty(), func(config.Document) error { return nil })
	press(a, tcell.KeyRune, 'n')
	field := a.form.GetFormItem(2).(*tview.InputField)
	field.SetText("two-words value")
	a.application.ForceDraw()
	x, y, width, _ := field.GetInnerRect()
	results := make(chan string, 1)
	a.application.SetAfterDrawFunc(func(tcell.Screen) {
		if field.GetText() == "z" {
			select {
			case results <- field.GetText():
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
	for i := 0; i < 2; i++ {
		a.application.QueueEvent(tcell.NewEventMouse(x+width-1, y, tcell.Button1, tcell.ModNone))
		a.application.QueueEvent(tcell.NewEventMouse(x+width-1, y, tcell.ButtonNone, tcell.ModNone))
	}
	a.application.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModNone))
	select {
	case <-results:
	case <-time.After(3 * time.Second):
		t.Fatal("native mouse double-click did not select all before typing")
	}
}
