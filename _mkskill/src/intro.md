---
mkskill:
  pos: 10
  in: readme
---

# quevu-syslog

The log of the system, seen as what it is: a queue. Something can be
written to it, and something can be read from it. Each side is a piece of
its own, in a package of its own:

| Package | Import path | What it is |
|---|---|---|
| `qsyslogwrite` | `github.com/pablo-botella/quevu-syslog/write` | Writes the chunks of a [quevu](https://github.com/pablo-botella/quevu-kernel) log to the system log |
| `qsyslogread` | `github.com/pablo-botella/quevu-syslog/read` | Reads the system log into a quevu log, going on where it left off |

One repository, one module, two packages: Go links only the one that is
imported.

On Windows the system log is the Event Log. On other systems neither side
is built yet: both answer `ErrUnsupported`.

```go
// one program writes
out, _ := qsyslogwrite.Open("billing", qsyslogwrite.Options{})
log, _ := qhub.New("alerts", qhub.WithWriter(out))

// another, maybe later, reads what the first one wrote
in, _ := qsyslogread.NewSource(qsyslogread.Config{Source: "billing", State: `C:\data\billing.place`}, quevu.NewInlet(hub))
panel.Add("billing-alerts", in)
```
