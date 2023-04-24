package gui

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v3"
	"github.com/lithammer/fuzzysearch/fuzzy"
)

type named interface {
	Name() string
}

type Item interface {
	FilterText() string
	gtk.Widgetter
	named
}

// Generic menu consisting of a list of filterable list
// items with activations
type FlowboxMenu struct {
	name    string
	items   map[string]Item
	FlowBox *gtk.FlowBox
	*gtk.ScrolledWindow
}

func NewFlowboxMenu(name string) *FlowboxMenu {
	menu := &FlowboxMenu{
		name:           name,
		FlowBox:        gtk.NewFlowBox(),
		ScrolledWindow: gtk.NewScrolledWindow(nil, nil),
		items:          map[string]Item{},
	}

	menu.FlowBox.CastType(gtk.GTypeOrientable).(*gtk.Orientable).SetOrientation(gtk.OrientationHorizontal)
	menu.FlowBox.SetMaxChildrenPerLine(1)
	menu.FlowBox.SetVAlign(gtk.AlignStart)
	menu.FlowBox.SetHAlign(gtk.AlignFill)
	menu.FlowBox.SetHAdjustment(menu.ScrolledWindow.HAdjustment())
	menu.FlowBox.SetVAdjustment(menu.ScrolledWindow.VAdjustment())
	menu.ScrolledWindow.Add(menu.FlowBox)

	menu.ShowAll()

	return menu
}

func (menu *FlowboxMenu) Activate(app *Application) {
	children := menu.FlowBox.SelectedChildren()
	if len(children) == 0 {
		children = []*gtk.FlowBoxChild{menu.FlowBox.ChildAtIndex(0)}
	}

	children[0].Activate()
}

func (menu *FlowboxMenu) Widget() *gtk.Widget {
	return &menu.FlowBox.Widget
}

func (menu *FlowboxMenu) Name() string {
	return menu.name
}

func (menu *FlowboxMenu) InvalidateFilter() {
	menu.FlowBox.UnselectAll()
	menu.FlowBox.InvalidateFilter()
}

func (menu *FlowboxMenu) Enter(app *Application) error {
	// Connect filtering to the application entry field
	menu.FlowBox.SetFilterFunc(func(child *gtk.FlowBoxChild) bool {
		return menu.filter(app, child)
	})
	return nil
}

func (menu *FlowboxMenu) filter(app *Application, child *gtk.FlowBoxChild) bool {
	// Get the child widget, which should be the item widget
	if item, ok := menu.items[child.Name()]; !ok {
		return false
	} else {
		// Use a simple contains for now
		result := app.Entry.Text() == "" || fuzzy.MatchFold(app.Entry.Text(), item.FilterText())
		if result && len(menu.FlowBox.SelectedChildren()) == 0 {
			menu.FlowBox.SelectChild(child)
		}

		return result
	}
}

func (menu *FlowboxMenu) Add(item Item) {
	menu.items[item.Name()] = item
	menu.FlowBox.Add(item)
}

func (menu *FlowboxMenu) EmptyItems() {
	menu.items = map[string]Item{}
	for _, child := range menu.FlowBox.Children() {
		menu.FlowBox.Remove(child)
	}
}

type LabelItem struct {
	Text string
	*gtk.FlowBoxChild
}

func NewLabelItem(iconName string, text string) *LabelItem {
	item := &LabelItem{
		Text:         text,
		FlowBoxChild: gtk.NewFlowBoxChild(),
	}

	box := gtk.NewBox(gtk.OrientationHorizontal, 5)

	icon := gtk.NewImageFromIconName(iconName, int(gtk.IconSizeLargeToolbar))
	box.PackStart(icon, false, false, 0)

	label := gtk.NewLabel(text)
	label.SetHAlign(gtk.AlignStart)
	box.PackStart(label, true, true, 0)

	item.Add(box)
	item.SetHExpand(true)
	item.SetName(text)
	item.ShowAll()

	return item
}

func NewLabelItemWithAction(icon string, text string, action func()) *LabelItem {
	item := NewLabelItem(icon, text)
	item.FlowBoxChild.ConnectActivate(action)
	return item
}

func (item *LabelItem) FilterText() string {
	return item.Text
}

type BrowseAllMenu struct {
	*FlowboxMenu
}

type MainMenu struct {
	*FlowboxMenu
}

func NewMainMenu() *MainMenu {
	menu := &MainMenu{
		FlowboxMenu: NewFlowboxMenu(""),
	}

	return menu
}

func (menu *MainMenu) Enter(app *Application) error {
	menu.EmptyItems()
	menu.Add(NewBrowseAllItem(app))
	menu.Add(NewBrowseFolderItem(app, "/", ""))
	menu.Add(NewLabelsViewItem(app))
	return menu.FlowboxMenu.Enter(app)
}

func (menu *MainMenu) Close(app *Application) error {
	return nil
}

func (menu *MainMenu) Leave(app *Application) error {
	return nil
}
