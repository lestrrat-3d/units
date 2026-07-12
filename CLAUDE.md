# CLAUDE.md

Guidance for working in this repository. Read before making structural changes.
Update when a design variable gets resolved.

## What this is

A small, self-contained **units-of-measure** library in Go. A `Unit` is a symbol
+ `Kind` + conversion factor; a `Value` is a magnitude + its `Unit`. A `Kind` is
a vector of dimension exponents (length, mass, angle), so kinds compose:
`Length.Mul(Length) == Area`, and `Value.Mul`/`Value.Div` derive the result kind
instead of enumerating it. Design contract: `docs/kinds-design.md`.

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
  kinds returns an `error`; it is never a silent reinterpretation. `Angle` is a
  dimension of its own, never unified with `Dimensionless`; the sole carve-out
  is `Add`/`Sub` between an angle and a dimensionless value. That includes
  presentation: `System.UnitFor(k)` returns a unit **of kind `k`**, falling back
  to the kind's base unit — or its synthetic factor-1 unit — rather than to
  `One`. `System` has exported fields and a usable zero value, so `UnitFor`
  **validates** them: a `Length`/`Angle` field that is unset or holds a unit of
  another kind is ignored.
- **NEVER let a non-finite magnitude escape from an operation that can report
  it.** `Add`, `Sub`, `Mul`, `Div`, `In` and `Convert` return a finite result or
  an error (`ErrDivideByZero`, `ErrNotFinite`) — never `+Inf`, never `NaN`. A
  result of a named kind carries a registered symbol, so an infinity would
  persist as if it were a real quantity. The error-free constructors and
  operations (`New`, `FromBase`, `Scale`, `Neg`) cannot check, and say so in
  their docs. Likewise a unit's factor: `Define` panics on a zero, negative,
  infinite or NaN factor.
- **NEVER form a base magnitude as an intermediate.** `mag * unit.factor` — what
  `Value.Base()` returns — overflows for ordinary values (`Meters(1e307)` is
  `1e310 mm`, `+Inf`) and underflows for others (`Grams(1e-322)` is `1e-325 kg`,
  `0`). `ErrNotFinite` is about the **result**, so an operation that routed
  through `Base()` would refuse a representable result, report a divide-by-zero
  for a nonzero divisor, or (`Equal`, `System.In` — no error channel) return a
  wrong one. Every operation does its arithmetic through
  `rescale`/`product`/`quotient` in `value.go`, which work on the `math.Frexp`
  mantissa and exponent and reassemble once with `math.Ldexp`; a new operation
  MUST use them, and `Div`'s zero guard reads the divisor's own **magnitude**
  (a unit factor is positive and finite, so `mag == 0` iff the quantity is zero).
  `Base()` stays a public **accessor** — it may honestly report `+Inf` or `0` —
  and no operation reads it.
- **The helpers keep the plain expression's rounding.** The `Frexp`/`Ldexp` split
  buys range, and it MUST NOT be paid for in accuracy: the mantissas are combined
  in the same order, and grouped the same way, as the plain `mag * from / to` and
  `(a * af) * (b * bf)` — reassociating them adds a rounding, and `25.4 mm` stops
  being exactly `1 in`. The suite asserts exactness and a no-worse-than-naive ulp
  bound against a `big.Rat` oracle; a relative tolerance (even `1e-9`, some `10⁷`
  ulps) cannot gate this class, so NEVER assert an accuracy claim with one.
- **NEVER hand back a finite number in a unit it is not in.** `System.In` answers
  in `UnitFor(v.Kind())` — always. A magnitude that unit cannot hold comes back
  as the infinity it is; `System.Display` returns a value it cannot carry in the
  presentation unit unchanged, in its own unit.
- **NEVER accept a unit as a bare string** in the value-building API. Units are
  typed constants. `Lookup` exists for deserialization only.
- **NEVER return a naked `float64`** for a quantity that has a unit. That is the
  bug this library exists to prevent.
- **A `Value` is immutable.** Operations return a new `Value`. The zero `Value`
  is 0 of `One`: the zero `Unit` is read as `One`, so a `var`-declared `Value`
  behaves as a plain 0 in every operation.
- **NEVER persist a value of an unnamed kind.** It carries a synthetic,
  unregistered unit (`[L^-1]`) that `Lookup` cannot resolve; it is a transient
  intermediate. Every named kind has a registered base unit — convert first.

## Layout

| Path | Responsibility |
|---|---|
| `kind.go` | `Kind` — dimension exponents, the named kinds, `Mul`/`Div`/`Pow`, `String`. |
| `unit.go` | `Unit`, the built-in unit set, `BaseUnit`, `Define`, `Lookup`, the mutex-guarded registry. |
| `value.go` | `Value` — magnitude + unit, conversion, arithmetic, formatting. |
| `system.go` | `System` — the current default units, for presenting base-unit quantities. |
| `doc.go` | Package doc: scope + the no-naked-float rule. |

## Conventions

- Go style, testing and file-layout rules: `~/.claude/docs/go.md`. Tests use
  `testify/require` (never `assert`), external `units_test` package.
- Docs state **current state only** — no changelogs, no "was X, now Y".
- **Unit symbols are ASCII**, with a caret for an exponent: `mm^2`, `in^3`,
  `kg/m^3`. `Kind.String()` is the one place Unicode superscripts appear (`L⁻¹`),
  and it is display text, never a unit symbol or a registry key.
- **`[…]` is a reserved symbol namespace.** The synthetic unit a value of an
  unnamed kind carries is `Kind.canonicalSymbol()` (`[L^-1]`, `[L^2*M]`, ASCII,
  order L·M·A). `Define` panics on a symbol opening with `[`.

## Verification

```
go test ./...      # must pass
go vet ./...       # must pass
golangci-lint run  # v2.12.2, config in .golangci.yml
```
