---
mkskill:
  pos: 10
  in: readme
---

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
