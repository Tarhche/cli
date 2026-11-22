// Package desktop is the desktop app the CLI opens with --desktop: the same
// estate, contexts and sessions as the commands, through a window.
//
// The app is Wails. What it binds to its frontend (frontend/, embedded from
// frontend/dist) are plain services, each reaching the context the app
// talks to through a Workspace: Session (contexts, signing in and out),
// Resources (whatever package resource describes), Workloads (one's own
// workload), Terminals, Forwards and Files, which reach into workloads
// through what a Reach gives them, and Updates, which upgrades the app to a
// newer release of tarhche.
package desktop

import (
	"context"
	_ "embed"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/Tarhche/cli/frontend"
	"github.com/Tarhche/cli/internal/auth"
	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/credentials"
	"github.com/Tarhche/cli/internal/resource"
	"github.com/Tarhche/cli/internal/upgrade"
	"github.com/Tarhche/cli/internal/version"
)

const (
	// name is what the app and its window are called.
	name = "Tarhche"

	windowWidth  = 1200
	windowHeight = 800
)

// The app's icons: its own, and the tray's, a glyph that is black, which
// macOS tints as it tints every template icon, and one that is white for a
// tray that is dark elsewhere.
var (
	//go:embed icons/app.png
	appIcon []byte

	//go:embed icons/tray.png
	trayIcon []byte

	//go:embed icons/tray-dark.png
	trayDarkIcon []byte
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
		Icon:                        appIcon,
		Assets:                      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac:                         application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		MarshalError:                MarshalError,
		Logger:                      slog.New(slog.DiscardHandler),
		DisableDefaultSignalHandler: true,
	})

	manager := auth.New(cfg, store, auth.WithInteractive(false), auth.WithPrompt(io.Discard), auth.WithBrowser(app.Browser.OpenURL))
	workspace := NewWorkspace(contextName, NewOpener(cli, cfg, manager, contextName))
	streams := NewStreams()
	reach := Reach{Terminals: streams, Forwards: streams, Files: streams}

	upgrader := upgrade.New(upgrade.WithUserAgent(version.UserAgent()))

	for _, service := range Services(ctx, workspace, manager, reach, app.Event, dialogs{manager: app.Dialog}, upgrader, app.Quit) {
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

	if runtime.GOOS == "darwin" {
		app.Menu.Set(newMenuBar(app, window))
	}

	stop := context.AfterFunc(ctx, app.Quit)
	defer stop()

	return app.Run()
}

// Services are what the app binds to its frontend, all of them over
// workspace: terminals and forwards last at most as long as ctx, and are
// heard through emitter; paths on the host are asked for through chooser;
// the app is upgraded by upgrader, and quit with quit to restart into what
// it installed.
func Services(ctx context.Context, workspace *Workspace, authenticator Authenticator, reach Reach, emitter Emitter, chooser Chooser, upgrader Upgrader, quit func()) []application.Service {
	return []application.Service{
		application.NewService(NewSession(workspace, authenticator)),
		application.NewService(NewResources(workspace, resource.All)),
		application.NewService(NewWorkloads(workspace)),
		application.NewService(NewTerminals(ctx, workspace, reach.Terminals, emitter)),
		application.NewService(NewForwards(ctx, workspace, reach.Forwards, emitter)),
		application.NewService(NewFiles(workspace, reach.Files, chooser)),
		application.NewService(NewUpdates(upgrader, emitter, quit)),
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
// window forward, looks for a newer release and quits it.
func newTray(app *application.App, window *application.WebviewWindow) {
	menu := app.NewMenu()
	menu.Add("Open").OnClick(func(*application.Context) {
		bringForward(window)
	})
	menu.Add(checkForUpdatesLabel).OnClick(func(*application.Context) {
		checkForUpdates(window, app.Event)
	})
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(*application.Context) {
		app.Quit()
	})

	tray := app.SystemTray.New()
	tray.SetTooltip(name)
	tray.SetMenu(menu)

	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(trayIcon)

		return
	}

	tray.SetIcon(trayIcon)
	tray.SetDarkModeIcon(trayDarkIcon)
}

// newMenuBar is the macOS menu bar: the one Wails gives every app, with
// Check for Updates… under About in the app's menu, and no Help menu, whose
// one item would take the window away from the app to Wails' website.
func newMenuBar(app *application.App, window *application.WebviewWindow) *application.Menu {
	bar := app.NewMenu()

	appMenu := bar.AddSubmenu(name)
	appMenu.AddRole(application.About)
	appMenu.Add(checkForUpdatesLabel).OnClick(func(*application.Context) {
		checkForUpdates(window, app.Event)
	})
	appMenu.AddSeparator()
	appMenu.AddRole(application.ServicesMenu)
	appMenu.AddSeparator()
	appMenu.AddRole(application.Hide)
	appMenu.AddRole(application.HideOthers)
	appMenu.AddRole(application.UnHide)
	appMenu.AddSeparator()
	appMenu.AddRole(application.Quit)

	bar.AddRole(application.FileMenu)
	bar.AddRole(application.EditMenu)
	bar.AddRole(application.ViewMenu)
	bar.AddRole(application.WindowMenu)

	return bar
}

// checkForUpdatesLabel is what the menu item that looks for a newer
// release says.
const checkForUpdatesLabel = "Check for Updates…"

// foreground is a window that can be brought in front of every other.
type foreground interface {
	UnMinimise()
	Show() application.Window
	Focus()
}

// bringForward shows window in front of every other.
func bringForward(window foreground) {
	window.UnMinimise()
	window.Show()
	window.Focus()
}

// checkForUpdates brings window forward and asks its frontend, through
// emitter, to look for a newer release of tarhche.
func checkForUpdates(window foreground, emitter Emitter) {
	bringForward(window)
	emitter.Emit(EventCheckForUpdates)
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
