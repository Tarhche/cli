// Package units reads sizes as docker's flags take them, 512m or 4g, and
// writes them back the same way: memory and disk are bytes everywhere else.
package units

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// ErrInvalidSize is a size that cannot be read.
var ErrInvalidSize = errors.New("invalid size")

// multiples are the units a size is given in, each 1024 times the one
// before, as docker reads them.
var multiples = map[string]float64{
	"":  1,
	"k": 1 << 10,
	"m": 1 << 20,
	"g": 1 << 30,
	"t": 1 << 40,
	"p": 1 << 50,
}

// written are the units a size is written in, largest first.
var written = []struct {
	suffix string
	size   uint64
}{
	{"p", 1 << 50},
	{"t", 1 << 40},
	{"g", 1 << 30},
	{"m", 1 << 20},
	{"k", 1 << 10},
}

// sizePattern is a number and the unit it is in: k, m, g, t or p, with an i
// and a b after it or not, or b alone, whatever their case.
var sizePattern = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(?:([kmgtp])(?:i?b)?|b)?$`)

// ParseSize is the size s says, in bytes: a number, which may have a
// fraction, and a unit, each 1024 times the one before (512, 512b, 512k,
// 512m, 512MiB, 4g, 4GB, 1.5t). A size that is not a whole number of bytes
// is rounded up to the next one.
func ParseSize(s string) (uint64, error) {
	matched := sizePattern.FindStringSubmatch(strings.ToLower(strings.TrimSpace(s)))
	if matched == nil {
		return 0, fmt.Errorf("%w: %q", ErrInvalidSize, s)
	}

	number, err := strconv.ParseFloat(matched[1], 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrInvalidSize, s)
	}

	size := math.Ceil(number * multiples[matched[2]])
	if size >= math.MaxUint64 {
		return 0, fmt.Errorf("%w: %q is too large", ErrInvalidSize, s)
	}

	return uint64(size), nil
}

// FormatSize is size as ParseSize reads it, in the largest unit it is a
// whole number of: 512m, 4g, 1536k, 100.
func FormatSize(size uint64) string {
	for _, unit := range written {
		if size >= unit.size && size%unit.size == 0 {
			return strconv.FormatUint(size/unit.size, 10) + unit.suffix
		}
	}

	return strconv.FormatUint(size, 10)
}

// Size is a size in bytes that a flag is given as docker's are, as
// --memory 512m. Its zero is no size given.
type Size uint64

// Set reads value into the size.
func (s *Size) Set(value string) error {
	size, err := ParseSize(value)
	if err != nil {
		return err
	}

	*s = Size(size)

	return nil
}

// String is the size as it is given.
func (s Size) String() string {
	return FormatSize(uint64(s))
}

// Type names what the flag takes in its help.
func (s *Size) Type() string {
	return "bytes"
}

// Zero is the size given as nothing, which help does not mention.
func (s *Size) Zero() string {
	return "0"
}
