package gui

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v3"
)

type LogView struct {
	*gtk.TextView
}

func NewLogView() *LogView {
	view := &LogView{
		TextView: gtk.NewTextView(),
	}
	view.SetWrapMode(gtk.WrapWord)

	return view
}

func (view *LogView) Name() string {
	return "Logs"
}

func (view *LogView) Enter(app *Application) error {
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
