package gui

import (
	"context"
	"fmt"
	"strings"

	"github.com/diamondburned/gotk4/pkg/glib/v2"

	"github.com/calebstewart/vroomm/virt"
)

type SnapshotAction func(app *Application, domain *virt.Domain, snapshot string)

// A menu with a list of snapshots displayed
type SnapshotListView struct {
	Domain *virt.Domain
	Action SnapshotAction
	*FlowboxMenu
}

func NewSnapshotListView(domain *virt.Domain, actionName string, action SnapshotAction) *SnapshotListView {
	return &SnapshotListView{
		Domain:      domain,
		Action:      action,
		FlowboxMenu: NewFlowboxMenu(fmt.Sprintf("%v Snapshots", actionName)),
	}
}

func (view *SnapshotListView) Enter(app *Application) error {
	ctx, cancel := context.WithCancel(context.Background())

	view.EmptyItems()
	app.PulseProgress(ctx, "Loading Labels...")

	go func() {
		if snapshots, err := view.Domain.SnapshotListNames(0); err != nil {
			app.Logger.Error(err)
		} else {
			glib.IdleAdd(func() {
				for _, name := range snapshots {
					snapshotName := strings.Clone(name)
					view.Add(NewLabelItemWithAction("camera-photo-symbolic", name, func() {
						view.Action(app, view.Domain, snapshotName)
					}))
				}
			})
		}
		cancel()
	}()

	return view.FlowboxMenu.Enter(app)
}

func (view *SnapshotListView) Leave(app *Application) error {
	return nil
}

func (view *SnapshotListView) Close(app *Application) error {
	return nil
}
