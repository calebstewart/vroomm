package gui

import (
	"fmt"
)

type PromptAction func(app *Application, input string)

type Prompt struct {
	Title  string
	Prompt string
	Action PromptAction
	*FlowboxMenu
}

func NewPrompt(title string, prompt string, action PromptAction) *Prompt {
	return &Prompt{
		Title:       title,
		Prompt:      prompt,
		Action:      action,
		FlowboxMenu: NewFlowboxMenu(title),
	}
}

func (view *Prompt) Name() string {
	return view.Title
}

func (view *Prompt) Enter(app *Application) error {
	app.Prompt.SetText(view.Prompt)
	return view.FlowboxMenu.Enter(app)
}

func (view *Prompt) Activate(app *Application) {
	input := app.Entry.Text()
	if input == "" {
		app.AddError(fmt.Errorf("No input provided"))
		return
	}

	view.Action(app, input)
}

func (view *Prompt) Leave(app *Application) error {
	return nil
}

func (view *Prompt) Close(app *Application) error {
	return nil
}
