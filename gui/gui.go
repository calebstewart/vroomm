package gui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/adrg/xdg"
	layershell "github.com/diamondburned/gotk4-layer-shell/pkg/gtklayershell"
	"github.com/diamondburned/gotk4/pkg/gdk/v3"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v3"
	"github.com/sirupsen/logrus"

	"github.com/calebstewart/vroomm/config"
	"github.com/calebstewart/vroomm/resources"
	"github.com/calebstewart/vroomm/virt"
)

const (
	VroommApplicationId string = "art.calebstew.vroomm"
)

// An individual view within the application
// For example, a folder menu or a VM view
type View interface {
	Name() string                 // Get the display name for this view
	Enter(app *Application) error // Show this view (called every time it is brought to the focus)
	Leave(app *Application) error // Called when the view is unfocused (i.e. a new view is added)
	Close(app *Application) error // Called when the view is closed permanently
	Widget() *gtk.Widget          // Get the widget
	Activate(app *Application)    // Activate the widget
	InvalidateFilter()
	gtk.Widgetter
}

// Vroomm application
type Application struct {
	Config           *config.Config         // Application configuration
	Builder          *gtk.Builder           // Builder that holds the primary widgets
	Window           *gtk.ApplicationWindow // Application Window
	RootBox          *gtk.Box               // The root gtk.Box holding all window widgets
	Prompt           *gtk.Label             // Prompt label
	Entry            *gtk.Entry             // Entry widget where the user interacts
	ViewTitle        *gtk.Label             // Label showing the view path
	Stack            *gtk.Stack             // View stack
	MessageBox       *gtk.Box               // A box which contains error or warning messages
	Views            []View                 // Slice of views currently in the stack
	Logger           *logrus.Logger         // A logger used to dump console and GUI logs
	LogView          *LogView               // A view that displays log entries interactively
	InfoBar          *gtk.InfoBar           // The info bar displaying the most recent log message
	ViewLock         sync.Mutex             // A lock for switching views
	virtConn         *virt.Connection       // Libvirt connection object
	*gtk.Application                        // GTK Application
}

func NewApplication(cfg *config.Config) *Application {
	app := &Application{
		Config:      cfg,
		Application: gtk.NewApplication(VroommApplicationId, gio.ApplicationFlagsNone),
		Views:       make([]View, 0),
		Logger:      logrus.StandardLogger(),
	}

	app.ConnectActivate(app.activate)

	return app
}

func (app *Application) Virt() *virt.Connection {
	if app.virtConn != nil {
		if alive, err := app.virtConn.IsAlive(); err == nil && alive {
			return app.virtConn
		}
	}

	if conn, err := virt.New(app.Config.ConnectionString); err != nil {
		logrus.WithError(err).WithField("connect_uri", app.Config.ConnectionString).Error("failed to connect to libvirt")
		app.Quit()
		return nil
	} else {
		app.virtConn = conn
		return conn
	}
}

func (app *Application) activate() {

	if app.Config.UseStyle {
		cssProvider := gtk.NewCSSProvider()
		loaded := false

		// Load explicit style path
		if app.Config.Style != "" {
			if err := cssProvider.LoadFromPath(app.Config.Style); err != nil {
				logrus.WithError(err).WithField("path", app.Config.Style).Error("failed to load configured stylesheet")
			} else {
				loaded = true
			}
		}

		// Nope, so we load styles from XDG directories
		if !loaded {
			paths := []string{xdg.ConfigHome}
			paths = append(paths, xdg.ConfigDirs...)
			for _, path := range paths {
				style_path := filepath.Join(path, "vroomm", "style.css")
				if _, err := os.Stat(style_path); errors.Is(err, os.ErrNotExist) {
					continue
				} else if err := cssProvider.LoadFromPath(style_path); err != nil {
					logrus.WithError(err).WithField("path", style_path).Error("failed to load stylesheet")
				} else {
					loaded = true
					break
				}
			}
		}

		// Still no, so we load built-in style
		if !loaded {
			if cssData, err := resources.Read("style.css"); err != nil {
				logrus.WithError(err).Error("failed to load bundled css")
			} else if err := cssProvider.LoadFromData(string(cssData)); err != nil {
				// Damn, still nothing. We give up.
				logrus.WithError(err).Error("failed to load bundled css")
			}
		}

		gtk.StyleContextAddProviderForScreen(gdk.ScreenGetDefault(), cssProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	}

	if builderDef, err := resources.Read("main-window.glade"); err != nil {
		logrus.WithError(err).Error("failed to load interface definition")
		app.Quit()
		return
	} else {
		app.Builder = gtk.NewBuilderFromString(string(builderDef), -1)
	}

	// Lookup the main window and add it to the application
	app.Window = app.Builder.GetObject("main").Cast().(*gtk.ApplicationWindow)
	app.Window.SetApplication(app.Application)

	// Lookup all the internal widgets
	app.Prompt = app.Builder.GetObject("prompt").Cast().(*gtk.Label)
	app.Entry = app.Builder.GetObject("input").Cast().(*gtk.Entry)
	app.ViewTitle = app.Builder.GetObject("view-title").Cast().(*gtk.Label)
	app.Stack = app.Builder.GetObject("view").Cast().(*gtk.Stack)
	app.RootBox = app.Builder.GetObject("root").Cast().(*gtk.Box)

	// Create a view for the logs
	app.LogView = NewLogView()

	// Create the info bar. It is only shown when there's a message to show.
	app.InfoBar = gtk.NewInfoBar()
	app.InfoBar.ContentArea().PackStart(gtk.NewLabel("Ready..."), true, true, 0)
	app.InfoBar.AddButton("Logs", int(gtk.ResponseOK))
	app.InfoBar.ConnectResponse(func(_ int) {
		app.Push(app.LogView)
	})
	app.RootBox.PackStart(app.InfoBar, false, false, 0)

	// Setup wlr-layer-shell if requested
	if app.Config.LayerShell.Enabled {
		// Configure layershell
		layershell.InitForWindow(&app.Window.Window)
		layershell.AutoExclusiveZoneEnable(&app.Window.Window)
		layershell.SetLayer(&app.Window.Window, layershell.LayerShellLayerTop)
		layershell.SetKeyboardInteractivity(&app.Window.Window, true)
		layershell.SetKeyboardMode(&app.Window.Window, layershell.LayerShellKeyboardModeExclusive)
	}

	// Enable key press events for the root window
	app.Window.AddEvents(int(gdk.KeyPressMask))

	// Handle escape globally so we can exit out of views
	app.Window.ConnectKeyPressEvent(app.keyPressEvent)

	// Ensure that modifying the entry invalidates any filters applied
	// in the view
	app.Entry.ConnectChanged(func() {
		app.Top().InvalidateFilter()
	})

	app.Logger.AddHook(app)

	// Setup the initial menu view
	mainMenu := NewMainMenu()
	app.Stack.AddNamed(mainMenu, mainMenu.Name())
	app.Views = []View{mainMenu}
	app.Stack.SetVisibleChildFull(mainMenu.Name(), gtk.StackTransitionTypeNone)
	mainMenu.Enter(app)
	app.updateViewTitle()

	app.Window.ShowAll()

	if app.Config.LayerShell.Enabled {
		// This is stupid, but it's the only way to properly configure the size
		// since GTK reports the display size of the default window for an indeterminant
		// amount of time after creation, and there is no signal for when this changes.
		//
		// Instead, we just run this background routine which executes updateWindowGeometry
		// every millisecond for up to 1 second, and then quits. It works, but I'm not happy
		// about it.
		go func() {
			ctx, _ := context.WithTimeout(context.Background(), time.Second)
			for {
				select {
				case <-time.After(time.Millisecond):
					app.updateWindowGeometry(app.Window)
				case <-ctx.Done():
					app.Logger.Info("geometry thread exiting")
					return
				}
			}
		}()
	}

	// Setup the initial geometry
	app.updateWindowGeometry(app.Window)
}

// Handler for 'configure-event' which handles changes in geometry based on
// the GdkDisplay geometry.
func (app *Application) updateWindowGeometry(_ *gtk.ApplicationWindow) bool {
	if !app.Window.Realized() {
		// We are not realized yet, so just set the size request to 0 so we dont
		// have a weird flickering affect when we resize appropriately
		app.Window.SetSizeRequest(100, 100)
	} else {
		// Find the active monitor's geometry to calculate the correct window size
		monitor := app.Window.Screen().MonitorGeometry(app.Window.Screen().MonitorAtWindow(app.Window.Window.Window()))

		// Get the current window gemoetry
		currentWidth, currentHeight := app.Window.GetSizeRequest()

		// Calculate the new geometry from the monitor geometry
		width := (monitor.Width() * app.Config.LayerShell.Width) / 100
		height := (monitor.Height() * app.Config.LayerShell.Height) / 100

		// Update if needed
		if currentWidth != width || currentHeight != height {
			logrus.Infof("screen %vx%v, resizing from %vx%v to %vx%v", monitor.Width(), monitor.Height(), currentWidth, currentHeight, width, height)
			app.Window.SetSizeRequest(width, height)
			app.Window.SetResizable(false)
		}
	}

	return false
}

func (app *Application) updateViewTitle() {
	titles := []string{}
	for _, view := range app.Views {
		titles = append(titles, view.Name())
	}

	title := strings.Join(titles, " / ")
	if title == "" {
		title = "/"
	}

	app.ViewTitle.SetText(title)
}

// Handle key presses at the top level
func (app *Application) keyPressEvent(event *gdk.EventKey) bool {
	// Always handle escape to go back
	if event.Keyval() == gdk.KEY_Escape {
		if event.State().Has(gdk.ShiftMask) {
			app.Quit()
		} else {
			app.Pop()
		}
		return true
	} else if event.Keyval() == gdk.KEY_l && event.State().Has(gdk.ControlMask) {
		if app.Top() != app.LogView {
			app.Push(app.LogView)
		} else {
			app.Pop()
		}
	} else if app.Entry.HasFocus() {
		// The entry has focus
		if event.Keyval() == gdk.KEY_Return {
			view := app.Top()
			view.Activate(app)
			return true
		} else if event.Keyval() == gdk.KEY_Down {
			app.Top().Widget().ChildFocus(gtk.DirDown)
			app.Top().Widget().GrabFocus()
			return false
		}
	} else {
		// The entry does not have focus
		// Check if the entry is printable, in which case, we focus the entry
		if event.Keyval() != gdk.KEY_Up && event.Keyval() != gdk.KEY_Down && event.Keyval() != gdk.KEY_Return {
			app.Entry.GrabFocusWithoutSelecting()
		}
	}

	return false
}

func (app *Application) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (app *Application) Fire(entry *logrus.Entry) error {

	glib.IdleAdd(func() {
		buffer := app.LogView.Widget().Cast().(*gtk.TextView).Buffer()
		buffer.Insert(
			buffer.EndIter(),
			fmt.Sprintf(
				"[%v] %v - %v\n",
				entry.Time.Format(time.RFC3339),
				entry.Level.String(),
				entry.Message,
			),
		)

		box := app.InfoBar.ContentArea()
		for _, child := range box.Children() {
			box.Remove(child)
		}

		iconName := "dialog-information"
		if entry.Level == logrus.ErrorLevel || entry.Level == logrus.FatalLevel || entry.Level == logrus.PanicLevel {
			iconName = "dialog-error"
			app.InfoBar.SetMessageType(gtk.MessageError)
		} else if entry.Level == logrus.WarnLevel {
			iconName = "dialog-warning"
			app.InfoBar.SetMessageType(gtk.MessageWarning)
		} else {
			app.InfoBar.SetMessageType(gtk.MessageInfo)
		}

		box.Add(gtk.NewImageFromIconName(iconName, int(gtk.IconSizeSmallToolbar)))
		box.Add(gtk.NewLabel(entry.Message))

		app.InfoBar.Hide()
		app.InfoBar.ShowAll()
	})

	return nil
}

func (app *Application) PulseProgress(ctx context.Context, text string) {

	app.Logger.Info(text)
	app.StartProgress()

	go func() {
		for {
			select {
			case <-ctx.Done():
				glib.IdleAdd(func() {
					app.StopProgress()
				})
				return
			case <-time.After(100 * time.Millisecond):
				glib.IdleAdd(func() {
					app.Entry.ProgressPulse()
				})
			}
		}
	}()
}

func (app *Application) ActivationWithPulse(message string, activate func(app *Application) (string, error)) func() {
	return func() {
		ctx, cancel := context.WithCancel(context.Background())
		app.PulseProgress(ctx, message)

		go func() {
			status, err := activate(app)
			cancel()

			glib.IdleAdd(func() {
				if err != nil {
					app.Logger.Error(err.Error())
				} else {
					app.Logger.Info(status)
				}
			})
		}()
	}
}

func (app *Application) Activation(activate func(app *Application) (string, error)) func() {
	return func() {
		status, err := activate(app)
		if err != nil {
			app.Logger.Error(err.Error())
		} else if status != "" {
			app.Logger.Info(status)
		}
	}
}

func (app *Application) StartProgress() {
	app.Entry.ProgressPulse()
}

func (app *Application) StopProgress() {
	app.Entry.SetProgressFraction(0)
}

// Push a new view onto the GtkStack
func (app *Application) Push(view View) {

	if !app.ViewLock.TryLock() {
		return
	}

	// Grab the current view
	current := app.Top()

	if view == app.LogView && current == app.LogView {
		app.ViewLock.Unlock()
		return
	}

	// Ensure we can leave this view
	if err := current.Leave(app); err != nil {
		app.Logger.Error(err.Error())
		app.ViewLock.Unlock()
		return
	}

	// Add the view to the GtkStack
	app.Stack.AddNamed(view, view.Name())

	// Add the view to our view slice
	app.Views = append(app.Views, view)

	// Transition to the new view
	view.Widget().ShowAll()

	transitionType := gtk.StackTransitionTypeSlideLeft
	if view == app.LogView {
		transitionType = gtk.StackTransitionTypeSlideUp
	}
	app.Stack.SetVisibleChildFull(view.Name(), transitionType)

	app.updateViewTitle()
	app.Prompt.SetText("VM Manager>")

	// Notify the view of the new focus
	if err := view.Enter(app); err != nil {
		app.Logger.Error(err.Error())
	}

	// Reset the input box
	app.Entry.SetText("")
	app.Window.SetFocus(app.Entry)

	go func() {
		for app.Stack.TransitionRunning() {
			time.Sleep(time.Millisecond * 10)
		}
		app.ViewLock.Unlock()
	}()
}

// Return the current view
func (app *Application) Top() View {
	return app.Views[len(app.Views)-1]
}

func (app *Application) ReplaceTop(view View) {
	if !app.ViewLock.TryLock() {
		return
	}

	// Return the current view
	current := app.Top()

	// Attempt to close the current view
	if err := current.Close(app); err != nil {
		// There was a problem, so leave the view open
		app.Logger.Error(err.Error())
		app.ViewLock.Unlock()
		return
	}

	// Add the view to the GtkStack
	app.Stack.AddNamed(view, view.Name())

	// Add the view to our view slice
	app.Views = append(app.Views[:len(app.Views)-1], view)

	// Transition to the new view
	app.Stack.SetVisibleChildFull(view.Name(), gtk.StackTransitionTypeSlideLeft)

	// Delete the old view after the transition is complete
	var signalHandle glib.SignalHandle
	pSignalHandle := &signalHandle
	lock := sync.Mutex{}

	lock.Lock()
	defer lock.Unlock()

	*pSignalHandle = app.Stack.Connect("notify::transition-running", func() {
		lock.Lock()
		defer lock.Unlock()
		app.Stack.Remove(current)
		app.Stack.HandlerDisconnect(*pSignalHandle)
		app.ViewLock.Unlock()
	})

	app.updateViewTitle()
	app.Prompt.SetText("VM Manager>")

	// Notify the view of the new focus
	if err := view.Enter(app); err != nil {
		app.Logger.Error(err.Error())
	}

	// Reset the input box
	app.Entry.SetText("")
	app.Window.SetFocus(app.Entry)
}

func (app *Application) PopNoTransition() {

	if !app.ViewLock.TryLock() {
		return
	}

	// Return the current view
	current := app.Top()

	defer app.ViewLock.Unlock()

	// Attempt to close the current view
	if err := current.Close(app); err != nil {
		// There was a problem, so leave the view open
		app.Logger.Error(err.Error())
		return
	}

	// No more views to display, so we exit
	if len(app.Views) == 1 {
		app.Quit()
		return
	}

	// Remove the top view
	app.Views = app.Views[:len(app.Views)-1]

	// Transition to the new view
	newCurrent := app.Top()
	app.Stack.SetVisibleChildFull(newCurrent.Name(), gtk.StackTransitionTypeNone)
	app.Stack.Remove(current)

	app.updateViewTitle()
	app.Prompt.SetText("VM Manager>")

	if err := newCurrent.Enter(app); err != nil {
		app.Logger.Error(err.Error())
	}

	// Reset the input box
	app.Entry.SetText("")
	app.Window.SetFocus(app.Entry)
}

// Remove the current view and transition to the previous
func (app *Application) Pop() {
	if !app.ViewLock.TryLock() {
		return
	}

	// Return the current view
	current := app.Top()

	// Attempt to close the current view
	if err := current.Close(app); err != nil {
		// There was a problem, so leave the view open
		app.Logger.Error(err.Error())
		app.ViewLock.Unlock()
		return
	}

	// No more views to display, so we exit
	if len(app.Views) == 1 {
		app.Quit()
		app.ViewLock.Unlock()
		return
	}

	// Remove the top view
	app.Views = app.Views[:len(app.Views)-1]

	// Transition to the new view
	newCurrent := app.Top()

	transitionType := gtk.StackTransitionTypeSlideRight
	if current == app.LogView {
		transitionType = gtk.StackTransitionTypeSlideDown
	}
	app.Stack.SetVisibleChildFull(newCurrent.Name(), transitionType)

	// Delete the old view after the transition is complete
	var signalHandle glib.SignalHandle
	pSignalHandle := &signalHandle
	lock := sync.Mutex{}

	lock.Lock()
	defer lock.Unlock()

	*pSignalHandle = app.Stack.Connect("notify::transition-running", func() {
		lock.Lock()
		defer lock.Unlock()
		app.Stack.Remove(current)
		app.Stack.HandlerDisconnect(*pSignalHandle)
		app.ViewLock.Unlock()
	})

	app.updateViewTitle()
	app.Prompt.SetText("VM Manager>")

	if err := newCurrent.Enter(app); err != nil {
		app.Logger.Error(err.Error())
	}

	// Reset the input box
	app.Entry.SetText("")
	app.Window.SetFocus(app.Entry)
}
