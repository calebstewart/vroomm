package gui

import (
	"github.com/calebstewart/vroomm/virt"
)

func NewVirtualMachineItem(app *Application, domain *virt.Domain) (*LabelItem, error) {
	if domainName, err := domain.GetName(); err != nil {
		return nil, err
	} else {
		return NewLabelItemWithAction(
			"computer-symbolic",
			domainName,
			func() {
				if view, err := NewVirtualMachineView(app, domain); err != nil {
					app.AddError(err)
				} else {
					app.Push(view)
				}
			},
		), nil
	}
}
