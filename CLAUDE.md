# CLAUDE.md

Guidance for working in this repository. Read before making structural changes.
Update when a design variable gets resolved.

## What this is

A small, self-contained **units-of-measure** library in Go. A `Unit` is a symbol
+ `Kind` + conversion factor; a `Value` is a magnitude + its `Unit`.

It is a **foundation module**: consumed by `github.com/lestrrat-3d/sketch` and
`github.com/lestrrat-3d/decad`. It knows about neither.

## Hard rules

- **NEVER depend on anything outside the standard library** (production code).
  `testify/require` in tests only. The value of this module is that anything can
  embed it.
- **NEVER know about a consumer.** No sketch, no CAD, no document state, no
  geometry. Quantities and conversions only — if it is not a quantity or a
  conversion, it does not belong here.
- **NEVER coerce across kinds.** A length is not an angle. Converting between
  kinds returns an `error`; it is never a silent reinterpretation.
- **NEVER accept a unit as a bare string** in the value-building API. Units are
  typed constants. `Lookup` exists for deserialization only.
- **NEVER return a naked `float64`** for a quantity that has a unit. That is the
  bug this library exists to prevent.
- **A `Value` is immutable.** Operations return a new `Value`.

## Layout

| Path | Responsibility |
|---|---|
| `unit.go` | `Kind`, `Unit`, the built-in unit set, `BaseUnit`, `Define`, `Lookup`, the registry. |
| `value.go` | `Value` — magnitude + unit, conversion, arithmetic, formatting. |
| `system.go` | `System` — the current default units, for presenting base-unit quantities. |
| `doc.go` | Package doc: scope + the no-naked-float rule. |

## Conventions

- Go style, testing and file-layout rules: `~/.claude/docs/go.md`. Tests use
  `testify/require` (never `assert`), external `units_test` package.
- Docs state **current state only** — no changelogs, no "was X, now Y".

## Verification

```
go test ./...      # must pass
go vet ./...       # must pass
golangci-lint run  # v2.12.2, config in .golangci.yml
```
