package ui

import "github.com/gdamore/tcell/v2"

type inputScreen struct {
	tcell.Screen
	initialized bool
	pasting     bool
	pasteCR     bool
}

func (s *inputScreen) Init() error {
	if s.initialized {
		return nil
	}
	if err := s.Screen.Init(); err != nil {
		return err
	}
	s.initialized = true
	return nil
}

func (s *inputScreen) PollEvent() tcell.Event {
	for {
		event := s.Screen.PollEvent()
		switch event := event.(type) {
		case *tcell.EventPaste:
			s.pasting, s.pasteCR = event.Start(), false
		case *tcell.EventKey:
			if s.pasting {
				previousCR := s.pasteCR
				s.pasteCR = event.Key() == tcell.KeyEnter
				// tview's paste accumulator recognizes CR but drops LF key
				// events. Treat CRLF as one newline, not two.
				if event.Key() == tcell.KeyCtrlJ {
					if previousCR {
						continue
					}
					return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
				}
			}
		}
		return event
	}
}
