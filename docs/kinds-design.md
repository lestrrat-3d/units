# Kinds — the dimension model

`Kind` is what a quantity *is*: a length, an area, a mass. This document is the
contract for how kinds are represented and how they compose.

## 1. The problem with an enumeration

A closed enum of named quantities cannot express arithmetic. Enumerate `Length`,
`Area`, `Volume`, `Mass`, `Density`, and you still cannot answer *"what is
`Area × Length`?"* — because the answer is a fact about **dimensions**, not a
name in a list.

The symptom is a hard error where a result should be:

```go
// with a closed enum, the only honest thing to write:
return 0, fmt.Errorf("cannot multiply a %s by a %s (no compound unit)", ka, kb)
```

That is a ceiling, not a missing feature. Adding `Area` and `Volume` as two more
enum constants does not raise it: `length × length` still has nowhere to land
unless something *computes* that the result has length-exponent 2.

And the enumeration explodes. A CAD verification layer needs mass properties:
moment of inertia (`M·L²`), density (`M·L⁻³`), second moment of area (`L⁴`),
radius of gyration (`L`), section modulus (`L³`). Each is a new constant, each
pairwise product is a new special case, and none of them compose.

## 2. The model: exponents, not names

A `Kind` is a **vector of dimension exponents**. Multiplication adds them,
division subtracts them, and every derived quantity falls out for free.

```go
// Kind is the physical dimension of a quantity: the exponents of the base
// dimensions it is built from. The zero value is Dimensionless.
//
// Kind is comparable: two kinds are equal exactly when their exponents match and
// neither has overflowed.
type Kind struct {
    l, m, a int8 // length, mass, angle
    ovf     bool // an exponent saturated; sticky (see §4)
}
```

Three base dimensions, because three is what this domain needs:

| | Why |
|---|---|
| **Length** (`l`) | the base of all geometry: length, area, volume, inertia |
| **Mass** (`m`) | mass properties: mass, density, moment of inertia |
| **Angle** (`a`) | **see below — this one is a judgment call, not physics** |

There is no time, current, or temperature. This is a geometry library; a
quantity that needs seconds does not belong in it. Exponents are `int8` — `L⁴` is
as exotic as it gets — so the whole `Kind` stays a handful of bytes and comparable
with `==`, and it carries one bit besides the exponents: the overflow mark, which
§4 explains.

### Angle is tracked, though physics says it is dimensionless

Radians are a ratio of two lengths, so strictly `Angle` *is* `Dimensionless`.
Tracking it anyway is a deliberate deviation, and it is the single most valuable
thing this library does.

If angle were dimensionless, then `Radians(2)` and `Scalar(2)` would be the same
kind, and this would type-check:

```go
taper := units.Scalar(2)   // "2 what?"
extrude.WithTaper(taper)   // silently 2 radians
```

That is exactly the `ValueInput.createByReal(2)` trap — where Fusion silently
reads a bare `2` as 2 *radians* — that the consumers of this library exist to
escape. So `Angle` gets its own exponent and never unifies with a bare number.

The cost is one wart, inherited from the existing behavior and kept: **an angle
may be added to a dimensionless value** (`theta + pi/2` is an angle), because
radians really are dimensionless when you do trigonometry. That asymmetry is
stated, tested, and confined to `Add`/`Sub`.

## 3. The named kinds

Package-level `var`s — a struct cannot be a Go `const` — so nobody writes
`Kind{l: 3}` by hand. They are read-only by contract: **never reassign one.**
Rebinding `Length` would change what every value in the process measures.

```go
var (
    Dimensionless = Kind{}
    Length        = Kind{l: 1}
    Area          = Kind{l: 2}
    Volume        = Kind{l: 3}
    Angle         = Kind{a: 1}
    Mass          = Kind{m: 1}
    Density       = Kind{l: -3, m: 1}   // M·L⁻³
    MomentOfInertia = Kind{l: 2, m: 1}  // M·L²
    SecondMomentOfArea = Kind{l: 4}     // L⁴
)
```

`Length`, `Angle` and `Dimensionless` keep their names and meaning, so existing
consumers compile unchanged.

**A kind with no name is still a kind.** `Kind{l: -1}` (an inverse length —
curvature) has no constant and needs none: it is produced by division, compares
correctly, and prints as `L⁻¹`. That is the whole point of the model.

## 4. Composition

```go
func (k Kind) Mul(o Kind) Kind   // exponents add
func (k Kind) Div(o Kind) Kind   // exponents subtract
func (k Kind) Pow(n int) Kind    // exponents scale

func (v Value) Mul(o Value) (Value, error)
func (v Value) Div(o Value) (Value, error)
```

`Value.Mul` multiplies magnitudes **in base units** and composes the kinds, so
`Millimeters(2).Mul(Millimeters(3))` is `6 mm²` — an `Area`, without anybody
enumerating that length times length is an area.

`Add`/`Sub` still require equal kinds (plus the angle/dimensionless carve-out).

### The result is finite, or it is an error

Every operation that **can** report an error yields a **finite** magnitude or that
error — never an `±Inf`, never a `NaN`. That is `Value.Add`, `Value.Sub`,
`Value.Mul`, `Value.Div`, `Value.In` and `Value.Convert`:

- A zero divisor is `ErrDivideByZero`. A unit's factor is positive and finite, so
  a magnitude is zero exactly when the quantity is: a divisor that is nonzero in
  its own unit is a real divisor, however small.
- A sum, difference, product, quotient or conversion that overflows to an
  infinity, or that is a `NaN` (`Inf × 0`), is `ErrNotFinite`. Two sentinels, so
  the error says which thing actually happened.

This is not fastidiousness. A result of a *named* kind carries a **registered**
symbol, so `+Inf mm^2` serializes exactly like a real area and nothing downstream
can tell it apart — an infinity that escaped with a nil error is a poisoned
document. `Add` is no different from `Mul` here: `Meters(math.MaxFloat64)` added
to itself is `+Inf m`, as persistable as any length. The same reason a unit's
factor must be positive and finite: `Define` panics on a zero, negative, infinite
or `NaN` factor, because every magnitude expressed in such a unit would convert
back to an infinity or a `NaN`.

**The known edge:** the operations with **no error to return** cannot enforce
this — `New`, `FromBase`, `Value.Scale` and `Value.Neg`. `New(math.Inf(1), Meter)`
is an infinite length, and `Scale` on a large enough factor overflows to one.
Their signatures are the constraint, not an oversight: a `Scale` returning
`(Value, error)` would put an error check on every multiplication by a plain
number. The limitation is stated in each doc comment, and the arithmetic that
*can* refuse — `Add`, `Sub`, `Mul`, `Div` — does refuse, so a non-finite value has
to be **constructed** on purpose rather than stumbled into.

### It is the *result* that must be finite — never an intermediate

`ErrNotFinite` fires when the **result** overflows, and only then. It is not a
licence to fail on an operand.

The trap is the **base magnitude**: `mag × unit.factor`, what `Value.Base()`
returns. It overflows for perfectly ordinary values. `Meters(1e307)` is a finite
magnitude in a built-in unit, and its base magnitude is `1e310 mm` — `+Inf`. An
operation that formed one as an intermediate would inherit that infinity and
report `ErrNotFinite` for a result that is representable, or — where there is no
error channel — return a wrong answer:

```go
units.Meters(1e307).In(units.Meter)                   // 1e307, not an error
units.Meters(1e307).Div(units.Meters(1e307))          // Scalar(1)
units.Meters(1e307).Mul(units.Millimeters(1e-300))    // 1e10 mm²
units.Meters(1e307).Equal(units.Meters(1e307), 1e-9)  // true
```

So **no operation forms a base magnitude.** Every one routes its arithmetic
through the four helpers in `value.go` — `rescale`, `product`, `quotient`, `sum` —
which split their operands with `math.Frexp`, combine the mantissas (a bounded
handful, so their product can neither overflow nor underflow, and every intermediate
keeps its full 53 bits), sum the binary exponents as `int`s, and put the scale back
with `math.Ldexp`. A new operation gets this by using the helpers; reaching for
`Base()` instead is how the bug comes back.

**An operation is one helper, never a composition of them.** `Add` and `Sub` are the
case that makes this a rule rather than a style note. A sum written as *rescale each
operand into the result's unit, then add* is two operations, and each can be
faultless on its own while the pair is not: the rescales round to 53 bits, and the
addition then cancels bits that are already gone. Two operands whose factors are
600 decades apart —

```go
tiny := units.Define("tiny", units.Length, 1e-300)  // and huge, 1e300
a := units.New(-math.MaxFloat64, tiny)
b := units.New(1.7976931348623157e-292, huge)       // all but −a
a.Add(b)  // 6.531456099116113e+291 tiny — the true sum, not the 0 two roundings leave
b.Add(a)  // 6.53145609911611e-309 huge — the same quantity, a subnormal, rounded once
```

— rescale into two 53-bit numbers that annihilate, and a sum that float64 holds
without difficulty comes back as zero, with a nil error. So `sum` is the whole
addition: it decides finiteness and rounding on the **true sum**, from the operands'
own magnitudes and factors, and rounds once.

**The range is free.** The exponent split is *all* the helpers do: the mantissas
are combined in the same order, and grouped the same way, as the plain
`mag × from ÷ to` and `(a × af) × (b × bf)` would combine the operands, so each
rounds exactly where the plain expression rounds and no everyday conversion pays
an ulp for the extra range. `25.4 mm` is exactly `1 in`, and `1000 g/cm³` exactly
`1e6 kg/m³`. Factors that cancel — the same factor above and below — cancel
*exactly*, so a value divided by itself is exact, and `rescale` returns the
magnitude untouched when the two factors are the same, so a conversion into a
value's own unit is the identity.

**But a rounded mantissa cannot decide a boundary, so at the ends of the range the
helpers do not ask it to.** Every mantissa the split path combines is rounded to 53
bits while its exponent is still carried separately, and that is exactly the
information a boundary needs:

- **The top.** A mantissa rounded with exponent room to spare cannot overflow, so it
  cannot report an overflow. `Ldexp` then scales it to a finite `MaxFloat64`, and a
  product whose true value is *past* the last float64 comes back as a real quantity
  with a nil error — the poisoned document the finiteness rule exists to prevent.
  Its mirror: a quotient mantissa that rounds **up** over the boundary scales to an
  `+Inf`, and `ErrNotFinite` is returned for a true value float64 holds perfectly
  well.
- **The bottom.** `Ldexp` lands the already-rounded mantissa in the subnormal range,
  where a float64 has fewer bits to land on, and rounds it a **second** time. Double
  rounded, the helper is *worse* than the plain expression it replaced, precisely
  where the plain expression still works and rounds once.

So where the assembled binary exponent runs past `±1000` (`atTheEnds`) — twenty-odd
binades clear of every boundary, and nowhere an ordinary result lands — the helpers
redo the arithmetic in **exact rationals** and round **once**. A float64 magnitude
and a float64 factor are exact rationals, so `math/big` holds the true quantity
whatever its size, and rounding it to float64 once is by definition the correctly
rounded result: an infinity exactly when the true result is past the last float64,
the nearest float64 when it is not, and the nearest *subnormal* at the bottom. No
float64 is nearer, so it is never worse than the plain expression either.

```go
units.Scalar(1e-300).Mul(units.New(math.MaxFloat64, huge))  // ErrNotFinite — the true product is past the last float64
units.Scalar(1.25).Div(units.Centimeters(1e307))            // 1.25e-308, not 1.2499999999999996e-308
units.Meters(5e-324).Mul(units.Grams(-2.5))                 // -1.5e-323, not -1e-323
```

`sum` reads the same rule off its own shape rather than off an exponent, because a
cancellation puts the ends of the range nowhere near the operands: the bits it loses
are lost in the middle. The addition itself is safe — an IEEE addition of two exact
terms is correctly rounded by definition, an infinity exactly when the true sum
overflows and the nearest float64, subnormals included, when it does not — so the
fast path is kept exactly where the terms **are** exact: where both operands are
already carried in the result's unit and neither is rescaled. Wherever a rescale
would round an operand on its way in, that rounding is one the sum has not
authorised, and the arithmetic is redone in exact rationals instead. `Add` and `Sub`
are therefore **correctly rounded**, always: the float64 nearest the true `a ± b`, or
`ErrNotFinite` when no float64 is near enough.

An infinity or a `NaN` operand has no exact rational and keeps the fast path, where
it propagates as it would through the plain arithmetic — `Inf × 0` is still a `NaN`,
and still `ErrNotFinite`.

The test suite pins all of this down against a `big.Rat` oracle. Accuracy: every
conversion, product, quotient and sum — over everyday magnitudes, and over the
extremes (subnormals, `1e307`, `MaxFloat64`, `Define`d factors of `1e±300`) — must
land no further from the true result than the plain expression it replaced, and
within two ulps of it. Two ulps and not half of one because `(a × af) × (b × bf)`
rounds three times whoever computes it; what is gated is that the helpers round where
the plain expression rounds, and no more often. A relative tolerance — even `1e-9` —
is some `10⁷` ulps and would not see a regression of this class. `Add` and `Sub` are
gated harder still, bit for bit against the correctly rounded true sum, and swept
over the operand that most nearly **annihilates** the first — its neighbours and its
half besides — for every pair of units of a kind, so that the whole significand of
the result is what the cancellation left.

Finiteness the same oracle decides outright, and there is **no ambiguous band** to
excuse a wrong answer near `MaxFloat64`, or a cancellation too fine to judge: the
exact rational either exceeds the last float64 or it does not, and it is the true sum
whether or not the operands were near-equal. The overflow boundary is swept densely —
both signs, the last three float64s, the factors either side of 1 that carry them
over the end — and every point asserts `ErrNotFinite` when the true result overflows,
and the correctly rounded value when it does not.

`Value.Base()` stays as it is: it is an **accessor**, not an operation. A value
whose base magnitude genuinely overflows reports `+Inf` there, honestly, and one
whose base magnitude underflows reports `0`. **No operation reads it** — not even
`Div`'s zero-divisor guard, which reads the divisor's own magnitude: a base
magnitude would report a divide-by-zero for an ordinary small divisor whose
product with its factor underflows, and refuse a quotient that is perfectly
representable. The one place a base magnitude is still formed outside the accessor
is `System.In`'s error path, where the infinity is the answer: a magnitude the
presentation unit cannot hold comes back as the infinity it is, never as a finite
number in the wrong unit.

`Equal` is the sharpest case, because it has no error channel at all:
`|Inf − Inf|` is `NaN`, and `NaN <= tol` is `false`, so a value would not be equal
to itself at any tolerance. It subtracts in a unit common to both operands and
rescales only the difference.

That common unit is chosen by a property of the **pair** — the larger of the two
factors — and never by which operand is the receiver. An equality predicate whose
answer depends on the order of its operands is broken whichever answer it gives:
rescaling only the *other* operand rounds one side and not the other, so
`a.Equal(b, 0)` and `b.Equal(a, 0)` could disagree about two renderings of one
quantity. Rescaling **both** operands into the same unit makes the swap negate the
difference and leave its magnitude alone, and equal factors leave nothing to
choose — both rescales are the identity, and the final rescale is by that same
factor whichever unit is named. The suite sweeps every built-in pair of a kind, at
every tolerance including zero, for exactly this.

### An overflowed exponent is sticky

Exponents **saturate**, never wrap: `Pow` answers an out-of-range multiplier with
the endpoint the signs point at — even `Area.Pow(math.MinInt64)` — rather than
letting an `int64` product wrap into a plausible-looking named kind.

Saturating is not enough on its own. A clamped exponent is a **lie about the
number**: `L¹²⁷` standing in for an exponent of some 9·10¹⁸ is not `L¹²⁷`, and
arithmetic can walk it back out of the endpoint —

```go
overflowed := units.Length.Pow(math.MaxInt64)  // saturates to the L¹²⁷ endpoint
overflowed.Div(units.Length.Pow(126))          // …and back out again: Length?
```

— handing a CAD consumer an overflowed quantity dressed as an ordinary length,
with a registered symbol and a nil error. So the saturation is **recorded**, on
the kind, and it is sticky:

```go
type Kind struct {
    l, m, a int8
    ovf     bool // set when an exponent saturated; never cleared
}
```

`Kind` stays comparable and its zero value stays `Dimensionless`. `Mul`, `Div`
and `Pow` propagate `ovf` from either operand, so nothing composes an overflowed
kind back into an ordinary one — not even `sat.Div(sat)`, which has zero
exponents but is still overflow. `Kind.Overflowed()` reports it, and every
downstream answer follows from the flag alone:

- it is **never equal to a named kind** (that is the whole point);
- `String()` is `"overflowed"` — the exponents are stand-ins and printing them
  would state a dimension the kind does not have;
- `BaseUnit` reports no base unit, and the synthetic unit a value of it carries
  is `[overflow]` — ASCII, in the reserved bracketed namespace, and resolvable by
  nothing;
- `System.UnitFor` hands back that same unit, of that same kind, so presentation
  cannot turn it into a length or a bare number either.

An out-of-range composition is a programming error, and it must look like one for
as long as it lives.

## 5. Units for derived kinds

A `Unit` is unchanged: symbol + kind + factor to that kind's base. Derived kinds
need base units and a starter set:

| Kind | Base | Built-ins |
|---|---|---|
| Dimensionless | `One` (the empty symbol) | — |
| Length | mm | mm, cm, m, in, ft, thou |
| Area | mm² | mm², cm², m², in² |
| Volume | mm³ | mm³, cm³ (= mL), m³, in³, L |
| Mass | kg | kg, g, lb |
| Density | kg/mm³ | kg/m³, g/cm³ |
| MomentOfInertia | kg*mm^2 | — |
| SecondMomentOfArea | mm^4 | — |
| Angle | rad | rad, deg |

**Every named kind has a registered base unit.** `BaseUnit(k)` is a lookup rather
than a switch, and returns `(Unit, bool)` — an unnamed kind like `L⁻¹` has no
base unit registered, and fabricating one would be a lie. The `bool` is the
honest part: the function has an answer it cannot always give.

Unit symbols are ASCII, with a caret for an exponent. `Kind.String()` — Unicode
superscripts, `L⁻¹`, and English names for named kinds — is **display text and
never a unit symbol**.

The registry behind `Define`/`Lookup` is guarded by a `sync.RWMutex`: an
application may register its own units from any goroutine while others resolve
symbols.

### A presentation unit never changes the kind

`System.UnitFor(k)` returns a unit that measures `k` — always. For a length or an
angle it is the system's configured unit, for any other named kind that kind's
base unit, and for an unnamed kind the **synthetic factor-1 unit of that kind**.

The configured unit is **validated**, not trusted. `System` has exported fields
and a usable zero value — `units.System{Length: units.Meter}`, angle forgotten,
is an ordinary construction — and the zero `Unit` reads as `One`, which is
`Dimensionless`. So `UnitFor` checks that the configured field actually measures
the kind it is configured for, and falls through to the base unit when it does
not:

```go
case Length:
    if u := s.Length.normalize(); u.kind == Length {
        return u
    }
```

Without that check `System{}.UnitFor(Length)` would be `One`, and a 5 mm length
routed through it would come back **dimensionless** — then addable to an angle,
via `Add`'s carve-out, with a nil error the whole way.

It never falls back to `One`, which would be `Dimensionless`: routing a curvature
through a dimensionless "default" would rebuild it as a bare number, and `Add`'s
angle/dimensionless carve-out would then let it be added to an angle. A silent
cross-kind coercion, with a nil error the whole way, is precisely what this
library exists to prevent — so `Display` and `In` are kind-preserving by
construction, not by luck.

`System.In` returns a bare `float64` and has no error to report, so the **unit**
that number is in is the whole contract: it is always the unit `UnitFor` gives for
the value's kind. A magnitude that unit cannot hold — `Metric().In(Meters(1e307))`
is `1e310 mm` — comes back as the **infinity it is**. Returning the value's own
magnitude instead would be finite, in the wrong unit, and silent: the caller reads
millimetres and gets metres. `System.Display` has the same problem and the same
answer: a value it cannot carry in the presentation unit is returned **unchanged**,
in its own unit, rather than relabelled.

### Synthetic units for unnamed kinds

`Value.Mul`/`Value.Div` must carry their result somewhere, so a result of an
unnamed kind gets a **synthetic** unit of factor 1, whose symbol is that kind's
canonical form: ASCII, exponents in the fixed order length, mass, angle, wrapped
in brackets — `[L^-1]`, `[L^2*M]`. Two rules keep it honest:

- The synthetic unit is **not** in the registry: `Lookup` does not find it and
  `BaseUnit` still reports the kind as having none.
- The bracketed namespace is **reserved**. `Define` panics on any symbol opening
  with `[`, so an application cannot register a unit that a persisted synthetic
  symbol would later resolve to — which would deserialize a value as a *different
  kind*.

An overflowed kind gets `[overflow]` from the same namespace: its exponents are
clamped stand-ins, so there is no dimension form to write, and the symbol says so
rather than pretending to `[L^127]`.

**A value of an unnamed kind is a transient intermediate and MUST NOT be
persisted.** Compose it back into a named kind first. Every kind a consumer
actually measures — length, area, volume, mass, density, moment of inertia,
second moment of area — is named and has a registered base unit, so this bites
only genuine intermediates such as `L⁻¹`.

## 6. What this breaks

Almost nothing, because of one lucky fact: **`Kind` is never serialized.** JSON
stores the unit *symbol* (`"mm"`) and re-derives the kind through `Lookup`, so
changing `Kind`'s underlying type from `int` to a struct changes **no wire
format** and no persisted document. That is also why only a value of a *named*
kind is persistable: a synthetic `[L^-1]` symbol has nothing to re-derive from.

The compile-time breakage is small and mechanical:

- `return 0, err` → `return Kind{}, err` (kind-returning functions).
- `switch k { case Length: ... }` still works — `Kind` is comparable.
- `BaseUnit` gains a `bool`.
- In `sketch/param`, the `'*'` and `'/'` cases that today return *"no compound
  unit"* errors are **deleted** and replaced with `ka.Mul(kb)` / `ka.Div(kb)`.
  That is a strict capability gain: expressions that were rejected now evaluate.

`sketch` is pre-1.0 with no tags, and its only consumer is `decad`, which has no
code yet. This is the cheapest this migration will ever be.

## 7. Non-goals

Time, current, temperature and the rest of SI — this is a geometry library.
Unit *parsing* from arbitrary strings (`Lookup` by symbol is for deserialization,
not a expression parser; `sketch/param` owns expressions). Automatic unit
*selection* for display (that is `System`'s job). Compound symbol synthesis for
unnamed kinds beyond a readable `L²·M` form.
