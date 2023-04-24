package gui

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v3"
	"libvirt.org/go/libvirt"
	"libvirt.org/go/libvirtxml"

	"github.com/calebstewart/vroomm/virt"
)

type VirtualMachineView struct {
	Domain       *virt.Domain        // The domain we are interacting with
	DomainName   string              // Name of the domain
	FlowBoxMenu  *FlowboxMenu        // Menu for interactions with the VM
	PropertyView *gtk.ScrolledWindow // View for VM status
	Cancel       func()              // A method for cancelling the background update task
	*gtk.Box                         // Container for above widgets
}

func NewVirtualMachineView(app *Application, domain *virt.Domain) (*VirtualMachineView, error) {
	name, err := domain.GetName()
	if err != nil {
		return nil, err
	}

	view := &VirtualMachineView{
		Domain:       domain,
		DomainName:   name,
		FlowBoxMenu:  NewFlowboxMenu(name),
		PropertyView: gtk.NewScrolledWindow(nil, nil),
		Box:          gtk.NewBox(gtk.OrientationHorizontal, 2),
	}

	view.Box.PackStart(view.FlowBoxMenu, true, true, 0)
	view.Box.PackStart(gtk.NewSeparator(gtk.OrientationVertical), false, false, 0)
	view.Box.PackStart(view.PropertyView, true, true, 0)
	view.SetName(view.DomainName)
	view.ShowAll()

	return view, nil
}

func (view *VirtualMachineView) Name() string {
	return view.DomainName
}

func (view *VirtualMachineView) updateView(app *Application) error {

	domXml := libvirtxml.Domain{}
	if xmlDoc, err := view.Domain.GetXMLDesc(libvirt.DOMAIN_XML_SECURE); err != nil {
		return err
	} else if err := xml.Unmarshal([]byte(xmlDoc), &domXml); err != nil {
		return err
	}

	state, _, err := view.Domain.GetState()
	if err != nil {
		return err
	}

	selectedIndex := -1
	if selected := view.FlowBoxMenu.FlowBox.SelectedChildren(); len(selected) > 0 {
		selectedIndex = selected[0].Index()
	}
	view.FlowBoxMenu.EmptyItems()

	prettyState := ""

	switch state {
	case libvirt.DOMAIN_BLOCKED:
		fallthrough
	case libvirt.DOMAIN_PMSUSPENDED:
		fallthrough
	case libvirt.DOMAIN_RUNNING:
		prettyState = "Running"
		view.CreateItem(app, "computer-symbolic", "Open Viewer", app.ActivationWithPulse(view.openViewer))
		view.CreateItem(app, "system-search-symbolic", "Open Looking Glass", app.ActivationWithPulse(view.openLookingGlass))
		view.CreateItem(app, "system-shutdown-symbolic", "Shutdown", app.ActivationWithPulse(view.shutDown))
		view.CreateItem(app, "face-shutmouth-symbolic", "Force Off", app.ActivationWithPulse(view.forceOff))
		view.CreateItem(app, "media-floppy-symbolic", "Save State", app.ActivationWithPulse(view.saveState))
	case libvirt.DOMAIN_CRASHED:
		fallthrough
	case libvirt.DOMAIN_SHUTOFF:
		fallthrough
	case libvirt.DOMAIN_SHUTDOWN:
		prettyState = "Off"
		view.CreateItem(app, "media-playback-start-symbolic", "Start", app.ActivationWithPulse(view.start))
	}

	view.CreateItem(app, "edit-copy-symbolic", "Linked Clone", app.Activation(view.linkedClone))
	view.CreateItem(app, "edit-copy-symbolic", "Full Clone", app.Activation(view.fullClone))
	view.CreateItem(app, "camera-photo-symbolic", "Take Snapshot", app.Activation(view.snapshot))
	view.CreateItem(app, "document-open-recent-symbolic", "Restore Snapshot", app.Activation(view.restoreSnapshot))
	view.CreateItem(app, "user-trash-symbolic", "Delete Snapshot", app.Activation(view.deleteSnapshot))
	view.CreateItem(app, "folder-symbolic", "Move To...", app.ActivationWithPulse(view.move))
	view.CreateItem(app, "user-bookmarks-symbolic", "Add Label", app.ActivationWithPulse(view.addLabel))
	view.CreateItem(app, "user-bookmarks-symbolic", "Remove Label", app.ActivationWithPulse(view.addLabel))
	view.CreateItem(app, "document-edit-symbolic", "Edit XML", app.ActivationWithPulse(view.editXML))

	if selectedIndex > -1 {
		child := view.FlowBoxMenu.FlowBox.ChildAtIndex(selectedIndex)
		view.FlowBoxMenu.FlowBox.SelectChild(child)
		child.GrabFocus()
	}

	grid := gtk.NewGrid()
	grid.SetHExpand(true)
	grid.SetVExpand(true)
	addPropertyRow(grid, 0, "Name:", view.DomainName)
	addPropertyRow(grid, 1, "State:", prettyState)
	addPropertyRow(grid, 2, "CPU:", "%v", domXml.VCPU.Value)
	addPropertyRow(grid, 3, "Memory:", "%v-%v", domXml.Memory.Value, domXml.Memory.Unit)
	grid.ShowAll()

	if view.PropertyView.Child() != nil {
		view.PropertyView.Remove(view.PropertyView.Child())
	}
	view.PropertyView.Add(grid)

	return nil
}

func (view *VirtualMachineView) Enter(app *Application) error {
	if err := view.updateView(app); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	view.Cancel = cancel

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
				glib.IdleAdd(func() {
					err := view.updateView(app)
					if err != nil {
						app.AddError(err)
					}
				})
			}
		}
	}()

	return view.FlowBoxMenu.Enter(app)
}

func (view *VirtualMachineView) CreateItem(app *Application, icon string, text string, action func()) *LabelItem {
	item := NewLabelItem(icon, text)
	item.FlowBoxChild.ConnectActivate(action)
	view.FlowBoxMenu.Add(item)
	return item
}

func (view *VirtualMachineView) Leave(app *Application) error {
	view.Cancel()
	return nil
}

func (view *VirtualMachineView) Close(app *Application) error {
	view.Cancel()
	return nil
}

func (view *VirtualMachineView) Widget() *gtk.Widget {
	return view.FlowBoxMenu.Widget()
}

func (view *VirtualMachineView) Activate(app *Application) {
	view.FlowBoxMenu.Activate(app)
}

func (view *VirtualMachineView) InvalidateFilter() {
	view.FlowBoxMenu.InvalidateFilter()
}

func (view *VirtualMachineView) openViewer(app *Application) (string, error) {
	uuid, err := view.Domain.GetUUIDString()
	if err != nil {
		return "", err
	}

	// Setup the virt-viewer command
	command := exec.Command(
		"virt-viewer",
		"--connect",
		app.Config.ConnectionString,
		"--auto-resize=always",
		"--cursor=auto",
		"--wait",
		"--reconnect",
		"--shared",
		"--uuid",
		uuid,
	)

	// Ensure we have no stdio
	command.Stderr = nil
	command.Stdout = nil
	command.Stdin = nil

	if err := command.Start(); err != nil {
		return "", err
	}

	// We should exit since we just spawned an interactive application
	app.Quit()

	return "Started virt-viewer", nil
}

func (view *VirtualMachineView) openLookingGlass(app *Application) (string, error) {
	uuid, err := view.Domain.GetUUIDString()
	if err != nil {
		return "", err
	}

	// Setup the virt-viewer command
	command := exec.Command(
		"looking-glass-client",
		uuid,
	)

	// Ensure we have no stdio
	command.Stderr = nil
	command.Stdout = nil
	command.Stdin = nil

	if err := command.Start(); err != nil {
		return "", err
	}

	// We should exit since we just spawned an interactive application
	app.Quit()

	return "Started Looking Glass Client", nil
}

func (view *VirtualMachineView) shutDown(app *Application) (string, error) {
	return "Virtual Machine Powered Off", view.Domain.Shutdown()
}

func (view *VirtualMachineView) forceOff(app *Application) (string, error) {
	return "Virtual Machine Forced Off", view.Domain.Destroy()
}

func (view *VirtualMachineView) saveState(app *Application) (string, error) {
	return "Virtual Machine State Saved", view.Domain.ManagedSave(0)
}

func (view *VirtualMachineView) start(app *Application) (string, error) {
	return "Virtual Machine Started", view.Domain.Create()
}

func (view *VirtualMachineView) linkedClone(app *Application) (string, error) {
	prompt := NewPrompt("Linked Clone", "Clone Name>", func(app *Application, name string) {
		conn := app.Virt()
		if _, err := conn.LookupDomainByName(name); err == nil {
			app.AddError(fmt.Errorf("Virtual Machine '%v' already exists", name))
			return
		}

		app.ActivationWithPulse(func(app *Application) (string, error) {
			domain, err := view.Domain.Clone(app.Virt(), name, true)
			if err != nil {
				return "", err
			}

			domainView, err := NewVirtualMachineView(app, domain)
			if err == nil {
				glib.IdleAdd(func() {
					app.ReplaceTop(domainView)
				})
			}

			return fmt.Sprintf("Virtual Machine '%v' Cloned to '%v'", view.DomainName, name), nil
		})()
	})

	app.Push(prompt)

	return "", nil
}

func (view *VirtualMachineView) fullClone(app *Application) (string, error) {
	prompt := NewPrompt("Full Clone", "Clone Name>", func(app *Application, name string) {
		conn := app.Virt()
		if _, err := conn.LookupDomainByName(name); err == nil {
			app.AddError(fmt.Errorf("Virtual Machine '%v' already exists", name))
			return
		}

		app.ActivationWithPulse(func(app *Application) (string, error) {
			domain, err := view.Domain.Clone(app.Virt(), name, true)
			if err != nil {
				return "", err
			}

			domainView, err := NewVirtualMachineView(app, domain)
			if err == nil {
				glib.IdleAdd(func() {
					app.ReplaceTop(domainView)
				})
			}

			return fmt.Sprintf("Virtual Machine '%v' Cloned to '%v'", view.DomainName, name), nil
		})()
	})

	app.Push(prompt)

	return "", nil
}

// Simple snapshot interface. It does not provide a way to specify the snapshot description
// or customize the devices or memory snapshotting. It will naively select memory snapshotting
// if the domain is current running, and snapshot all disks that are not read-only. The snapshot
// will have a state of running if the VM is currently running. A description can be added
// separately by editing the snapshot XML
func (view *VirtualMachineView) snapshot(app *Application) (string, error) {
	app.Push(
		NewPrompt(
			"Snapshot",
			"Snapshot Name>",
			view.doSnapshot,
		),
	)
	return "", nil
}

// This is the prompt action for the snapshot name prompt. It's a complex action, so I split
// it out from a clojure into a real function.
func (view *VirtualMachineView) doSnapshot(app *Application, name string) {
	app.Pop()
	ctx, cancel := context.WithCancel(context.Background())
	app.PulseProgress(ctx, "Snapshotting Virtual Machine...")

	go func() {
		defer cancel()

		domainDescription := libvirtxml.Domain{}
		if xmlDescr, err := view.Domain.GetXMLDesc(libvirt.DOMAIN_XML_SECURE); err != nil {
			glib.IdleAdd(func() { app.AddError(err) })
			return
		} else if err := xml.Unmarshal([]byte(xmlDescr), &domainDescription); err != nil {
			glib.IdleAdd(func() { app.AddError(err) })
			return
		}

		domainSnapshot := libvirtxml.DomainSnapshot{
			Name:        name,
			Description: "",
			Parent:      nil,
			Disks: &libvirtxml.DomainSnapshotDisks{
				Disks: []libvirtxml.DomainSnapshotDisk{},
			},
			Domain: &domainDescription,
		}

		for _, disk := range domainDescription.Devices.Disks {
			snapshot := ""
			if disk.ReadOnly != nil {
				snapshot = "no"
			}

			domainSnapshot.Disks.Disks = append(domainSnapshot.Disks.Disks, libvirtxml.DomainSnapshotDisk{
				Name:     disk.Target.Dev,
				Snapshot: snapshot,
			})
		}

		if snapshotXml, err := xml.Marshal(&domainSnapshot); err != nil {
			glib.IdleAdd(func() { app.AddError(err) })
		} else if _, err := view.Domain.CreateSnapshotXML(string(snapshotXml), 0); err != nil {
			glib.IdleAdd(func() { app.AddError(err) })
		} else {
			glib.IdleAdd(func() {
				app.StatusBar.Push(
					app.StatusBar.ContextID("snapshot"),
					fmt.Sprintf("Created Snapshot '%v' of '%v'", name, domainDescription.Name),
				)
			})
		}
	}()
}

func (view *VirtualMachineView) deleteSnapshot(app *Application) (string, error) {
	app.Push(
		NewSnapshotListView(view.Domain, "Delete", func(app *Application, domain *virt.Domain, snapshotName string) {

			// Leave the snapshot selection view
			app.Pop()

			snapshot, err := domain.SnapshotLookupByName(snapshotName, 0)
			if err != nil {
				app.AddError(err)
				return
			}

			app.ActivationWithPulse(func(app *Application) (string, error) {
				if domainName, err := domain.GetName(); err != nil {
					return "", err
				} else if err := snapshot.Delete(0); err != nil {
					return "", err
				} else {
					return fmt.Sprintf(
						"Deleted Snapshot '%v' from Virtual Machine '%v'",
						snapshotName,
						domainName,
					), err
				}
			})()
		}),
	)

	return "", nil
}

func (view *VirtualMachineView) restoreSnapshot(app *Application) (string, error) {

	app.Push(
		NewSnapshotListView(view.Domain, "Revert", func(app *Application, domain *virt.Domain, snapshotName string) {
			app.Pop()

			snapshot, err := domain.SnapshotLookupByName(snapshotName, 0)
			if err != nil {
				app.AddError(err)
				return
			}

			app.ActivationWithPulse(func(app *Application) (string, error) {
				if domainName, err := domain.GetName(); err != nil {
					return "", err
				} else if err := snapshot.RevertToSnapshot(0); err != nil {
					return "", err
				} else {
					return fmt.Sprintf(
						"Virtual Machine '%v' reverted to snapshot '%v'",
						domainName,
						snapshotName,
					), nil
				}
			})()
		}),
	)

	return "", nil
}

func (view *VirtualMachineView) move(app *Application) (string, error) {
	return "", nil
}

func (view *VirtualMachineView) addLabel(app *Application) (string, error) {
	return "", nil
}

func (view *VirtualMachineView) editXML(app *Application) (string, error) {

	// Read the domain XML
	domXml, err := view.Domain.GetXMLDesc(libvirt.DOMAIN_XML_SECURE)
	if err != nil {
		return "", err
	}

	// Create a temporary file for editing the XML
	filp, err := os.CreateTemp("", "vroomm-domain.*.xml")
	if err != nil {
		return "", err
	}

	// Ensure the temporary file is removed
	defer os.Remove(filp.Name())

	// Write the domain XML to the file
	if count, err := filp.WriteString(domXml); err != nil || count != len(domXml) {
		return "", fmt.Errorf("failed to save temporary domain definition")
	}

	// Ensure our writes get to disk
	if err := filp.Sync(); err != nil {
		return "", err
	}

	// Seek to the beginning for later
	if _, err := filp.Seek(0, 0); err != nil {
		return "", err
	}

	// Edit the file with xdg-open
	command := exec.Command(
		"xdg-open",
		filp.Name(),
	)
	command.Stdout = nil
	command.Stderr = nil
	command.Stdin = nil

	// Hide the application window
	glib.IdleAdd(func() {
		app.Window.Hide()
	})

	// Ensure we show the window when completed no matter what
	defer func() {
		glib.IdleAdd(func() {
			app.Window.Show()
		})
	}()

	if err := command.Run(); err != nil {
		return "", nil
	}

	if newDomXmlBytes, err := io.ReadAll(filp); err != nil {
		return "", fmt.Errorf("failed to read edited domain definition")
	} else if newDomXml := string(newDomXmlBytes); newDomXml == domXml {
		return "Virtual Machine XML Unchanged", nil
	} else if domain, err := app.Virt().DomainDefineXML(newDomXml); err != nil {
		return "", err
	} else if viewDomain, err := virt.NewDomain(*domain); err != nil {
		return "", err
	} else {
		view.Domain = viewDomain
		return "Updated Virtual Machine XML Definition", nil
	}
}

func addPropertyRow(grid *gtk.Grid, row int, labelText string, format string, args ...interface{}) {
	label := gtk.NewLabel(labelText)
	label.SetHAlign(gtk.AlignEnd)
	label.SetHExpand(false)
	grid.Attach(label, 0, row, 2, 1)

	entry := gtk.NewEntry()
	entry.SetEditable(false)
	entry.SetCanFocus(false)
	entry.SetText(fmt.Sprintf(format, args...))
	entry.SetHExpand(true)
	grid.Attach(entry, 2, row, 6, 1)
}
