# CLAUDE.md

Guidance for working in this repository. Read before making structural changes.
Update when a design variable gets resolved.

## What this is

A small, self-contained **units-of-measure** library in Go. A `Unit` is a symbol
+ `Kind` + conversion factor; a `Value` is a magnitude + its `Unit`. A unit whose
zero is not its kind's base zero (the degree Celsius) is a **separate type**,
`AffineUnit`, carrying a separate `AffineValue`. A `Kind` is a vector of dimension exponents (length,
mass, angle, time, temperature), so kinds compose: `Length.Mul(Length) == Area`,
and `Value.Mul`/`Value.Div` derive the result kind instead of enumerating it.
Design contract: `docs/kinds-design.md`.

It is a **foundation module**: consumed by `github.com/lestrrat-3d/sketch`,
`github.com/lestrrat-3d/decad` and `github.com/lestrrat-3d/osafune`. It knows
about none of them.

## Hard rules

- **NEVER depend on anything outside the standard library** (production code).
  `testify/require` in tests only. The value of this module is that anything can
  embed it.
- **NEVER know about a consumer.** No sketch, no CAD, no document state, no
  geometry. Quantities and conversions only — if it is not a quantity or a
  conversion, it does not belong here.
- **NEVER give a type a method it must refuse for its own values.** This is why
  `AffineUnit`/`AffineValue` are separate types rather than an `offset` field on
  `Unit`. An affine quantity has no meaningful `Add`, `Sub`, `Mul`, `Div`, `Scale`
  or `Neg` — `20 °C × 2` is not `40 °C`, and two absolute temperatures do not add
  — so `AffineValue` does not HAVE them, rather than having them and returning an
  error. The earlier one-type design could not hold the line: `Scale`/`Neg` have
  no error channel, so `Celsius(20).Scale(2)` silently gave `40 degC`. Splitting
  the **unit** type (not just the value) is what makes it a compile error, because
  a unit is a run-time value and `New(20, Celsius)` would otherwise have to panic.
  Cross with `AffineValue.ToRatio(Kelvin)` and compute there. NEVER add arithmetic
  to `AffineValue`, and NEVER re-add an `offset` to `Unit`. `DefineAffine` is for a
  unit whose **zero really moves**, never for folding a datum, a bias or a
  calibration constant into a unit; it rejects a zero offset for that reason.
- **One symbol namespace, two registries.** `Define` and `DefineAffine` both
  refuse a symbol either table holds, so `Lookup` and `LookupAffine` never
  disagree and neither text form can carry the other type's quantity. The affine
  conversion is ONE rational expression rounded once, never a shift composed with
  a rescale — the rule below applies to it unchanged.
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
  `rescale`/`product`/`quotient`/`sum` in `value.go`, which work on the `math.Frexp`
  mantissa and exponent and reassemble once with `math.Ldexp`; a new operation
  MUST use them, and `Div`'s zero guard reads the divisor's own **magnitude**
  (a unit factor is positive and finite, so `mag == 0` iff the quantity is zero).
  `Base()` stays a public **accessor** — it may honestly report `+Inf` or `0` —
  and no operation reads it, `System.In` (which calls `rescale` for the number it
  returns, having no error to pass on) included.
- **An operation is ONE helper, never a composition of them.** A step that is sound
  on its own does not make the composition sound: the rounding *between* the steps
  has already happened. `Add`/`Sub` are `sum` — the whole addition, decided on the
  **true sum** — and NEVER a rescale of each operand followed by an addition, which
  rounds each rescaled operand to 53 bits and so destroys exactly the bits the
  addition would have cancelled against: `New(-math.MaxFloat64, tiny)` plus
  `New(1.7976931348623157e-292, huge)` (factors `1e-300` and `1e300`) is
  `6.531456099116113e+291 tiny`, not the `0` with a nil error that two roundings
  leave. A cancellation is **not an extreme** — the operands are, the result is not —
  so the `atTheEnds` exponent guard cannot see it: `sum` keeps the fast path only
  where both operands are already carried in the result's unit (both terms are then
  exact, and an IEEE addition of exact terms is correctly rounded), and redoes the
  arithmetic in exact rationals wherever a rescale would round an operand on the way
  in. `Add` and `Sub` are correctly rounded, always — the suite asserts it bit for
  bit, and sweeps each operand against the one that most nearly annihilates it.
  **`Equal` is the same rule, not an exception to it.** A tolerance predicate that
  rescales both operands into a common unit and subtracts *there* is a composition,
  and the rounding in between erases the difference it was asked to judge:
  `Millimeters(1e-300)` against `New(0, huge)` (factor `1e300`) rescales to `0 − 0`,
  and two quantities that differ by `1e-300 mm` come back equal at `tol == 0`. So
  `Equal` is `|v − o| <= tol` on the **true** difference in base units, in exact
  rationals wherever float64 cannot hold it, with the fast path kept only where it is
  lossless (both operands in the same unit: equal quantities iff equal magnitudes).
  Symmetry and reflexivity follow from the difference being the true one.
- **A non-finite magnitude is NOT a quantity, and `Equal` never puts one beside
  one.** `New`/`FromBase`/`Scale`/`Neg` can build a `Value` carrying an `±Inf` or a
  `NaN`, so `Equal` answers it from the magnitudes, before any arithmetic: exactly
  one non-finite → **false at every `tol`, `+Inf` included** (there is no real
  difference for a tolerance to bound); both infinite → true iff the **same signed
  infinity** (a factor is positive, so the magnitude's sign is the quantity's), which
  is what keeps `Equal` reflexive for a value built from an infinity; either a `NaN`
  → false, itself included. A `tol` that is negative or a `NaN` admits nothing.
  NEVER let this reach the float path, where `|Inf − 1|` is `+Inf` (which an infinite
  `tol` admits) and `|Inf − Inf|` a `NaN` (which breaks reflexivity).
- **`tol == 0` means the same real number — and two units generally do not agree on
  one.** A unit's factor is itself a *rounded* float64, so `Degrees(180)` and
  `Radians(math.Pi)` are different quantities (`Degree`'s factor is a rounded
  `pi/180`), as are `Kilograms(1)` and `Grams(1000)` (a rounded `0.001`).
  `Equal(…, 0)` says so, and that is the honest answer: it is for values in the same
  unit, or for asking whether two quantities are *exactly* the same real number. A
  cross-unit comparison passes a tolerance, in base units. NEVER "fix" this by
  rounding the difference back into some unit first — that is the composition above.
- **The helpers keep the plain expression's rounding — and add none.** The
  `Frexp`/`Ldexp` split buys range, and it MUST NOT be paid for in accuracy: the
  mantissas are combined in the same order, and grouped the same way, as the plain
  `mag * from / to` and `(a * af) * (b * bf)` — reassociating them adds a rounding,
  and `25.4 mm` stops being exactly `1 in`. The suite asserts exactness and a
  no-worse-than-naive ulp bound against a `big.Rat` oracle, over the extremes
  (subnormals, `1e308`, `MaxFloat64`, `Define`d factors of `1e±300`) as well as
  everyday magnitudes; a relative tolerance (even `1e-9`, some `10⁷` ulps) cannot
  gate this class, so NEVER assert an accuracy claim with one.
- **A rounded mantissa MUST NOT decide a boundary.** A mantissa rounded to 53 bits
  while its exponent is still carried separately cannot see the ends of the range:
  it cannot overflow (so a true product past the last float64 assembles into a
  finite `MaxFloat64` with a **nil error** — the poisoned document), it can round
  **up** over the boundary (so a representable quotient assembles into `+Inf` and
  is refused), and `Ldexp` rounds it a **second** time into the subnormals (worse
  than the plain expression, which rounds once: `Scalar(1.25).Div(Centimeters(
  1e307))` is `1.25e-308`, not a bit less). So past a binary exponent of `±1000`
  (`atTheEnds`, twenty-odd binades clear of every boundary) `rescale`/`product`/
  `quotient` redo the arithmetic in exact rationals (`math/big`) and round **once**
  — correctly rounded, an infinity exactly when the true result is past the last
  float64. There is **no ambiguous band** near `MaxFloat64`: the oracle decides
  every point, and the suite sweeps the boundary at both signs. NEVER write a test
  that declares that band untestable.
- **NEVER hand back a finite number in a unit it is not in.** `System.In` answers
  in `UnitFor(v.Kind())` — always. A magnitude that unit cannot hold comes back
  as the infinity it is; `System.Display` returns a value it cannot carry in the
  presentation unit unchanged, in its own unit.
- **NEVER accept a unit as a bare string** in the value-building API. Units are
  typed constants. `Lookup` exists for deserialization only.
- **NEVER return a naked `float64`** for a quantity that has a unit. That is the
  bug this library exists to prevent.
- **A zero result is always `+0`; a `Value` NEVER carries a negative zero.** The
  sign of a zero is not a property of a quantity — there is no `-0 mm` — it records
  how a float64 expression reached zero, and it must not be observable. `Add`/`Sub`
  are correctly rounded from the true sum, and the true sum of two quantities that
  annihilate is *one* zero; but an IEEE addition gives `(-0) + (-0) == -0` while an
  exact rational carries no signed zero, so an uncanonicalised result would say which
  path `sum` took — the fast one (both operands already in the result's unit) or the
  exact one. **An operation's result must never depend on which internal path computed
  it.** So a zero is canonicalised (`canonicalZero`) wherever one can arise: `sum`
  (both arms, the angle/dimensionless carve-out included), `product`, `quotient`,
  `rescale`, `exact`, `Scale`, `Neg`, `Base` — and **on construction**, in `New`,
  `FromBase` and every built-in constructor, so `Mag()` never returns a `-0`, nothing
  prints as `-0 mm`, and no operation has to defend against one arriving from outside.
  The suite asserts it on `math.Float64bits`: in IEEE `+0 == -0`, so **NEVER state an
  exactness claim about a float64 with `==` or `require.Equal`** — it cannot see this.
- **A `Value` is immutable.** Operations return a new `Value`. The zero `Value`
  is 0 of `One`: the zero `Unit` is read as `One`, so a `var`-declared `Value`
  behaves as a plain 0 in every operation. `UnmarshalText` is the one pointer
  method — `encoding.TextUnmarshaler` requires it — and it **assigns** the whole
  value (`*v = New(...)`) rather than mutating a field.
- **The text form carries the unit, and round-trips bit for bit.**
  `Value.MarshalText`/`UnmarshalText` are `encoding.TextMarshaler`/
  `TextUnmarshaler`, so `encoding/json` uses them: `"<magnitude> <symbol>"` —
  `"10 mm"`, `"7850 kg/m^3"` — and the bare number for a dimensionless value, whose
  unit `One` has an **empty symbol**. Without them a `Value`'s unexported fields
  encode to `{}` **with a nil error**: the quantity deleted, silently, in a document
  that is the whole reason this library exists.
  - **The round trip is exact, not close.** The magnitude is
    `strconv.FormatFloat(m, 'g', -1, 64)` — the shortest text that reads back as the
    *same* float64 — and `strconv.ParseFloat` reads it; the symbol goes through
    `Lookup`, which hands back the very unit that wrote it. `25.4 mm` is exactly
    `1 in` on both sides of a document, and the suite asserts the round trip on
    `math.Float64bits` over every built-in unit × a magnitude sweep (subnormals,
    `MaxFloat64`, both zeros) **and** over 200k random bit patterns. NEVER "improve"
    the formatting to a fixed precision — that is a lost bit, which is a lost
    guarantee.
  - **A registered symbol is a readable symbol — enforced at `Define`.** The symbol
    grammar is ONE rule: **printable ASCII except the space** (every byte `!` through
    `~`), not opening with `[`. `Define` **panics** on anything else and registers
    nothing, as it does for a duplicate symbol, an overflowed kind or an unusable
    factor. A registered symbol must survive the library's parser, a standard text
    encoder, AND a reader's eyes, **byte-identically**. `One`'s **empty** symbol is
    the one symbol with no separator, and it stays `One`'s: `Define("")` collides with
    it. NEVER patch any of this in `MarshalText` instead — the invariant is that an
    unreadable symbol is **unregistrable**, which is what makes `Lookup` a sufficient
    guard.
  - **Why each class is refused.** A **non-ASCII** symbol is a homoglyph trap: `mm²`
    is a different registry key from the built-in `mm^2` yet renders identically, so a
    document saying `10 mm²` would parse, nil-error, to whatever unit wore the
    lookalike (same for Cyrillic `мм`, fullwidth `ｍｍ`, combining marks). Refusing the
    class whole also keeps everything an encoder rewrites out of the registry: pure
    ASCII is valid UTF-8 (`encoding.TextMarshaler` is a UTF-8 contract; `encoding/json`
    replaces invalid bytes with U+FFFD) and can carry neither **U+FFFD** (the standing
    alias target every corrupted document would resolve to — a value deserializing as
    a **different kind**) nor **U+FFFE**/**U+FFFF** nor a Unicode space. A real-world
    non-ASCII symbol (µm, °, Å) registers under an ASCII spelling (`um`, `deg`,
    `angstrom`), as the built-ins do. **Whitespace** (the space and the C0 whitespace
    controls) is the text form's separator: `"3 probe space"` cuts into a magnitude
    and two tokens. A **control character** (the rest of C0, and DEL) is what
    `encoding/xml` rewrites as U+FFFD rather than fail. `MarshalText` output is
    therefore always valid UTF-8, and the suite's hostile-symbol property test
    round-trips every accepted symbol **through `encoding/json`**, not just through
    MarshalText/UnmarshalText.
  - **NEVER emit a symbol that cannot be read back.** The marshaller's check *is*
    `Lookup` — and `Lookup` is *enough*, because registered means readable (above). A
    value of an **unnamed kind** (synthetic `[L^-1]`) is
    `ErrUnnamedKind`, and one of an **overflowed kind** (`[overflow]`) is
    `ErrOverflowedKind` — the "must not be persisted" rule, enforced at the one place
    it would have been broken. A **non-finite** magnitude — `New` and friends can
    build one — is `ErrNotFinite`: a persisted `+Inf mm` read back is a length that
    is not a length.
  - **NEVER guess a unit.** An unregistered symbol is `ErrUnknownUnit`; there is no
    fallback to a base unit and no silent `One`. Malformed text (`ErrMalformedText`)
    is anything that is not `<magnitude> <symbol>` — a trailing or doubled space, an
    extra token, a magnitude `ParseFloat` rejects — so the bare-number dimensionless
    form is unambiguous. A literal `+Inf`/`NaN`, or one past the last float64
    (`1e999`), is `ErrNotFinite`; one below the smallest subnormal (`1e-999`) is the
    nearest float64, `+0`, the same rounding the arithmetic makes there.
  - A marshalled zero is `"0"`, and any zero in a document — `-0` included — reads
    back as `+0`, bit for bit. The negative-zero rule holds across the boundary.
- **NEVER persist a value of an unnamed kind.** It carries a synthetic,
  unregistered unit (`[L^-1]`) that `Lookup` cannot resolve; it is a transient
  intermediate. Every named kind has a registered base unit — convert first.
- **An overflowed kind is STICKY.** Exponents are `int8` and saturate rather than
  wrap, but a saturated exponent is a *lie about the number*, so `Kind` carries an
  `ovf` flag that `Mul`/`Div`/`Pow` propagate from either operand and nothing ever
  clears. Without it, `Length.Pow(math.MaxInt64).Div(Length.Pow(126))` would come
  back as `Length` — an astronomically overflowed quantity wearing a plausible
  kind. An overflowed kind equals no named kind, prints `overflowed`, has no base
  unit, and carries the reserved synthetic symbol `[overflow]`. **`Define` panics
  on one**, registering nothing: it takes a `Kind` from the caller, and a
  registered symbol is a symbol `Lookup` resolves and a document persists — the
  one way an overflowed kind could be laundered into an ordinary unit. Keep `Kind`
  comparable and its zero value `Dimensionless`.

## Layout

| Path | Responsibility |
|---|---|
| `kind.go` | `Kind` — dimension exponents, the named kinds, `Mul`/`Div`/`Pow`, `Overflowed`, `String`. |
| `unit.go` | `Unit`, the built-in unit set, `BaseUnit`, `Define`, `Lookup`, the mutex-guarded registry. |
| `value.go` | `Value` — magnitude + unit, conversion, arithmetic, formatting, the text form (`MarshalText`/`UnmarshalText`). |
| `system.go` | `System` — the current default units, for presenting base-unit quantities. |
| `doc.go` | Package doc: scope + the no-naked-float rule. |

## Conventions

- Go style, testing and file-layout rules: `~/.claude/docs/go.md`. Tests use
  `testify/require` (never `assert`), external `units_test` package.
- Docs state **current state only** — no changelogs, no "was X, now Y".
- **Unit symbols are printable ASCII except the space** — every byte `!` through
  `~` — with a caret for an exponent: `mm^2`, `in^3`, `kg/m^3`. `Define` panics
  on anything else: non-ASCII (a lookalike such as `mm²` must never alias the
  built-in `mm^2`), whitespace (the text form's separator), a control character
  (a text encoder rewrites one). A real-world non-ASCII symbol (µm, °, Å)
  registers under an ASCII spelling (`um`, `deg`, `angstrom`), as the built-ins
  do. `Kind.String()` is the one place Unicode superscripts appear (`L⁻¹`), and
  it is display text, never a unit symbol or a registry key.
- **`[…]` is a reserved symbol namespace.** The synthetic unit a value of an
  unnamed kind carries is `Kind.canonicalSymbol()` (`[L^-1]`, `[L^2*M]`, ASCII,
  order L·M·A). `Define` panics on a symbol opening with `[`.

## Verification

```
go test ./...      # must pass
go vet ./...       # must pass
golangci-lint run  # v2.12.2, config in .golangci.yml
```
