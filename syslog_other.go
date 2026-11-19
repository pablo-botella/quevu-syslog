//go:build !windows

package qsyslog

func open(string) (writer, error) { return nil, ErrUnsupported }

// Register does nothing on this system: only the Windows Event Log has
// sources to register.
func Register(string) error { return nil }

// Unregister does nothing on this system.
func Unregister(string) error { return nil }

// Registered is always false on this system.
func Registered(string) bool { return false }
