---
mkskill:
  pos: 5
  in: ai*
---

# quevu-syslog - agent notes

Go package `github.com/pablo-botella/quevu-syslog`, imported as
`qsyslog`: a `quevu.Consumer` over the system log. One public type,
`qsyslog.Device`, from `Open`. Windows only for now, through
`golang.org/x/sys/windows/svc/eventlog`; on other systems `Open` returns
`ErrUnsupported`. Whether to build Linux and macOS is not decided: do not
add them without being asked.
