//go:build !windows

package qsyslogread

const supported = false

func openLog(string) (reader, error) { return nil, ErrUnsupported }
