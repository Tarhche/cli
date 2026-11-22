// Package desktop is the desktop app the CLI opens with --desktop: the same
// estate, contexts and sessions as the commands, through a window.
//
// The app is Wails. What it binds to its frontend (frontend/, embedded from
// frontend/dist) are plain services, each reaching the context the app
// talks to through a Workspace: Session (contexts, signing in and out),
// Resources (whatever package resource describes), Workloads (one's own
// workload), and Terminals, Forwards and Files, which reach into workloads
// through what a Reach gives them.
package desktop

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/icons"

	"github.com/Tarhche/cli/frontend"
	"github.com/Tarhche/cli/internal/auth"
	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/credentials"
	"github.com/Tarhche/cli/internal/resource"
)

const (
	// name is what the app and its window are called.
	name = "Tarhche"

	windowWidth  = 1200
	windowHeight = 800
)

// ErrNoDisplay is a machine the desktop app cannot open a window on.
var ErrNoDisplay = errors.New("the desktop app needs a display to open on: neither DISPLAY nor WAYLAND_DISPLAY is set")

// Reach is how the desktop reaches into workloads: their terminals, their
// ports and their files.
type Reach struct {
	Terminals TerminalOpener
	Forwards  Forwarder
	Files     Copier
}

// Run opens the desktop app on the context cli talks to, and returns once
// it is closed or ctx is done.
func Run(ctx context.Context, cli *command.Cli) error {
	if err := displayable(runtime.GOOS, os.Getenv); err != nil {
		return err
	}

	cfg, err := cli.Config()
	if err != nil {
		return err
	}

	contextName, err := cli.ContextName()
	if err != nil {
		return err
	}

	store, err := credentials.Open(cfg)
	if err != nil {
		return err
	}

	assets, err := fs.Sub(frontend.Assets, "dist")
	if err != nil {
		return err
	}

	app := application.New(application.Options{
		Name:                        name,
		Description:                 "Tarhche on the desktop",
		Assets:                      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac:                         application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		MarshalError:                MarshalError,
		Logger:                      slog.New(slog.DiscardHandler),
		DisableDefaultSignalHandler: true,
	})

	manager := auth.New(cfg, store, auth.WithInteractive(false), auth.WithPrompt(io.Discard), auth.WithBrowser(app.Browser.OpenURL))
	workspace := NewWorkspace(contextName, NewOpener(cli, cfg, manager, contextName))
	reach := Reach{Terminals: Unavailable{}, Forwards: Unavailable{}, Files: Unavailable{}}

	for _, service := range Services(ctx, workspace, manager, reach, app.Event, dialogs{manager: app.Dialog}) {
		app.RegisterService(service)
	}

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      "main",
		Title:     name,
		Width:     windowWidth,
		Height:    windowHeight,
		MinWidth:  360,
		MinHeight: 480,
		URL:       "/",
	})

	newTray(app, window)

	stop := context.AfterFunc(ctx, app.Quit)
	defer stop()

	return app.Run()
}

// Services are what the app binds to its frontend, all of them over
// workspace: terminals and forwards last at most as long as ctx, and are
// heard through emitter; paths on the host are asked for through chooser.
func Services(ctx context.Context, workspace *Workspace, authenticator Authenticator, reach Reach, emitter Emitter, chooser Chooser) []application.Service {
	return []application.Service{
		application.NewService(NewSession(workspace, authenticator)),
		application.NewService(NewResources(workspace, resource.All)),
		application.NewService(NewWorkloads(workspace)),
		application.NewService(NewTerminals(ctx, workspace, reach.Terminals, emitter)),
		application.NewService(NewForwards(ctx, workspace, reach.Forwards, emitter)),
		application.NewService(NewFiles(workspace, reach.Files, chooser)),
	}
}

// displayable says why the app cannot open a window on goos, if it cannot:
// beside macOS and Windows, it needs an X or a Wayland display, which
// getenv says where to find.
func displayable(goos string, getenv func(string) string) error {
	switch {
	case goos == "darwin", goos == "windows":
		return nil
	case len(getenv("DISPLAY")) > 0, len(getenv("WAYLAND_DISPLAY")) > 0:
		return nil
	default:
		return ErrNoDisplay
	}
}

// newTray puts the app in the system tray, with a menu that brings its
// window forward and quits it.
func newTray(app *application.App, window *application.WebviewWindow) {
	menu := app.NewMenu()
	menu.Add("Open").OnClick(func(*application.Context) {
		window.UnMinimise()
		window.Show()
		window.Focus()
	})
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(*application.Context) {
		app.Quit()
	})

	tray := app.SystemTray.New()
	tray.SetTooltip(name)
	tray.SetMenu(menu)

	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(icons.SystrayMacTemplate)

		return
	}

	tray.SetIcon(icons.SystrayLight)
	tray.SetDarkModeIcon(icons.SystrayDark)
}

// dialogs ask for paths on the host in the operating system's own dialogs.
type dialogs struct {
	manager *application.DialogManager
}

// Open asks for a file or a directory to read.
func (d dialogs) Open(title string) (string, error) {
	return d.manager.OpenFile().
		SetTitle(title).
		CanChooseFiles(true).
		CanChooseDirectories(true).
		PromptForSingleSelection()
}

// Save asks where to write what is called filename.
func (d dialogs) Save(title, filename string) (string, error) {
	return d.manager.SaveFile().
		SetMessage(title).
		SetFilename(filename).
		CanCreateDirectories(true).
		PromptForSingleSelection()
}
