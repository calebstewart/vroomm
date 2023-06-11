package gui

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v3"
)

type PromptAction func(app *Application, input string)

type Prompt struct {
	Title  string
	Prompt string
	Action PromptAction
	*FlowboxMenu
}

type PromptNewItem struct {
	single bool
	*LabelItem
}

func NewPromptNewItem(app *Application, single bool, action PromptAction) *PromptNewItem {
	return &PromptNewItem{
		single: single,
		LabelItem: NewLabelItemWithAction("tab-new-symbolic", "New...", func() {
			action(app, app.Entry.Text())
		}),
	}
}

func (item *PromptNewItem) Match(query string) bool {
	if query == "" && !item.single {
		return false
	} else {
		item.Child().(*gtk.Box).Children()[1].(*gtk.Label).SetText(fmt.Sprintf("New... %v", query))
		return true
	}
}

func NewPrompt(app *Application, title string, prompt string, requireExisting bool, action PromptAction, items ...*LabelItem) *Prompt {
	view := &Prompt{
		Title:       title,
		Prompt:      prompt,
		Action:      action,
		FlowboxMenu: NewFlowboxMenu(title),
	}

	if !requireExisting {
		view.Add(NewPromptNewItem(app, len(items) == 0, action))
	}

	for idx := range items {
		var item *LabelItem = items[idx]
		item.ConnectActivate(func() {
			action(app, item.Text)
		})
		view.Add(item)
	}

	return view
}

func (view *Prompt) Name() string {
	return view.Title
}

func (view *Prompt) Enter(app *Application) error {
	app.Prompt.SetText(view.Prompt)
	return view.FlowboxMenu.Enter(app)
}

func (view *Prompt) Leave(app *Application) error {
	return nil
}

func (view *Prompt) Close(app *Application) error {
	return nil
}
