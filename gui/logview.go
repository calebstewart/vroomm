package gui

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v3"
)

type LogView struct {
	TextView *gtk.TextView
	*gtk.ScrolledWindow
}

func NewLogView() *LogView {
	view := &LogView{
		ScrolledWindow: gtk.NewScrolledWindow(nil, nil),
		TextView:       gtk.NewTextView(),
	}
	view.TextView.SetWrapMode(gtk.WrapWord)
	view.ScrolledWindow.Add(view.TextView)
	view.ScrolledWindow.ShowAll()

	return view
}

func (view *LogView) Name() string {
	return "Logs"
}

func (view *LogView) Enter(app *Application) error {
	// Scroll to end of logs view
	view.TextView.ScrollToIter(view.TextView.Buffer().EndIter(), 0, true, 0.5, 0.5)
	return nil
}

func (view *LogView) Leave(app *Application) error {
	return nil
}

func (view *LogView) Close(app *Application) error {
	return nil
}

func (view *LogView) Widget() *gtk.Widget {
	return &view.TextView.Widget
}

func (view *LogView) Activate(app *Application) {
	app.Pop()
}

func (view *LogView) InvalidateFilter() {
	return
}
