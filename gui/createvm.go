package gui

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"libvirt.org/go/libvirtxml"

	"github.com/calebstewart/vroomm/virt"
)

// Create a new CreateVmFlow which will walk the user through creating
// a new Virtual machine and place the VM in the given folder and with
// the given labels.
func NewCreateVmFlow(app *Application, folder string, labels ...string) *Prompt {

	domain := &libvirtxml.Domain{}
	metadata := &virt.VmmDomainMetadata{
		Path:   folder,
		Labels: labels,
	}

	return NewPrompt(
		app,
		"Create VM", "VM Name>",
		false,
		func(app *Application, input string) {
			finishVmName(app, domain, metadata, input)
		},
	)
}

func finishVmName(app *Application, domain *libvirtxml.Domain, metadata *virt.VmmDomainMetadata, input string) {
	ctx, cancel := context.WithCancel(context.Background())
	app.PulseProgress(ctx, "Validating VM name...")

	go func() {
		// Ensure the progress bar stops
		defer cancel()

		conn := app.Virt()

		// Retrieve a list of existing domains
		domains, err := conn.EnumerateAllDomains()
		if err != nil {
			app.Logger.Error(err.Error())
			return
		}

		for _, domain := range domains {
			if name, err := domain.GetName(); err != nil {
				app.Logger.Error(err.Error())
			} else if name == input {
				app.Logger.Errorf("VM '%v' already exists", input)
				return
			}
		}

		// Update the domain name
		domain.Name = input
		glib.IdleAdd(func() {
			app.Push(newCreateVmMemoryPrompt(app, domain, metadata))
		})
	}()
}

func newCreateVmMemoryPrompt(app *Application, domain *libvirtxml.Domain, metadata *virt.VmmDomainMetadata) *Prompt {
	return NewPrompt(
		app,
		"Memory Size",
		"Memory (MB)>",
		false,
		func(app *Application, input string) {

		},
	)
}
