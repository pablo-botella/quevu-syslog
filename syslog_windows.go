package qsyslogwrite

import (
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc/eventlog"
)

// eventID is the identifier every entry is written with. Once a source
// is registered with Register, identifiers from 1 to 1000 display their
// text as written.
const eventID = 1

const sourcesKey = `SYSTEM\CurrentControlSet\Services\EventLog\Application`

type eventWriter struct{ l *eventlog.Log }

func open(source string) (writer, error) {
	// Works for a source that is not registered: the entries are stored
	// all the same.
	l, err := eventlog.Open(source)
	if err != nil {
		return nil, err
	}
	return eventWriter{l}, nil
}

func (w eventWriter) write(level Level, text string) error {
	switch level {
	case Error:
		return w.l.Error(eventID, text)
	case Warning:
		return w.l.Warning(eventID, text)
	}
	return w.l.Info(eventID, text)
}

func (w eventWriter) close() error { return w.l.Close() }

// Register records the source in the Windows registry so the Event
// Viewer displays its entries cleanly. It needs administrator rights and
// is done once, typically when the application is installed.
//
// Without it everything still works: entries are stored and can be read,
// but the Event Viewer puts a notice in front of each one saying the
// description for the event cannot be found.
func Register(source string) error {
	if Registered(source) {
		return nil
	}
	return eventlog.InstallAsEventCreate(source, eventlog.Info|eventlog.Warning|eventlog.Error)
}

// Unregister removes what Register recorded. It needs administrator
// rights.
func Unregister(source string) error {
	if !Registered(source) {
		return nil
	}
	return eventlog.Remove(source)
}

// Registered reports whether the source is recorded in the registry.
func Registered(source string) bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, sourcesKey+`\`+source, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	k.Close()
	return true
}
