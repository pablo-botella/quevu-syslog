---
mkskill:
  pos: 10
  in: readme
---

# quevu-syslogwrite

A persistent consumer of [quevu](https://github.com/pablo-botella/quevu-kernel)
that writes to the log of the operating system.

| System | Log | State |
|---|---|---|
| Windows | Event Log (Application) | Working |
| Linux | syslog / the systemd journal | Not built yet |
| macOS | The unified system log | Not built yet |

```go
sys, err := qsyslogwrite.Open("billing", qsyslogwrite.Options{
	// A chunk carries no grade of its own: the application says.
	Level: func(c quevu.Chunk) qsyslogwrite.Level {
		if bytes.Contains(c.Data, []byte("ERROR")) {
			return qsyslogwrite.Error
		}
		return qsyslogwrite.Info
	},
})
alerts, _ := qhub.New("alerts", qhub.WithConsumer(sys), qhub.WithConsumer(screen))
```
