package ui

import (
	"strings"
	"testing"

	"aks-cli/internal/config"
)

func TestCommandsHeadingWithoutOuterFrame(t *testing.T) {
	a, screen := simulate(t, config.Document{Version: config.Version, Clusters: []config.Cluster{fixture("alice", "dev")}},
		func(config.Document) error { return nil })
	a.application.SetFocus(a.preview)
	a.application.ForceDraw()
	x, y, width, height := a.preview.GetRect()
	innerX, innerY, innerWidth, innerHeight := a.preview.GetInnerRect()
	if x != innerX || y != innerY || width != innerWidth || height != innerHeight {
		t.Fatal("commands container still reserves space for an outer border")
	}
	if !strings.Contains(screenText(screen), "Commands - Execute runs in terminal") {
		t.Fatal("command heading is missing")
	}
	_, leftY, _, _ := a.newButton.GetRect()
	if y != leftY+1 {
		t.Fatal("heading must occupy one row above the command sections")
	}
	for col := x; col < x+width; col++ {
		char, _, _, _ := screen.GetContent(col, y-1)
		if strings.ContainsRune("─═│║┌┐└┘╔╗╚╝", char) {
			t.Fatal("command heading has a visible border line")
		}
	}
	section := a.preview.sections[0].panel
	sx, sy, _, _ := section.GetRect()
	ix, iy, _, _ := section.GetInnerRect()
	if ix != sx+1 || iy != sy+1 {
		t.Fatal("individual section border was removed")
	}
}
