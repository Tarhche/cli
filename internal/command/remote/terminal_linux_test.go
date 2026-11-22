package remote

import (
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// pseudoTerminal is the terminal end of a new pseudo-terminal, rows by cols.
func pseudoTerminal(t *testing.T, rows, cols uint16) *os.File {
	t.Helper()

	controller, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("no pseudo-terminal can be opened here: %s", err)
	}

	t.Cleanup(func() { _ = controller.Close() })

	require.NoError(t, unix.IoctlSetPointerInt(int(controller.Fd()), unix.TIOCSPTLCK, 0))

	number, err := unix.IoctlGetInt(int(controller.Fd()), unix.TIOCGPTN)
	require.NoError(t, err)

	terminal, err := os.OpenFile("/dev/pts/"+strconv.Itoa(number), os.O_RDWR|unix.O_NOCTTY, 0)
	require.NoError(t, err)

	t.Cleanup(func() { _ = terminal.Close() })

	require.NoError(t, unix.IoctlSetWinsize(int(terminal.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: rows, Col: cols}))

	return terminal
}

func TestLocalTerminalOfAPseudoTerminal(t *testing.T) {
	t.Parallel()

	terminal := pseudoTerminal(t, 40, 120)

	assert.True(t, localTerminal{}.IsTerminal(terminal))

	rows, cols, err := localTerminal{}.Size(terminal)
	require.NoError(t, err)
	assert.Equal(t, uint(40), rows)
	assert.Equal(t, uint(120), cols)

	before, err := term.GetState(int(terminal.Fd()))
	require.NoError(t, err)

	restore, err := localTerminal{}.MakeRaw(terminal, terminal)
	require.NoError(t, err)

	raw, err := term.GetState(int(terminal.Fd()))
	require.NoError(t, err)
	assert.NotEqual(t, before, raw)

	require.NoError(t, restore())

	after, err := term.GetState(int(terminal.Fd()))
	require.NoError(t, err)
	assert.Equal(t, before, after)
}
