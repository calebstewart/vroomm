package gui

import (
	"context"
	"sort"
	"strings"

	"github.com/diamondburned/gotk4/pkg/glib/v2"

	"github.com/calebstewart/vroomm/set"
	"github.com/calebstewart/vroomm/virt"
)

const (
	folderIcon = "user-desktop-symbolic"
)

type BrowseAllView struct {
	*FlowboxMenu
}

type BrowseFolderView struct {
	Folder string
	*FlowboxMenu
}

func NewBrowseAllItem(app *Application) *LabelItem {
	item := NewLabelItem(folderIcon, "Browse All")

	item.ConnectActivate(func() {
		app.Push(NewBrowseAllView())
	})

	return item
}

func NewBrowseFolderItem(app *Application, parent string, folder string) *LabelItem {
	name := folder
	if folder == "" && parent == "/" {
		name = "Browse Path"
	}
	return NewLabelItemWithAction(folderIcon, name, func() {
		app.Push(NewBrowseFolderView(name, parent+folder))
	})
}

func NewBrowseAllView() *BrowseAllView {
	return &BrowseAllView{
		FlowboxMenu: NewFlowboxMenu("Browse All"),
	}
}

func (menu *BrowseAllView) Enter(app *Application) error {
	// Remove all children
	menu.EmptyItems()

	go func() {
		virt := app.Virt()
		if domains, err := virt.EnumerateAllDomains(); err != nil {
			app.Logger.Error(err.Error())
		} else {
			glib.IdleAdd(func() {
				for _, domain := range domains {
					if item, err := NewVirtualMachineItem(app, domain); err != nil {
						app.Logger.Error(err.Error())
					} else {
						item.ShowAll()
						menu.Add(item)
					}
				}
				menu.InvalidateFilter()
			})
		}
	}()

	if err := menu.FlowboxMenu.Enter(app); err != nil {
		return err
	}

	return nil
}

func (menu *BrowseAllView) Close(app *Application) error {
	return nil
}

func (menu *BrowseAllView) Leave(app *Application) error {
	return nil
}

func NewBrowseFolderView(name string, folder string) *BrowseFolderView {
	if !strings.HasSuffix(folder, "/") {
		folder = folder + "/"
	}

	return &BrowseFolderView{
		Folder:      folder,
		FlowboxMenu: NewFlowboxMenu(strings.TrimSuffix(name, "/")),
	}
}

func (menu *BrowseFolderView) Enter(app *Application) error {
	// Remove all children
	menu.EmptyItems()

	go func() {
		virtConn := app.Virt()
		if domains, err := virtConn.EnumerateAllDomains(); err != nil {
			app.Logger.Error(err.Error())
		} else {
			glib.IdleAdd(func() {

				// Array of sub folders to show items for
				folders := []string{}
				// Array of domains to show items for
				directDomains := []*virt.Domain{}

				// Collect all direct domains and potential child folders
				for _, domain := range domains {
					metadata := domain.GetVmmData()
					if metadata.Path == menu.Folder {
						directDomains = append(directDomains, domain)
					} else if strings.HasPrefix(metadata.Path, menu.Folder) {
						folders = append(folders, metadata.Path)
					}
				}

				directFolders := []string{}

				// Sort the folders by length
				sort.Slice(folders, func(i, j int) bool {
					return len(folders[i]) < len(folders[j])
				})

				// Find all unique direct child folders
			outer:
				for _, folder := range folders {
					for _, existing := range directFolders {
						if strings.HasPrefix(folder, existing) {
							continue outer
						}
					}

					directFolders = append(directFolders, folder)
				}

				for _, folder := range directFolders {
					menu.Add(NewBrowseFolderItem(app, menu.Folder, strings.TrimPrefix(folder, menu.Folder)))
				}

				for _, domain := range directDomains {
					if item, err := NewVirtualMachineItem(app, domain); err != nil {
						app.Logger.Error(err.Error())
					} else {
						item.ShowAll()
						menu.Add(item)
					}
				}

				menu.InvalidateFilter()
			})
		}
	}()

	if err := menu.FlowboxMenu.Enter(app); err != nil {
		return err
	}

	return nil
}

func (menu *BrowseFolderView) Close(app *Application) error {
	return nil
}

func (menu *BrowseFolderView) Leave(app *Application) error {
	return nil
}

// A menu showing all existing labels
type LabelsView struct {
	*FlowboxMenu
}

func NewLabelsView() *LabelsView {
	return &LabelsView{
		FlowboxMenu: NewFlowboxMenu("Browse Labels"),
	}
}

func NewLabelsViewItem(app *Application) *LabelItem {
	return NewLabelItemWithAction("user-bookmarks-symbolic", "Browse Labels", func() {
		app.Push(NewLabelsView())
	})
}

func (view *LabelsView) Enter(app *Application) error {
	ctx, cancel := context.WithCancel(context.Background())

	view.EmptyItems()
	app.PulseProgress(ctx, "Loading Labels...")

	go func() {
		virtConn := app.Virt()
		if domains, err := virtConn.EnumerateAllDomains(); err != nil {
			app.Logger.Error(err.Error())
		} else {
			// Collect all valid labels from all domains
			labels := set.Set[string]{}
			for _, domain := range domains {
				metadata := domain.GetVmmData()
				labels.Add(metadata.Labels...)
			}

			// Create items for those labels
			glib.IdleAdd(func() {
				for label := range labels {
					view.Add(NewLabelItemWithAction("user-bookmarks-symbolic", label, func() {
						app.Push(NewBrowseLabelView(label))
					}))
				}
				cancel()
			})
		}
	}()

	return view.FlowboxMenu.Enter(app)
}

func (view *LabelsView) Leave(app *Application) error {
	return nil
}

func (view *LabelsView) Close(app *Application) error {
	return nil
}

// A menu showing all VMs with a specific label
type BrowseLabelView struct {
	Label string
	*FlowboxMenu
}

func NewBrowseLabelView(label string) *BrowseLabelView {
	return &BrowseLabelView{
		Label:       label,
		FlowboxMenu: NewFlowboxMenu(label),
	}
}

func (view *BrowseLabelView) Enter(app *Application) error {
	ctx, cancel := context.WithCancel(context.Background())

	view.EmptyItems()
	app.PulseProgress(ctx, "Loading Labels...")

	go func() {
		virtConn := app.Virt()
		if domains, err := virtConn.EnumerateAllDomains(); err != nil {
			app.Logger.Error(err.Error())
		} else {
			glib.IdleAdd(func() {
				// Collect all direct domains and potential child folders
				for _, domain := range domains {
					metadata := domain.GetVmmData()
					labels := set.New(metadata.Labels...)

					// Add any VMs with this label
					if labels.Has(view.Label) {
						item, err := NewVirtualMachineItem(app, domain)
						if err != nil {
							app.Logger.Error(err.Error())
						} else {
							view.Add(item)
						}
					}
				}

				// Ensure the pulsing stops
				cancel()
			})
		}
	}()

	return view.FlowboxMenu.Enter(app)
}

func (menu *BrowseLabelView) Close(app *Application) error {
	return nil
}

func (menu *BrowseLabelView) Leave(app *Application) error {
	return nil
}
