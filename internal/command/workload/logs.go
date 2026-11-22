package workload

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/workload"
)

const (
	// followEvery is how often a log followed is read again: its node ships
	// what was written every two seconds.
	followEvery = time.Second

	// drainsAtEnd is how many times a log is read again once what writes it
	// has ended, for what it wrote last, which its node ships a beat later.
	drainsAtEnd = 2
)

// logFlags are what docker's logs takes.
type logFlags struct {
	follow     bool
	tail       string
	since      string
	until      string
	timestamps bool
}

func (f *logFlags) define(flags *console.FlagSet) {
	console.Var(flags, &f.follow, console.Long("follow"), "follows what is written, until interrupted.", console.Short("f"))
	console.Var(flags, &f.tail, console.Long("tail"), "how many lines to show from the end, or all.", console.Short("n"), console.Default("all"))
	console.Var(flags, &f.since, console.Long("since"), "shows what was written since a time (2026-11-22T10:00:00Z) or a duration ago (42m).")
	console.Var(flags, &f.until, console.Long("until"), "shows what was written until a time or a duration ago.")
	console.Var(flags, &f.timestamps, console.Long("timestamps"), "shows when each line was written.", console.Short("t"))
}

// query is what of a log the flags ask for, and whether the lines it reads
// at first are only to follow from, as --tail 0 asks, rather than shown.
func (f *logFlags) query(flags *console.FlagSet, now time.Time) (workload.LogsQuery, bool, error) {
	since, err := momentOf(f.since, now)
	if err != nil {
		return workload.LogsQuery{}, false, usageError(flags, "%s", err)
	}

	until, err := momentOf(f.until, now)
	if err != nil {
		return workload.LogsQuery{}, false, usageError(flags, "%s", err)
	}

	query := workload.LogsQuery{From: since, To: until}

	if f.tail == "all" || len(f.tail) == 0 {
		return query, false, nil
	}

	tail, err := strconv.ParseUint(f.tail, 10, 32)
	if err != nil {
		return workload.LogsQuery{}, false, usageError(flags, "%q is not a number of lines: give one, or all", f.tail)
	}

	if tail == 0 {
		query.Tail = 1

		return query, true, nil
	}

	query.Tail = uint(tail)

	return query, false, nil
}

// logFollower writes what a log says, each line once, and follows it.
type logFollower struct {
	// read reads the log as query narrows it.
	read func(ctx context.Context, query workload.LogsQuery) (workload.Logs, error)

	// ended reports whether what writes the log will write no more, which a
	// follow ends at; a follow without one goes on until it is interrupted.
	ended func(ctx context.Context) (bool, error)

	// write writes one line.
	write func(line workload.LogLine) error

	every time.Duration

	// last is the moment of the last line taken, and seen how many lines
	// of that moment were taken: the next read starts from it.
	last time.Time
	seen int
}

// run writes what query reads, but for what hidden says is only to follow
// from, and then, when follow says so, what is written after it.
func (f *logFollower) run(ctx context.Context, query workload.LogsQuery, hidden bool, follow bool) error {
	f.last, f.seen = query.From, 0

	logs, err := f.read(ctx, query)
	if err != nil {
		return err
	}

	if err := f.take(logs.Items, !hidden); err != nil {
		return err
	}

	if !follow {
		return nil
	}

	ended, drains := false, 0

	for {
		if err := wait(ctx, f.every); err != nil {
			return err
		}

		if !ended && f.ended != nil {
			if ended, err = f.ended(ctx); err != nil {
				return err
			}
		}

		logs, err := f.read(ctx, workload.LogsQuery{From: f.last, To: query.To})
		if err != nil {
			return err
		}

		if err := f.take(logs.Items, true); err != nil {
			return err
		}

		if ended {
			if drains++; drains > drainsAtEnd {
				return nil
			}
		}

		if !query.To.IsZero() && time.Now().After(query.To) {
			return nil
		}
	}
}

// take writes the lines it has not taken yet when show says so, and keeps
// where they end.
func (f *logFollower) take(lines []workload.LogLine, show bool) error {
	skipped := 0

	for _, line := range lines {
		switch {
		case line.At.Before(f.last):
			continue
		case line.At.Equal(f.last) && skipped < f.seen:
			skipped++

			continue
		}

		if show {
			if err := f.write(line); err != nil {
				return err
			}
		}

		if line.At.After(f.last) {
			f.last, f.seen = line.At, 1
		} else {
			f.seen++
		}
	}

	return nil
}

// wait waits for every, or until ctx is done.
func wait(ctx context.Context, every time.Duration) error {
	timer := time.NewTimer(every)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// lineWriter writes the lines of a log, each on out but a container's
// stderr, which is written on errOut as docker writes it, and with when it
// was written first when timestamps says so.
func lineWriter(out, errOut io.Writer, timestamps bool) func(line workload.LogLine) error {
	return func(line workload.LogLine) error {
		to := out
		if line.Stream == workload.Stderr {
			to = errOut
		}

		if timestamps {
			_, err := fmt.Fprintf(to, "%s %s\n", line.At.Format(time.RFC3339Nano), line.Line)

			return err
		}

		_, err := fmt.Fprintln(to, line.Line)

		return err
	}
}
