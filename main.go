package main

import (
	"flag"
	"fmt"
	"os"

	"aks-cli/internal/config"
	"aks-cli/internal/ui"
)

func run() error {
	path, err := config.DefaultPath()
	if err != nil {
		return err
	}
	store := config.Store{Path: path}
	doc, err := store.Load()
	if err != nil {
		return err
	}
	app := ui.New(doc, store.Save, config.Defaults(os.Getenv("USER")))
	return app.Run()
}

func main() {
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: aks-cli\n\nManage local AKS configurations; preview, copy, and execute commands in a terminal pane.\nExecute runs real Azure operations; delete commands require confirmation.\nF2: terminal | Esc: panes | Ctrl-C: interrupt/quit | Ctrl-Q: quit\nConfig: ~/.config/aks-cli.json")
	}
	flag.Parse()
	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "aks-cli: %s\n", err)
		os.Exit(1)
	}
}
