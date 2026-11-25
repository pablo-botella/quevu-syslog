# quevu-syslog

The log of the system, seen as what it is: a queue. Something can be
written to it, and something can be read from it. Each side is a piece of
its own, in a package of its own:

| Package | Import path | What it is |
|---|---|---|
| `qsyslogwrite` | `github.com/pablo-botella/quevu-syslog/write` | Writes the chunks of a [quevu](https://github.com/pablo-botella/quevu) log to the system log |
| `qsyslogread` | `github.com/pablo-botella/quevu-syslog/read` | Reads the system log into a quevu log, going on where it left off |

One repository, one module, two packages: Go links only the one that is
imported.

On Windows the system log is the Event Log. On other systems neither side
is built yet: both answer `ErrUnsupported`.

```go
// one program writes
out, _ := qsyslogwrite.Open("billing", qsyslogwrite.Options{})
log, _ := qkernel.NewQHub("alerts", qkernel.WithWriter(out))

// another, maybe later, reads what the first one wrote
in, _ := qsyslogread.NewSource(qsyslogread.Config{Source: "billing", State: `C:\data\billing.place`}, qkernel.NewInlet(hub))
panel.Add("billing-alerts", in)
```

## Install

```
go get github.com/pablo-botella/quevu-syslog
```

## The two sides

They are two different devices. They share nothing but the queue between
them, and neither needs the other:

- The **writer** alone puts what somebody must see where the tools of the
  system look.
- The **reader** alone brings into quevu what any program wrote to the
  system log.
- **Together** they make the system log a way between two programs: one
  writes its lines there and another, that may not even be running at the
  time, picks them up later.

## Writing (`qsyslogwrite`)

A device like the text file or the SQL ones: a hub sends it batches, it
writes them, and it gives nothing back. Each chunk becomes one entry,
with the text as a console shows it (color sequences removed).

The system log is not the place for everything an application says.

- **`Options.Level`** decides the grade of each chunk: `Info`, `Warning`
  or `Error`. A chunk carries no grade of its own, so telling an error
  from a remark is the application's knowledge. Nil means `Info` for all.
- **`Options.Only`** decides which chunks are written at all. The rest
  are taken and dropped.

The usual arrangement is a log of its own for what somebody must see,
with this device as one of the places it goes to.

## Reading (`qsyslogread`)

A unit that, while running, reads the system log and appends every entry
to a quevu log, with the time the entry was written.

| `Config` | What it says |
|---|---|
| `Log` | Which log. Empty: `Application`, where programs write |
| `Source` | Keep only the entries written under this name (the one a writer was opened with). Empty: all |
| `State` | A file where the place of the last entry delivered is kept |
| `Begin` | Where to begin when nothing says: `Newest` (from now on, the default) or `Oldest` |
| `Poll` | How often to look when there is nothing new. Default: a second |

- **With `State`, nothing written while the reader was stopped is
  missed**: the next run goes on from the place noted. The place is noted
  from the first moment, before anything has been read.
- **An entry is noted as read only after the log has taken it.** If the
  log refuses it the reader ends with the error, and the next run begins
  with that entry again. An entry may arrive twice after a hard stop;
  none is skipped.
- **The system log keeps a limited amount.** What is not read in time is
  overwritten; the reader then goes on from the oldest entry there is and
  counts the ones it never saw (`Lost`). It does not invent them.
- **The text is the strings of the entry, as written.** The grade and the
  identifier of the event are not carried over.

## Windows: registered or not

Entries are attributed to a *source*, usually the name of the
application.

- **Not registered** (the default): everything works. Entries are stored,
  and are read back and filtered by source. The Event Viewer puts a
  notice in front of each one saying the description for the event cannot
  be found, followed by the text.
- **Registered**: the Event Viewer shows the text cleanly.
  `qsyslogwrite.Register(source)` records the source in the registry. It
  needs administrator rights and is done once, typically at install time;
  `Unregister` removes it and `Registered` reports which case applies.

Neither side ever tries to register by itself.

## Rules and gotchas

- **An entry that fails to be written does not hold back the rest of its
  batch.** The first error is returned and the hub counts the batch as
  failed, though the other entries were written.
- **Long chunks are cut** at 30,000 bytes, with a mark saying so: the
  Event Log has a limit per entry.
- **Every entry is written with event identifier 1.** quevu output is
  free text, not a catalogue of events.
- **Times read back have the precision of the Event Log: a second.**
- **The tests that touch the real Event Log are off by default.** Set
  `QUEVU_SYSLOG_TEST=1` to run them; they leave a few entries under the
  source `quevu-syslog-test`.

## License

MIT - see [LICENSE](LICENSE).
