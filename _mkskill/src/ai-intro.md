---
mkskill:
  pos: 5
  in: ai*
---

# quevu-syslog - agent notes

Go module `github.com/pablo-botella/quevu-syslog`, one `go.mod`, two
packages that do not import each other:

- `write` (package `qsyslogwrite`): `Open(source, Options)` gives a
  `Device`, a `qkernel.Writer`. `Register`, `Unregister`, `Registered`.
- `read` (package `qsyslogread`): `NewSource(Config, qkernel.TimedIntake)`
  gives a `Source`, a unit with `Start`, `Stop`, `Done`, `Read`, `Lost`.

The root holds no code. Each package has a file per system
(`*_windows.go`, `*_other.go`); a new system is added there, behind the
same small internal interface (`writer` in one, `reader` in the other),
and the tests with a fake log keep working unchanged.
