package gui

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v3"

	"github.com/calebstewart/vroomm/virt"
)

type VirtualMachineItem struct {
	DomainName string
	Domain     *virt.Domain
	*gtk.FlowBoxChild
}

func NewVirtualMachineItem(app *Application, domain *virt.Domain) (*VirtualMachineItem, error) {
	item := &VirtualMachineItem{
		Domain:       domain,
		FlowBoxChild: gtk.NewFlowBoxChild(),
	}

	box := gtk.NewBox(gtk.OrientationHorizontal, 5)

	if domainName, err := item.Domain.GetName(); err != nil {
		return nil, err
	} else {
		item.DomainName = domainName
	}

	icon := gtk.NewImageFromIconName("computer-symbolic", int(gtk.IconSizeLargeToolbar))
	box.PackStart(icon, false, false, 0)

	label := gtk.NewLabel(item.DomainName)
	label.SetHAlign(gtk.AlignStart)
	box.PackStart(label, true, true, 0)

	item.Add(box)
	item.SetName(item.DomainName)
	item.ShowAll()

	item.ConnectActivate(func() {
		view, err := NewVirtualMachineView(app, item.Domain)
		if err != nil {
			app.AddError(err)
		} else {
			app.Push(view)
		}
	})

	return item, nil
}

func (item *VirtualMachineItem) Name() string {
	return item.DomainName
}

func (item *VirtualMachineItem) FilterText() string {
	return item.Name()
}
