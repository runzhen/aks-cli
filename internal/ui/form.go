package ui

import (
	"aks-cli/internal/config"

	"github.com/rivo/tview"
)

func (a *App) showForm() {
	a.editing = true
	draft := a.defaults
	a.form = tview.NewForm().
		AddInputField("SUBSCRIPTION", draft.Subscription, 0, nil, func(v string) { draft.Subscription = v }).
		AddInputField("LOCATION", draft.Location, 0, nil, func(v string) { draft.Location = v }).
		AddInputField("RESOURCE_GROUP", draft.ResourceGroup, 0, nil, func(v string) { draft.ResourceGroup = v }).
		AddInputField("GPU_POOL", draft.GPUPool, 0, nil, func(v string) { draft.GPUPool = v }).
		AddInputField("CLUSTER_NAME", draft.ClusterName, 0, nil, func(v string) { draft.ClusterName = v }).
		AddInputField("GPU_VM_SIZE", draft.GPUVMSize, 0, nil, func(v string) { draft.GPUVMSize = v })
	a.form.AddButton("Save", func() { a.saveDraft(draft) }).
		AddButton("Cancel", a.closeForm).
		SetCancelFunc(a.closeForm)
	a.form.SetBorder(true).SetTitle(" New local configuration ")
	a.formError = tview.NewTextView().SetDynamicColors(false).SetWordWrap(true)
	help := tview.NewTextView().SetText(" Tab/Shift-Tab: fields | Enter: next/activate | Esc: cancel | Ctrl-C: quit")
	page := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.form, 0, 1, true).
		AddItem(a.formError, 4, 0, false).
		AddItem(help, 1, 0, false)
	a.pages.AddPage("form", page, true, true)
	a.application.SetFocus(a.form)
}

func (a *App) saveDraft(draft config.Cluster) {
	next := config.Document{Version: a.doc.Version, Clusters: append([]config.Cluster{}, a.doc.Clusters...)}
	next.Clusters = append(next.Clusters, draft)
	next, err := next.Clean()
	if err != nil {
		a.formError.SetText("Cannot save: " + err.Error())
		return
	}
	if err := a.save(next); err != nil {
		a.formError.SetText("Cannot save: " + err.Error())
		return
	}
	a.doc = next
	a.refresh()
	last := len(a.doc.Clusters) - 1
	a.list.SetCurrentItem(last)
	a.selectCluster(last)
	a.closeForm()
}

func (a *App) closeForm() {
	a.editing = false
	a.pages.RemovePage("form")
	if len(a.doc.Clusters) == 0 {
		a.application.SetFocus(a.newButton)
	} else {
		a.application.SetFocus(a.list)
	}
}
