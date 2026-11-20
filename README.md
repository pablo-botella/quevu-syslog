# quevu-syslog

A persistent consumer of [quevu](https://github.com/pablo-botella/quevu-kernel)
that writes to the log of the operating system.

| System | Log | State |
|---|---|---|
| Windows | Event Log (Application) | Working |
| Linux | syslog / the systemd journal | Not built yet |
| macOS | The unified system log | Not built yet |

```go
sys, err := qsyslog.Open("billing", qsyslog.Options{
	// A chunk carries no grade of its own: the application says.
	Level: func(c quevu.Chunk) qsyslog.Level {
		if bytes.Contains(c.Data, []byte("ERROR")) {
			return qsyslog.Error
		}
		return qsyslog.Info
	},
})
alerts, _ := qhub.New("alerts", qhub.WithConsumer(sys), qhub.WithConsumer(screen))
```

## Install

```
go get github.com/pablo-botella/quevu-syslog
```

## What it is

A consumer like the text file or the SQLite ones: a log sends it batches
through a queue of its own, it stores them, and it gives nothing back.
Each chunk becomes one entry, with the text as a console shows it (color
sequences removed).

There is no provider here: reading the system log back is not built.

## What goes in, and how much it matters

The system log is not the place for everything an application says.

- **`Options.Level`** decides the grade of each chunk: `Info`, `Warning`
  or `Error`. A chunk carries no grade of its own, so telling an error
  from a remark is the application's knowledge. Nil means `Info` for all.
- **`Options.Only`** decides which chunks are written at all. The rest are
  taken and dropped.

The usual arrangement is a log of its own for what somebody must see,
with this device as one of its consumers.

## Windows: registered or not

Entries are attributed to a *source*, usually the name of the
application.

- **Not registered** (the default): everything works. Entries are stored
  and can be read and filtered by source. The Event Viewer puts a notice
  in front of each one saying the description for the event cannot be
  found, followed by the text.
- **Registered**: the Event Viewer shows the text cleanly.
  `qsyslog.Register(source)` records the source in the registry. It needs
  administrator rights and is done once, typically at install time;
  `Unregister` removes it and `Registered` reports which case applies.

The device never tries to register by itself.

## Rules and gotchas

- **An entry that fails does not hold back the rest of its batch.** The
  first error is returned and the log counts the batch as failed, though
  the other entries were written.
- **Long chunks are cut** at 30,000 bytes, with a mark saying so: the
  Event Log has a limit per entry.
- **Every entry has event identifier 1.** quevu output is free text, not a
  catalogue of events.
- **The test that writes to the real Event Log is off by default.** Set
  `QUEVU_SYSLOG_TEST=1` to run it; it leaves two entries under the source
  `quevu-syslog-test`.

## License

MIT - see [LICENSE](LICENSE).
