package qsyslog

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/pablo-botella/quevu"
)

// This test writes real entries to the Application log of the machine,
// so it only runs when asked to:
//
//	set QUEVU_SYSLOG_TEST=1
func TestRealEventLog(t *testing.T) {
	if os.Getenv("QUEVU_SYSLOG_TEST") == "" {
		t.Skip("set QUEVU_SYSLOG_TEST=1 to write to the real Event Log")
	}
	const source = "quevu-syslog-test"
	marker := fmt.Sprintf("marker-%d", time.Now().UnixNano())
	t.Logf("source registered: %v", Registered(source))

	// The usual case: nobody registered the source.
	d, err := Open(source, Options{Level: func(c quevu.Chunk) Level {
		if strings.Contains(string(c.Data), "wrong") {
			return Error
		}
		return Info
	}})
	if err != nil {
		t.Fatal(err)
	}
	err = d.Write([]quevu.Chunk{
		{Seq: 1, Time: time.Now(), Data: []byte("all good " + marker)},
		{Seq: 2, Time: time.Now(), Data: []byte("\x1b[31msomething went wrong\x1b[0m " + marker)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}

	// Read them back the way an administrator would.
	query := fmt.Sprintf("*[System[Provider[@Name='%s']]]", source)
	out, err := exec.Command("wevtutil", "qe", "Application", "/q:"+query, "/c:2", "/rd:true", "/f:xml").CombinedOutput()
	if err != nil {
		t.Fatalf("wevtutil: %v\n%s", err, out)
	}
	got := string(out)
	if strings.Count(got, marker) != 2 {
		t.Fatalf("the entries are not in the Event Log:\n%s", got)
	}
	// Level 2 is Error and 4 is Information, in the Event Log's numbers.
	if !strings.Contains(got, "<Level>2</Level>") || !strings.Contains(got, "<Level>4</Level>") {
		t.Fatalf("levels not as written:\n%s", got)
	}
	if strings.Contains(got, "[31m") {
		t.Fatal("a color sequence reached the Event Log")
	}
}
