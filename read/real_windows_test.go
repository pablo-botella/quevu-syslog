package qsyslogread_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	quevu "github.com/pablo-botella/quevu-kernel"
	qsyslogread "github.com/pablo-botella/quevu-syslog/read"
	qsyslogwrite "github.com/pablo-botella/quevu-syslog/write"
)

type lines struct {
	mu  sync.Mutex
	got []string
	at  []time.Time
}

func (l *lines) Write(p []byte) (int, error)     { return len(p), nil }
func (l *lines) Append(d []byte) (uint64, error) { return l.AppendAt(d, time.Now()) }
func (l *lines) AppendAt(d []byte, at time.Time) (uint64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.got = append(l.got, string(d))
	l.at = append(l.at, at)
	return uint64(len(l.got)), nil
}
func (l *lines) with(marker string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, s := range l.got {
		if strings.Contains(s, marker) {
			out = append(out, s)
		}
	}
	return out
}

// The system log as a way between two programs: one writes with
// qsyslogwrite, the other reads with qsyslogread, also what was written
// while it was not running. It writes real entries to the Application
// log of the machine, so it only runs when asked to:
//
//	set QUEVU_SYSLOG_TEST=1
func TestThroughTheRealEventLog(t *testing.T) {
	if os.Getenv("QUEVU_SYSLOG_TEST") == "" {
		t.Skip("set QUEVU_SYSLOG_TEST=1 to write to and read from the real Event Log")
	}
	const source = "quevu-syslog-test"
	marker := fmt.Sprintf("marker-%d", time.Now().UnixNano())
	state := filepath.Join(t.TempDir(), "place")
	in := &lines{}
	read := func() *qsyslogread.Source {
		s, err := qsyslogread.NewSource(qsyslogread.Config{Source: source, State: state, Poll: 100 * time.Millisecond}, in)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		return s
	}
	write := func(texts ...string) {
		d, err := qsyslogwrite.Open(source, qsyslogwrite.Options{})
		if err != nil {
			t.Fatal(err)
		}
		var batch []quevu.Chunk
		for i, text := range texts {
			batch = append(batch, quevu.Chunk{Seq: uint64(i + 1), Time: time.Now(), Data: []byte(text + " " + marker)})
		}
		if err := d.Write(batch); err != nil {
			t.Fatal(err)
		}
		d.Close()
	}
	wait := func(n int) {
		t.Helper()
		for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
			if len(in.with(marker)) >= n {
				return
			}
		}
		t.Fatalf("got %q, wanted %d", in.with(marker), n)
	}

	before := time.Now().Add(-2 * time.Second)
	r := read()
	time.Sleep(300 * time.Millisecond) // reading from "now on"
	write("first", "second: áñ €")
	wait(2)
	if err := r.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Written while the reader is stopped.
	write("while stopped")
	r = read()
	wait(3)
	write("live again")
	wait(4)
	if err := r.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := in.with(marker)
	want := []string{"first " + marker, "second: áñ € " + marker, "while stopped " + marker, "live again " + marker}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("\n got  %q\n want %q", got, want)
	}
	// The time of each entry is the one it was written at.
	in.mu.Lock()
	defer in.mu.Unlock()
	for i, at := range in.at {
		if at.Before(before) || at.After(time.Now().Add(2*time.Second)) {
			t.Fatalf("entry %d carries the time %v", i, at)
		}
	}
	if r.Lost() != 0 {
		t.Errorf("%d entries counted as lost", r.Lost())
	}
}
