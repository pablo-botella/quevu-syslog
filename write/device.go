// Package qsyslogwrite is a persistent writer of quevu that writes to the
// log of the operating system: the Event Log on Windows, and what each
// other system has in its place.
//
// It is a writer like the text file or the SQLite ones: a log sends it
// batches through a queue of its own, it stores them, and it gives
// nothing back. Reading the system log is somebody else's work.
//
// Each chunk becomes one entry. The system log is not the place for
// everything an application says: it is usually attached to the log of a
// piece whose output somebody must see, or behind a filter (see
// Options.Level and Options.Only).
package qsyslogwrite

import (
	"errors"

	"github.com/pablo-botella/quevu"
)

// Level is how much an entry matters, in the three grades every system
// log has.
type Level int

const (
	Info Level = iota
	Warning
	Error
)

// ErrUnsupported is returned by Open where this module has no
// implementation for the system log.
var ErrUnsupported = errors.New("qsyslogwrite: the system log of this platform is not implemented")

// maxEntry bounds the text of one entry. The Windows Event Log takes
// strings up to about 31,839 characters; longer chunks are cut, with a
// mark saying so.
const maxEntry = 30_000

// Options tunes a Device.
type Options struct {
	// Level decides the grade of each chunk. Nil means Info for all: a
	// chunk carries no grade of its own, so telling an error from a
	// remark is the application's knowledge.
	Level func(quevu.Chunk) Level
	// Only, if set, decides which chunks are written at all. The rest
	// are taken and dropped: to the log that feeds this device they
	// count as consumed.
	Only func(quevu.Chunk) bool
}

// writer is what each platform provides.
type writer interface {
	write(level Level, text string) error
	close() error
}

// Device writes the chunks of a log to the system log.
type Device struct {
	fails quevu.Failures // the body of Run, and what it could not write

	w   writer
	opt Options
	buf []byte
}

// Open returns a device that writes under the given source name, which
// is how the entries are attributed in the system log: usually the name
// of the application.
//
// On Windows the entries are written whether or not the source is
// registered (see Register); registration only changes how the Event
// Viewer displays them.
func Open(source string, opt Options) (*Device, error) {
	if source == "" {
		return nil, errors.New("qsyslogwrite: empty source name")
	}
	w, err := open(source)
	if err != nil {
		return nil, err
	}
	return &Device{w: w, opt: opt}, nil
}

// Write stores a batch: one entry per chunk, in order. The text is the
// chunk as a console shows it, without the color sequences an
// application may have written for a terminal.
func (d *Device) Write(batch []quevu.Chunk) error {
	var first error
	for _, c := range batch {
		if d.opt.Only != nil && !d.opt.Only(c) {
			continue
		}
		level := Info
		if d.opt.Level != nil {
			level = d.opt.Level(c)
		}
		d.buf = quevu.PlainText(d.buf[:0], c.Data)
		text := string(d.buf)
		if len(text) > maxEntry {
			text = text[:maxEntry] + " [...cut]"
		}
		// One entry that fails does not hold back the ones behind it.
		if err := d.w.write(level, text); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Close releases the handle to the system log.
func (d *Device) Close() error { return d.w.close() }

var _ quevu.Writer = (*Device)(nil)

// Run receives from the channel the hub gave this writer until it is
// closed, writing what arrives in batches, and closes the writer at the
// end. It is what makes it a quevu.Writer.
func (d *Device) Run(in <-chan quevu.Chunk) error { return d.fails.Drain(in, 0, d) }

// Failed returns the chunks lost so far in failed batches, and the last
// error.
func (d *Device) Failed() (uint64, error) { return d.fails.Failed() }
