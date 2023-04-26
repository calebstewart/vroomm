package gui

import (
	"errors"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v3"
	"libvirt.org/go/libvirt"
)

var (
	credentialTypeName = map[libvirt.ConnectCredentialType]string{
		libvirt.CRED_USERNAME:     "User",
		libvirt.CRED_AUTHNAME:     "User",
		libvirt.CRED_PASSPHRASE:   "Passphrase",
		libvirt.CRED_ECHOPROMPT:   "Prompt",
		libvirt.CRED_NOECHOPROMPT: "Sensitive Prompt",
		libvirt.CRED_REALM:        "Realm",
	}
	cancelled = errors.New("User cancelled")
)

type AuthPromptView struct {
	Credential *libvirt.ConnectCredential // The auth prompt sent from libvirt
	ConnectUri string                     // The connection string we are using that needs authentication
	Done       bool                       // Whether the user submitted input
	Result     chan error                 // Where to send the completion notification
	*gtk.Box                              // The emtpy box displayed in the view
}

func NewLibvirtConnectAuth(app *Application) *libvirt.ConnectAuth {
	return &libvirt.ConnectAuth{
		CredType: []libvirt.ConnectCredentialType{
			libvirt.CRED_USERNAME,
			libvirt.CRED_AUTHNAME,
			libvirt.CRED_PASSPHRASE,
			libvirt.CRED_ECHOPROMPT,
			libvirt.CRED_NOECHOPROMPT,
			libvirt.CRED_REALM,
		},
		Callback: func(creds []*libvirt.ConnectCredential) {
			libvirtConnectCredentialCallback(app, creds)
		},
	}
}

func libvirtConnectCredentialCallback(app *Application, creds []*libvirt.ConnectCredential) {

	for _, cred := range creds {
		// Create a channel to recieve results from the GUI thread about credentials
		resultChan := make(chan error)

		// Display a prompt that asks for the relevant input
		view := NewAuthPromptView(cred, resultChan)
		glib.IdleAdd(func() {
			app.Push(view)
		})

		// Wait for an answer
		result := <-resultChan

		// Did the prompt fail for some reason?
		if result != nil {
			glib.IdleAdd(func() {
				app.AddError(result)
			})
			return
		}

		// Prompt did not fail, so prompt for the next one!
	}
}

func NewAuthPromptView(cred *libvirt.ConnectCredential, result chan error) *AuthPromptView {
	return nil
}

func (view *AuthPromptView) Name() string {
	return fmt.Sprintf("Authentication %v Prompt", credentialTypeName[view.Credential.Type])
}

func (view *AuthPromptView) Enter(app *Application) error {
	// Set the prompt to what we got from libvirt
	app.Prompt.SetText(fmt.Sprintf("%v>", view.Credential.Prompt))

	// Enable password mode if appropriate
	if view.Credential.Type == libvirt.CRED_PASSPHRASE || view.Credential.Type == libvirt.CRED_NOECHOPROMPT {
		app.Entry.SetVisibility(false)
	}

	return nil
}

func (view *AuthPromptView) Leave(app *Application) error {
	// Reset text and disable password mode upon leaving
	app.Entry.SetText("")
	app.Entry.SetVisibility(true)

	if !view.Done {
		view.Result <- cancelled
	}

	return nil
}

func (view *AuthPromptView) Close(app *Application) error {
	// Reset text and disable password mode upon leaving
	app.Entry.SetText("")
	app.Entry.SetVisibility(true)

	if !view.Done {
		view.Result <- cancelled
	}

	return nil
}

func (view *AuthPromptView) Widget() *gtk.Widget {
	return &view.Box.Widget
}

func (view *AuthPromptView) Activate(app *Application) {
	// Update the internal credential object
	view.Credential.Result = app.Entry.Text()
	view.Credential.ResultLen = len(app.Entry.Text())
	// Let the libvirt thread know we are done
	view.Result <- nil
	// Calling pop triggers a Close/Leave, so we need to let those
	// methods know not to send a value to view.Result.
	view.Done = true
	// Exit this view
	app.Pop()
}

func (view *AuthPromptView) InvalidateFilter() {}
