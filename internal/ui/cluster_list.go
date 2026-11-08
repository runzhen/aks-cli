package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type clusterList struct {
	*tview.List
	remove func(int)
}

func newClusterList(remove func(int)) *clusterList {
	list := &clusterList{List: tview.NewList().ShowSecondaryText(true), remove: remove}
	list.SetBorderPadding(0, 0, 0, 2)
	return list
}

func (l *clusterList) Draw(screen tcell.Screen) {
	l.List.Draw(screen)
	x, y, width, height := l.GetInnerRect()
	offset, _ := l.GetOffset()
	style := tcell.StyleDefault.Foreground(tcell.ColorRed).Background(l.GetBackgroundColor())
	for index := offset; index < l.GetItemCount(); index++ {
		row := 2 * (index - offset)
		if row >= height {
			break
		}
		screen.SetContent(x+width+1, y+row, 'x', nil, style)
	}
}

func (l *clusterList) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
		x, y, width, height := l.GetInnerRect()
		mx, my := event.Position()
		if action == tview.MouseLeftClick && mx == x+width+1 && my >= y && my < y+height && (my-y)%2 == 0 {
			offset, _ := l.GetOffset()
			index := offset + (my-y)/2
			if index < l.GetItemCount() {
				setFocus(l)
				l.remove(index)
				return true, nil
			}
		}
		return l.List.MouseHandler()(action, event, func(p tview.Primitive) {
			if p == l.List {
				p = l
			}
			setFocus(p)
		})
	}
}
