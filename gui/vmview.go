package gui

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v3"
	"github.com/skratchdot/open-golang/open"
	"libvirt.org/go/libvirt"
	"libvirt.org/go/libvirtxml"

	"github.com/calebstewart/vroomm/set"
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

	interfaces, err := view.Domain.ListAllInterfaceAddresses(libvirt.DOMAIN_INTERFACE_ADDRESSES_SRC_AGENT)
	if err != nil {
		interfaces = []libvirt.DomainInterface{}
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
		view.CreateItem(app, "computer-symbolic", "Open Viewer", app.ActivationWithPulse("Opening with virt-viewer...", view.openViewer))
		view.CreateItem(app, "system-search-symbolic", "Open Looking Glass", app.ActivationWithPulse("Opening with looking-glass...", view.openLookingGlass))

		if len(interfaces) > 0 {
			view.CreateItem(app, "utilities-terminal-symbolic", "Open SSH Connection", app.Activation(view.openSSHConnection))
		}

		view.CreateItem(app, "system-shutdown-symbolic", "Shutdown", app.ActivationWithPulse("Requesting VM Shutdown...", view.shutDown))
		view.CreateItem(app, "face-shutmouth-symbolic", "Force Off", app.ActivationWithPulse("Forcing VM Off...", view.forceOff))
		view.CreateItem(app, "media-floppy-symbolic", "Save State", app.ActivationWithPulse("Saving VM State...", view.saveState))
	case libvirt.DOMAIN_CRASHED:
		fallthrough
	case libvirt.DOMAIN_SHUTOFF:
		fallthrough
	case libvirt.DOMAIN_SHUTDOWN:
		prettyState = "Off"
		view.CreateItem(app, "media-playback-start-symbolic", "Start", app.ActivationWithPulse("Starting VM...", view.start))
	}

	view.CreateItem(app, "edit-copy-symbolic", "Linked Clone", app.Activation(view.linkedClone))
	view.CreateItem(app, "edit-copy-symbolic", "Full Clone", app.Activation(view.fullClone))
	view.CreateItem(app, "camera-photo-symbolic", "Take Snapshot", app.Activation(view.snapshot))
	view.CreateItem(app, "document-open-recent-symbolic", "Restore Snapshot", app.Activation(view.restoreSnapshot))
	view.CreateItem(app, "user-trash-symbolic", "Delete Snapshot", app.Activation(view.deleteSnapshot))
	view.CreateItem(app, "folder-symbolic", "Move To...", app.Activation(view.move))
	view.CreateItem(app, "user-bookmarks-symbolic", "Add Label", app.Activation(view.addLabel))
	view.CreateItem(app, "user-bookmarks-symbolic", "Remove Label", app.Activation(view.removeLabel))
	view.CreateItem(app, "document-edit-symbolic", "Edit XML", app.ActivationWithPulse("Opening VM XML w/ xdg-open...", view.editXML))

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

	for idx, iface := range interfaces {
		if iface.Name == "lo" {
			continue
		}

		// Collect all the addresses
		addresses := []string{}
		for _, addr := range iface.Addrs {
			addresses = append(addresses, fmt.Sprintf("%v/%v", addr.Addr, addr.Prefix))
		}

		// Add the interface
		addPropertyRow(grid, 4+idx, fmt.Sprintf("Interface %v", iface.Name), strings.Join(addresses, ", "))
	}

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
						app.Logger.Error(err)
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

func (view *VirtualMachineView) openSSHConnection(app *Application) (string, error) {

	interfaces, err := view.Domain.ListAllInterfaceAddresses(libvirt.DOMAIN_INTERFACE_ADDRESSES_SRC_AGENT)
	if err != nil {
		return "", err
	} else if len(interfaces) == 0 {
		return "", fmt.Errorf("No interfaces found for domain '%v'", view.DomainName)
	}

	ifaceMap := map[string]*libvirt.DomainIPAddress{}
	ifaceAddrItems := []*LabelItem{}
	for ifaceIdx := range interfaces {
		for addrIdx := range interfaces[ifaceIdx].Addrs {
			// Ignore loopback interfaces
			if interfaces[ifaceIdx].Name == "lo" {
				continue
			}
			itemLabel := fmt.Sprintf("%v: %v", interfaces[ifaceIdx].Name, interfaces[ifaceIdx].Addrs[addrIdx].Addr)
			ifaceMap[itemLabel] = &interfaces[ifaceIdx].Addrs[addrIdx]
			ifaceAddrItems = append(ifaceAddrItems, NewLabelItem("network-server-symbolic", itemLabel))
		}
	}

	currentUser, err := user.Current()

	app.Push(
		NewPrompt(
			app,
			"Select Interface",
			"Interface>",
			true,
			func(app *Application, ifaceName string) {
				app.Logger.Info(ifaceName)
				addr := ifaceMap[ifaceName]
				app.Push(
					NewPrompt(
						app,
						fmt.Sprintf("Select User for %v", addr.Addr),
						"User>",
						false,
						func(app *Application, username string) {
							app.Pop()
							app.Pop()
							connStr := fmt.Sprintf("ssh://%v@%v", username, addr.Addr)
							app.ActivationWithPulse(
								fmt.Sprintf("Opening %v...", connStr),
								func(app *Application) (string, error) {
									if err := open.Start(connStr); err != nil {
										return "", err
									}

									app.Quit()
									return "SSH Session Opened", nil
								},
							)()
						},
						NewLabelItem("user-info-symbolic", currentUser.Username),
					))
			},
			ifaceAddrItems...,
		),
	)

	return "", nil
}

func (view *VirtualMachineView) linkedClone(app *Application) (string, error) {
	prompt := NewPrompt(app, "Linked Clone", "Clone Name>", false, func(app *Application, name string) {
		conn := app.Virt()
		if _, err := conn.LookupDomainByName(name); err == nil {
			app.Logger.Errorf("Virtual Machine '%v' already exists", name)
			return
		}

		app.ActivationWithPulse(
			"Creating linked VM clone...",
			func(app *Application) (string, error) {
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
			},
		)()
	})

	app.Push(prompt)

	return "", nil
}

func (view *VirtualMachineView) fullClone(app *Application) (string, error) {
	prompt := NewPrompt(app, "Full Clone", "Clone Name>", false, func(app *Application, name string) {
		conn := app.Virt()
		if _, err := conn.LookupDomainByName(name); err == nil {
			app.Logger.Errorf("Virtual Machine '%v' already exists", name)
			return
		}

		app.ActivationWithPulse(
			"Creating full VM clone...",
			func(app *Application) (string, error) {
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
			},
		)()
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
			app,
			"Snapshot",
			"Snapshot Name>",
			false,
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
			app.Logger.Error(err)
			return
		} else if err := xml.Unmarshal([]byte(xmlDescr), &domainDescription); err != nil {
			app.Logger.Error(err)
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
			app.Logger.Error(err.Error())
		} else if _, err := view.Domain.CreateSnapshotXML(string(snapshotXml), 0); err != nil {
			app.Logger.Error(err.Error())
		} else {
			app.Logger.Infof("Created Snapshot '%v' of '%v'", name, domainDescription.Name)
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
				app.Logger.Error(err)
				return
			}

			app.ActivationWithPulse(
				"Deleting VM snapshot...",
				func(app *Application) (string, error) {
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
				},
			)()
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
				app.Logger.Error(err)
				return
			}

			app.ActivationWithPulse(
				"Restoring VM snapshot...",
				func(app *Application) (string, error) {
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
				},
			)()
		}),
	)

	return "", nil
}

func (view *VirtualMachineView) move(app *Application) (string, error) {

	virtConn := app.Virt()
	domains, err := virtConn.EnumerateAllDomains()
	if err != nil {
		return "", err
	}

	folders := set.New[string]()
	for _, domain := range domains {
		info := domain.GetVmmData()
		folders.Add(info.Path)
	}

	items := []*LabelItem{}
	for folder := range folders {
		items = append(items, NewLabelItem(folderIcon, folder))
	}

	app.Push(
		NewPrompt(
			app,
			"Move to Folder",
			"Path>",
			false,
			func(app *Application, entry string) {
				info := view.Domain.GetVmmData()
				info.Path = strings.TrimSuffix(entry, "/") + "/"
				view.Domain.UpdateVmmData(info)
				app.Logger.Infof("Moved '%v' to '%v'", view.DomainName, info.Path)
				app.Pop()
			},
			items...,
		),
	)

	return "", nil
}

func (view *VirtualMachineView) addLabel(app *Application) (string, error) {

	virtConn := app.Virt()
	domains, err := virtConn.EnumerateAllDomains()
	if err != nil {
		return "", err
	}

	existingLabels := set.New(view.Domain.GetVmmData().Labels...)

	labels := set.New[string]()
	for _, domain := range domains {
		info := domain.GetVmmData()
		labels.Add(info.Labels...)
	}

	items := []*LabelItem{}
	for label := range labels {
		if !existingLabels.Has(label) {
			items = append(items, NewLabelItem("user-bookmarks-symbolic", label))
		}
	}

	app.Push(
		NewPrompt(
			app,
			"Add VM Label",
			"Label>",
			false,
			func(app *Application, entry string) {
				info := view.Domain.GetVmmData()

				labels := set.New(info.Labels...)
				labels.Add(entry)
				info.Labels = labels.Array()

				view.Domain.UpdateVmmData(info)

				app.Logger.Infof("Added lable '%v' to VM '%v'", entry, view.DomainName)
				app.Pop()
			},
			items...,
		),
	)

	return "", nil
}

func (view *VirtualMachineView) removeLabel(app *Application) (string, error) {
	labels := set.New(view.Domain.GetVmmData().Labels...)

	items := []*LabelItem{}
	for label := range labels {
		items = append(items, NewLabelItem("user-bookmarks-symbolic", label))
	}

	app.Push(
		NewPrompt(
			app,
			"Remove VM Label",
			"Label>",
			true,
			func(app *Application, entry string) {
				info := view.Domain.GetVmmData()

				// Remove the label
				oldLabels := info.Labels
				info.Labels = []string{}
				for _, label := range oldLabels {
					if label != entry {
						info.Labels = append(info.Labels, label)
					}
				}

				view.Domain.UpdateVmmData(info)

				app.Logger.Infof("Removed VM label '%v' from '%v'", entry, view.DomainName)
				app.Pop()
			},
			items...,
		),
	)

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
