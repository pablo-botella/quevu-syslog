---
mkskill:
  pos: 5
  in: ai*
---

# quevu-syslogwrite - agent notes

Go package `github.com/pablo-botella/quevu-syslogwrite`, imported as
`qsyslogwrite`: a `quevu.Consumer` over the system log. One public type,
`qsyslogwrite.Device`, from `Open`. Windows only for now, through
`golang.org/x/sys/windows/svc/eventlog`; on other systems `Open` returns
`ErrUnsupported`. Whether to build Linux and macOS is not decided: do not
add them without being asked.
