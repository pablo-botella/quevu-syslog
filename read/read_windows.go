package qsyslogread

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const supported = true

var (
	advapi32                   = windows.NewLazySystemDLL("advapi32.dll")
	procOpenEventLog           = advapi32.NewProc("OpenEventLogW")
	procCloseEventLog          = advapi32.NewProc("CloseEventLog")
	procReadEventLog           = advapi32.NewProc("ReadEventLogW")
	procGetOldestEventLogRec   = advapi32.NewProc("GetOldestEventLogRecord")
	procGetNumberOfEventLogRec = advapi32.NewProc("GetNumberOfEventLogRecords")
)

const (
	seekRead     = 0x0002
	forwardsRead = 0x0004
	// The fixed part of an entry, before its names and strings.
	recordHeader = 56
	readBuffer   = 64 * 1024
)

type eventLog struct {
	h   windows.Handle
	buf []byte
}

func openLog(name string) (reader, error) {
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	h, _, e := procOpenEventLog.Call(0, uintptr(unsafe.Pointer(p)))
	if h == 0 {
		return nil, fmt.Errorf("qsyslogread: opening the log %s: %w", name, e)
	}
	return &eventLog{h: windows.Handle(h), buf: make([]byte, readBuffer)}, nil
}

func (l *eventLog) close() error {
	if r, _, e := procCloseEventLog.Call(uintptr(l.h)); r == 0 {
		return e
	}
	return nil
}

func (l *eventLog) bounds() (oldest, next uint32, err error) {
	var count uint32
	if r, _, e := procGetNumberOfEventLogRec.Call(uintptr(l.h), uintptr(unsafe.Pointer(&count))); r == 0 {
		return 0, 0, fmt.Errorf("qsyslogread: counting the entries: %w", e)
	}
	if r, _, e := procGetOldestEventLogRec.Call(uintptr(l.h), uintptr(unsafe.Pointer(&oldest))); r == 0 {
		return 0, 0, fmt.Errorf("qsyslogread: finding the oldest entry: %w", e)
	}
	if count == 0 {
		// An empty log has no oldest entry; the next one written will
		// say where it begins.
		return oldest, oldest, nil
	}
	return oldest, oldest + count, nil
}

func (l *eventLog) read(record uint32) ([]entry, error) {
	for {
		var got, needed uint32
		r, _, e := procReadEventLog.Call(uintptr(l.h), seekRead|forwardsRead, uintptr(record),
			uintptr(unsafe.Pointer(&l.buf[0])), uintptr(len(l.buf)), uintptr(unsafe.Pointer(&got)), uintptr(unsafe.Pointer(&needed)))
		if r == 0 {
			switch {
			case errors.Is(e, syscall.ERROR_INSUFFICIENT_BUFFER) && int(needed) > len(l.buf):
				l.buf = make([]byte, needed)
				continue
			case errors.Is(e, windows.ERROR_HANDLE_EOF), errors.Is(e, windows.ERROR_INVALID_PARAMETER):
				// Nothing at that record: not written yet, or gone. The
				// caller looks at the bounds again and decides.
				return nil, nil
			}
			return nil, fmt.Errorf("qsyslogread: reading entry %d: %w", record, e)
		}
		return parse(l.buf[:got]), nil
	}
}

// parse cuts a bufferful of entries. Each is a fixed header followed by
// the name of who wrote it, the name of the machine, and the strings of
// the message.
func parse(b []byte) []entry {
	var out []entry
	for len(b) >= recordHeader {
		length := int(binary.LittleEndian.Uint32(b[0:]))
		if length < recordHeader || length > len(b) {
			break
		}
		rec := b[:length]
		e := entry{
			record: binary.LittleEndian.Uint32(rec[8:]),
			at:     time.Unix(int64(binary.LittleEndian.Uint32(rec[12:])), 0),
		}
		strCount := int(binary.LittleEndian.Uint16(rec[26:]))
		strOffset := int(binary.LittleEndian.Uint32(rec[36:]))
		e.source, _ = wideString(rec, recordHeader)
		var parts []string
		for i, at := 0, strOffset; i < strCount && at < len(rec); i++ {
			s, next := wideString(rec, at)
			parts = append(parts, s)
			at = next
		}
		e.text = strings.Join(parts, " ")
		out = append(out, e)
		b = b[length:]
	}
	return out
}

// wideString reads the text that ends at the next zero, from at, and
// answers where what follows it begins.
func wideString(b []byte, at int) (string, int) {
	var u []uint16
	for at+1 < len(b) {
		c := binary.LittleEndian.Uint16(b[at:])
		at += 2
		if c == 0 {
			break
		}
		u = append(u, c)
	}
	return string(utf16.Decode(u)), at
}
