package qsyslogread

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

// intake is a log that remembers what it was given, and can refuse.
type intake struct {
	mu     sync.Mutex
	lines  []string
	times  []time.Time
	refuse error
}

func (i *intake) Write(p []byte) (int, error) { return len(p), nil }
func (i *intake) Append(d []byte) error {
	return i.AppendAt(d, time.Now())
}
func (i *intake) AppendAt(d []byte, at time.Time) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.refuse != nil {
		return i.refuse
	}
	i.lines = append(i.lines, string(d))
	i.times = append(i.times, at)
	return nil
}
func (i *intake) got() []string {
	i.mu.Lock()
	defer i.mu.Unlock()
	return append([]string(nil), i.lines...)
}

// fakeLog is a system log in memory: entries numbered from first, of
// which only the newest keep are still held.
type fakeLog struct {
	mu      sync.Mutex
	first   uint32
	entries []entry
}

func (f *fakeLog) add(source, text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = append(f.entries, entry{record: f.first + uint32(len(f.entries)), at: time.Unix(1700000000+int64(len(f.entries)), 0), source: source, text: text})
}

// drop makes the log forget its n oldest entries, as a full log does.
func (f *fakeLog) drop(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = f.entries[n:]
	f.first += uint32(n)
}

func (f *fakeLog) bounds() (uint32, uint32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.first, f.first + uint32(len(f.entries)), nil
}

func (f *fakeLog) read(record uint32) ([]entry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if record < f.first || record >= f.first+uint32(len(f.entries)) {
		return nil, nil
	}
	rest := f.entries[record-f.first:]
	if len(rest) > 3 { // a bufferful
		rest = rest[:3]
	}
	return append([]entry(nil), rest...), nil
}

func (f *fakeLog) close() error { return nil }

func source(t *testing.T, cfg Config, log *fakeLog, in *intake) *Source {
	t.Helper()
	cfg.Poll = 5 * time.Millisecond
	s := &Source{cfg: cfg, into: in, open: func(string) (reader, error) { return log, nil }}
	s.unit = goUnit(s.run)
	return s
}

func wait(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("never happened: %s", what)
}

func TestItReadsWhatIsWrittenFromNowOn(t *testing.T) {
	log := &fakeLog{first: 100}
	log.add("app", "before we began")
	in := &intake{}
	s := source(t, Config{}, log, in)
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	for _, text := range []string{"one", "two", "three", "four", "five"} {
		log.add("app", text)
	}
	wait(t, "five entries read", func() bool { return s.Read() == 5 })
	if err := s.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := in.got(); !reflect.DeepEqual(got, []string{"one", "two", "three", "four", "five"}) {
		t.Fatalf("%q", got)
	}
	// Each entry keeps the time it was written, not the time it was read.
	if !in.times[0].Equal(time.Unix(1700000001, 0)) {
		t.Fatalf("time %v", in.times[0])
	}
}

func TestItCanBeginWithTheOldestAndKeepOnlyOneSource(t *testing.T) {
	log := &fakeLog{first: 7}
	log.add("mine", "a")
	log.add("other", "not mine")
	log.add("MINE", "b")
	in := &intake{}
	s := source(t, Config{Begin: Oldest, Source: "Mine"}, log, in)
	s.Start(context.Background())
	wait(t, "two entries read", func() bool { return s.Read() == 2 })
	s.Stop(context.Background())
	if got := in.got(); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("%q", got)
	}
}

func TestItGoesOnWhereItLeftOff(t *testing.T) {
	state := filepath.Join(t.TempDir(), "place")
	log := &fakeLog{first: 1}
	log.add("app", "old")
	in := &intake{}
	s := source(t, Config{State: state}, log, in)
	s.Start(context.Background())
	// The place is noted from the first moment, before anything is read.
	wait(t, "the place noted", func() bool { _, err := os.Stat(state); return err == nil })
	log.add("app", "one")
	wait(t, "one read", func() bool { return s.Read() == 1 })
	s.Stop(context.Background())

	// Written while nobody was reading: not missed.
	log.add("app", "while stopped 1")
	log.add("app", "while stopped 2")
	s = source(t, Config{State: state}, log, in)
	s.Start(context.Background())
	wait(t, "two more read", func() bool { return s.Read() == 2 })
	log.add("app", "live again")
	wait(t, "a third", func() bool { return s.Read() == 3 })
	s.Stop(context.Background())
	if got := in.got(); !reflect.DeepEqual(got, []string{"one", "while stopped 1", "while stopped 2", "live again"}) {
		t.Fatalf("%q", got)
	}

	// A state that says nothing readable is as no state.
	os.WriteFile(state, []byte("garbage"), 0o644)
	s = source(t, Config{State: state, Begin: Oldest}, log, &intake{})
	s.Start(context.Background())
	wait(t, "everything read again", func() bool { return s.Read() == 5 })
	s.Stop(context.Background())
}

func TestAnEntryNotDeliveredIsReadAgain(t *testing.T) {
	state := filepath.Join(t.TempDir(), "place")
	log := &fakeLog{first: 1}
	in := &intake{}
	s := source(t, Config{State: state, Begin: Oldest}, log, in)
	log.add("app", "one")
	log.add("app", "two")
	s.Start(context.Background())
	wait(t, "two read", func() bool { return s.Read() == 2 })
	in.mu.Lock()
	in.refuse = errors.New("log closed")
	in.mu.Unlock()
	done := s.Done()
	log.add("app", "three")
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("ended with no error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("did not end when the log refused")
	}
	in.mu.Lock()
	in.refuse = nil
	in.mu.Unlock()
	s = source(t, Config{State: state, Begin: Oldest}, log, in)
	s.Start(context.Background())
	wait(t, "the refused one read", func() bool { return s.Read() == 1 })
	s.Stop(context.Background())
	if got := in.got(); !reflect.DeepEqual(got, []string{"one", "two", "three"}) {
		t.Fatalf("%q", got)
	}
}

func TestWhatTheLogDroppedIsCountedNotInvented(t *testing.T) {
	state := filepath.Join(t.TempDir(), "place")
	os.WriteFile(state, []byte("12\n"), 0o644) // we had read up to 11
	log := &fakeLog{first: 10}
	for i := 0; i < 10; i++ { // records 10 to 19
		log.add("app", string(rune('a'+i)))
	}
	log.drop(5) // the log is full: 10 to 14 are gone, three of them unread
	in := &intake{}
	s := source(t, Config{State: state}, log, in)
	s.Start(context.Background())
	wait(t, "the five that are left", func() bool { return s.Read() == 5 })
	s.Stop(context.Background())
	if s.Lost() != 3 {
		t.Fatalf("%d lost, want 3", s.Lost())
	}
	if got := in.got(); !reflect.DeepEqual(got, []string{"f", "g", "h", "i", "j"}) {
		t.Fatalf("%q", got)
	}
	// A log that was cleared begins again: its numbers are behind ours.
	log.mu.Lock()
	log.entries, log.first = nil, 1
	log.mu.Unlock()
	log.add("app", "after clearing")
	s = source(t, Config{State: state}, log, in)
	s.Start(context.Background())
	wait(t, "the new first entry", func() bool { return s.Read() == 1 })
	s.Stop(context.Background())
}

func TestConfigIsChecked(t *testing.T) {
	if _, err := NewSource(Config{}, nil); err == nil {
		t.Fatal("nowhere to write, accepted")
	}
	if _, err := NewSource(Config{Begin: 9}, &intake{}); err == nil {
		t.Fatal("unknown place to begin, accepted")
	}
}
