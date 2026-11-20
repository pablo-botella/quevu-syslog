// Package qsyslogread reads the log of the system and writes what it
// finds into a quevu log: the system log seen as a queue, read from its
// far end.
//
// With qsyslogwrite on the other side it makes the system log a way
// between processes: one writes its lines there, and another, that may
// not even be running at the time, picks them up later where it left
// off. Alone, it brings into quevu what any program wrote to the system
// log.
//
// On Windows the system log is the Event Log. On other systems there is
// no reader yet and NewSource answers ErrUnsupported.
package qsyslogread

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	quevu "github.com/pablo-botella/quevu-kernel"
)

// ErrUnsupported is what NewSource answers on a system whose log cannot
// be read from here yet.
var ErrUnsupported = errors.New("qsyslogread: the log of this system cannot be read yet")

// Where to begin when nothing says where the reading was left.
const (
	// Newest begins with what is written from now on.
	Newest = iota
	// Oldest begins with the oldest entry the log still holds.
	Oldest
)

// Config says what to read.
type Config struct {
	// Log is the log to read. Empty: "Application", where programs
	// write.
	Log string
	// Source keeps only the entries written under this name (the name a
	// qsyslogwrite device was opened with). Empty: every entry.
	Source string
	// State is a file where the place of the last entry delivered is
	// kept, so the next run goes on from there and nothing written
	// while this program was stopped is missed. Empty: every run begins
	// as Begin says.
	State string
	// Begin says where to begin when there is no State, or it is the
	// first run: Newest (the default) or Oldest.
	Begin int
	// Poll is how often the log is looked at when there is nothing new.
	// Zero: a second.
	Poll time.Duration
}

// Entry is one entry of the system log.
type entry struct {
	record uint32
	at     time.Time
	source string
	text   string
}

// reader is the log of one system.
type reader interface {
	// bounds returns the oldest record the log holds and the one the
	// next entry will get.
	bounds() (oldest, next uint32, err error)
	// read returns the entries from record on, at most a bufferful.
	read(record uint32) ([]entry, error)
	close() error
}

// Source is an input: a unit that, while running, reads the system log
// and appends every entry to a log of quevu, with the time the entry was
// written.
type Source struct {
	cfg  Config
	into quevu.TimedIntake
	unit quevu.Unit
	read atomic.Uint64
	lost atomic.Uint64

	open func(log string) (reader, error)
}

// NewSource returns the input. Nothing is opened until it is started.
func NewSource(cfg Config, into quevu.TimedIntake) (*Source, error) {
	if into == nil {
		return nil, errors.New("qsyslogread: nowhere to write")
	}
	if cfg.Log == "" {
		cfg.Log = "Application"
	}
	if cfg.Poll <= 0 {
		cfg.Poll = time.Second
	}
	if cfg.Begin != Newest && cfg.Begin != Oldest {
		return nil, fmt.Errorf("qsyslogread: unknown place to begin %d", cfg.Begin)
	}
	if !supported {
		return nil, ErrUnsupported
	}
	s := &Source{cfg: cfg, into: into, open: openLog}
	s.unit = goUnit(s.run)
	return s, nil
}

// Start begins reading.
func (s *Source) Start(ctx context.Context) error { return s.unit.Start(ctx) }

// Stop ends the reading after the entry in hand has been delivered and
// its place noted.
func (s *Source) Stop(ctx context.Context) error { return s.unit.Stop(ctx) }

// Done tells when the reading ends by itself, with the error that ended
// it.
func (s *Source) Done() <-chan error {
	if e, ok := s.unit.(quevu.Exiter); ok {
		return e.Done()
	}
	return nil
}

// Read returns how many entries have been delivered.
func (s *Source) Read() uint64 { return s.read.Load() }

// Lost returns how many entries the log had already dropped when their
// turn came: the system log keeps a limited amount, and what is not read
// in time is overwritten.
func (s *Source) Lost() uint64 { return s.lost.Load() }

// place reads where the last run left off: the record that follows the
// last one delivered. ok is false when nothing says.
func (s *Source) place() (next uint32, ok bool) {
	if s.cfg.State == "" {
		return 0, false
	}
	b, err := os.ReadFile(s.cfg.State)
	if err != nil {
		return 0, false
	}
	n, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(n), true
}

// keep notes the place: written apart and renamed over, so a death in
// the middle leaves the old place and never half of a new one.
func (s *Source) keep(next uint32) error {
	if s.cfg.State == "" {
		return nil
	}
	tmp := s.cfg.State + ".tmp"
	if err := os.WriteFile(tmp, []byte(strconv.FormatUint(uint64(next), 10)+"\n"), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.cfg.State)
}

func (s *Source) run(ctx context.Context) error {
	r, err := s.open(s.cfg.Log)
	if err != nil {
		return err
	}
	defer r.close()
	oldest, end, err := r.bounds()
	if err != nil {
		return err
	}
	next, ok := s.place()
	switch {
	case ok:
	case s.cfg.Begin == Oldest:
		next = oldest
	default:
		next = end
		// The place is noted at once: what is written from this moment
		// is ours even if we are stopped before reading any of it.
		if err := s.keep(next); err != nil {
			return err
		}
	}
	for {
		oldest, end, err = r.bounds()
		if err != nil {
			return err
		}
		if next < oldest || next > end {
			// The log moved under us: it was cleared, or it dropped
			// what we had not read yet.
			if next < oldest {
				s.lost.Add(uint64(oldest - next))
			}
			next = oldest
		}
		if next == end {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(s.cfg.Poll):
			}
			continue
		}
		entries, err := r.read(next)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			// Told there is something, given nothing: wait and look again.
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(s.cfg.Poll):
			}
			continue
		}
		for _, e := range entries {
			if s.cfg.Source == "" || strings.EqualFold(e.source, s.cfg.Source) {
				if _, err := s.into.AppendAt([]byte(e.text), e.at); err != nil {
					// Not delivered: its place is not noted, and the
					// next run begins with it again.
					return err
				}
				s.read.Add(1)
			}
			next = e.record + 1
			if err := s.keep(next); err != nil {
				return err
			}
			if ctx.Err() != nil {
				return nil
			}
		}
	}
}

// goUnit makes the unit that runs the reading.
func goUnit(run func(ctx context.Context) error) quevu.Unit { return quevu.Go(run) }
