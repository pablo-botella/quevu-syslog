package qsyslogwrite

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pablo-botella/quevu"
)

type entry struct {
	level Level
	text  string
}

// recorder stands for the system log.
type recorder struct {
	entries []entry
	fail    map[string]error
	closed  bool
}

func (r *recorder) write(level Level, text string) error {
	if err := r.fail[text]; err != nil {
		return err
	}
	r.entries = append(r.entries, entry{level, text})
	return nil
}

func (r *recorder) close() error { r.closed = true; return nil }

func chunks(texts ...string) []quevu.Chunk {
	out := make([]quevu.Chunk, len(texts))
	for i, t := range texts {
		out[i] = quevu.Chunk{Seq: uint64(i + 1), Time: time.Now(), Data: []byte(t)}
	}
	return out
}

func TestOneEntryPerChunkAsAConsoleShowsIt(t *testing.T) {
	r := &recorder{}
	d := &Device{w: r}
	if err := d.Write(chunks("plain", "\x1b[31mERROR\x1b[0m disk full", "")); err != nil {
		t.Fatal(err)
	}
	want := []entry{{Info, "plain"}, {Info, "ERROR disk full"}, {Info, ""}}
	if len(r.entries) != len(want) {
		t.Fatalf("entries: %+v", r.entries)
	}
	for i := range want {
		if r.entries[i] != want[i] {
			t.Fatalf("entry %d: %+v", i, r.entries[i])
		}
	}
	d.Close()
	if !r.closed {
		t.Fatal("not closed")
	}
}

func TestTheApplicationSaysWhatMattersAndWhatGoesIn(t *testing.T) {
	r := &recorder{}
	d := &Device{w: r, opt: Options{
		Level: func(c quevu.Chunk) Level {
			switch {
			case strings.Contains(string(c.Data), "ERROR"):
				return Error
			case strings.Contains(string(c.Data), "WARN"):
				return Warning
			}
			return Info
		},
		// The system log is not the place for everything.
		Only: func(c quevu.Chunk) bool { return !strings.HasPrefix(string(c.Data), "debug") },
	}}
	d.Write(chunks("debug: tick", "started", "WARN slow", "debug: tock", "ERROR down"))
	want := []entry{{Info, "started"}, {Warning, "WARN slow"}, {Error, "ERROR down"}}
	if len(r.entries) != 3 || r.entries[0] != want[0] || r.entries[1] != want[1] || r.entries[2] != want[2] {
		t.Fatalf("entries: %+v", r.entries)
	}
}

func TestAnEntryThatFailsDoesNotHoldBackTheRest(t *testing.T) {
	boom := errors.New("log full")
	r := &recorder{fail: map[string]error{"b": boom}}
	d := &Device{w: r}
	if err := d.Write(chunks("a", "b", "c")); !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	if len(r.entries) != 2 || r.entries[0].text != "a" || r.entries[1].text != "c" {
		t.Fatalf("entries: %+v", r.entries)
	}
}

func TestAHugeChunkIsCut(t *testing.T) {
	r := &recorder{}
	d := &Device{w: r}
	d.Write(chunks(strings.Repeat("x", 100_000)))
	got := r.entries[0].text
	if len(got) > maxEntry+20 || !strings.HasSuffix(got, "[...cut]") {
		t.Fatalf("length %d, tail %q", len(got), got[len(got)-10:])
	}
}

func TestNoSourceNoDevice(t *testing.T) {
	if _, err := Open("", Options{}); err == nil {
		t.Fatal("empty source accepted")
	}
}
