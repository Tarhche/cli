package units

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSize(t *testing.T) {
	t.Parallel()

	cases := map[string]uint64{
		"0":         0,
		"512":       512,
		"512b":      512,
		"512B":      512,
		"1k":        1 << 10,
		"1kb":       1 << 10,
		"1KiB":      1 << 10,
		"512m":      512 << 20,
		"512MiB":    512 << 20,
		"512 MB":    512 << 20,
		"4g":        4 << 30,
		"4GB":       4 << 30,
		" 4gib ":    4 << 30,
		"1.5g":      3 << 29,
		"2t":        2 << 40,
		"1p":        1 << 50,
		"0.5":       1,
		"1.0000001": 2,
	}

	for given, want := range cases {
		t.Run(given, func(t *testing.T) {
			t.Parallel()

			got, err := ParseSize(given)

			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}

	for _, given := range []string{"", "m", "-1m", "1x", "1.m", "1ib", "1 g b", "1e3", "99999999999999999999999p"} {
		t.Run("refuses "+given, func(t *testing.T) {
			t.Parallel()

			_, err := ParseSize(given)

			assert.ErrorIs(t, err, ErrInvalidSize)
		})
	}
}

func TestFormatSize(t *testing.T) {
	t.Parallel()

	cases := map[uint64]string{
		0:             "0",
		100:           "100",
		1 << 10:       "1k",
		1536 << 10:    "1536k",
		512 << 20:     "512m",
		4 << 30:       "4g",
		3 << 40:       "3t",
		1 << 50:       "1p",
		(1 << 30) + 1: "1073741825",
	}

	for size, want := range cases {
		t.Run(want, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, want, FormatSize(size))

			read, err := ParseSize(want)
			require.NoError(t, err)
			assert.Equal(t, size, read, "what is written is read back as it was")
		})
	}
}

func TestSize(t *testing.T) {
	t.Parallel()

	t.Run("a size is set from what docker's flags take", func(t *testing.T) {
		t.Parallel()

		var s Size
		require.NoError(t, s.Set("512m"))

		assert.Equal(t, Size(512<<20), s)
		assert.Equal(t, "512m", s.String())
		assert.Equal(t, "bytes", s.Type())
		assert.Equal(t, "0", s.Zero())
	})

	t.Run("a size that cannot be read leaves it as it was", func(t *testing.T) {
		t.Parallel()

		s := Size(4 << 30)

		assert.ErrorIs(t, s.Set("lots"), ErrInvalidSize)
		assert.Equal(t, Size(4<<30), s)
	})
}
