package desktop

import "github.com/wailsapp/wails/v3/pkg/application"

// The events the desktop tells its frontend of, beside what its services
// answer.
const (
	// EventTerminalOutput carries what a terminal said: a TerminalOutput.
	EventTerminalOutput = "terminal:output"

	// EventTerminalExit says a terminal ended: a TerminalExit.
	EventTerminalExit = "terminal:exit"

	// EventForwardStopped says a port forward stopped by itself: a
	// ForwardStopped.
	EventForwardStopped = "forward:stopped"

	// EventCheckForUpdates asks the frontend to look for a newer release of
	// tarhche, as the app's menus do: it carries nothing.
	EventCheckForUpdates = "updates:check"

	// EventUpdateProgress says how much of an upgrade's download arrived: an
	// UpdateProgress.
	EventUpdateProgress = "updates:progress"
)

// TerminalOutput is what a terminal said, in the order it said it.
type TerminalOutput struct {
	// ID is the terminal's, as it was opened.
	ID string `json:"id"`

	// Data are the terminal's bytes, which carry what it says in UTF-8 and
	// may end in the middle of a character.
	Data []byte `json:"data"`
}

// TerminalExit is a terminal that ended: closed, or what ran in it done.
type TerminalExit struct {
	ID string `json:"id"`

	// Code is the status what ran in it exited with, zero when it ended
	// well or was closed.
	Code int `json:"code"`

	// Error says why it ended, when it did not end well.
	Error string `json:"error,omitempty"`
}

// ForwardStopped is a port forward that stopped without being asked to.
type ForwardStopped struct {
	ID    string `json:"id"`
	Error string `json:"error,omitempty"`
}

// UpdateProgress is how much of the download of a release an upgrade
// installs has arrived.
type UpdateProgress struct {
	// Version is the release's.
	Version string `json:"version"`

	// Done is how many bytes of it arrived, of Total, which is 0 while how
	// many there are is not known.
	Done  int64 `json:"done"`
	Total int64 `json:"total"`
}

// Emitter tells the frontend that something happened: the app's event
// manager.
type Emitter interface {
	// Emit tells the frontend of the event called name, carrying data.
	Emit(name string, data ...any) bool
}

func init() {
	application.RegisterEvent[TerminalOutput](EventTerminalOutput)
	application.RegisterEvent[TerminalExit](EventTerminalExit)
	application.RegisterEvent[ForwardStopped](EventForwardStopped)
	application.RegisterEvent[application.Void](EventCheckForUpdates)
	application.RegisterEvent[UpdateProgress](EventUpdateProgress)
}
