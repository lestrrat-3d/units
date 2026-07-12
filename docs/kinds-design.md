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
// Kind is comparable: two kinds are equal exactly when their exponents match.
type Kind struct {
    l, m, a int8 // length, mass, angle
}
```

Three base dimensions, because three is what this domain needs:

| | Why |
|---|---|
| **Length** (`l`) | the base of all geometry: length, area, volume, inertia |
| **Mass** (`m`) | mass properties: mass, density, moment of inertia |
| **Angle** (`a`) | **see below — this one is a judgment call, not physics** |

There is no time, current, or temperature. This is a geometry library; a
quantity that needs seconds does not belong in it. Exponents are `int8` —
`L⁴` is as exotic as it gets, and the whole `Kind` stays 3 bytes and comparable
with `==`.

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

`Value.Mul` and `Value.Div` both yield a **finite** magnitude or an error — never
an `±Inf`, never a `NaN`:

- A zero base magnitude in the divisor is `ErrDivideByZero`.
- A product or quotient that overflows to an infinity, or that is a `NaN`
  (`Inf × 0`), is `ErrNotFinite`. Two sentinels, so the error says which thing
  actually happened.

This is not fastidiousness. A result of a *named* kind carries a **registered**
symbol, so `+Inf mm^2` serializes exactly like a real area and nothing downstream
can tell it apart — an infinity that escaped with a nil error is a poisoned
document. The same reason a unit's factor must be positive and finite: `Define`
panics on a zero, negative, infinite or `NaN` factor, because every magnitude
expressed in such a unit would convert back to an infinity or a `NaN`.

Exponents **saturate**, never wrap. `Pow` narrows its multiplier into the `int8`
range before scaling, so even `Area.Pow(math.MinInt64)` lands on an endpoint of
the right sign rather than wrapping into a plausible-looking named kind. An
out-of-range composition is a programming error, and it must look like one.

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
It never falls back to `One`, which would be `Dimensionless`: routing a curvature
through a dimensionless "default" would rebuild it as a bare number, and `Add`'s
angle/dimensionless carve-out would then let it be added to an angle. A silent
cross-kind coercion, with a nil error the whole way, is precisely what this
library exists to prevent — so `Display` and `In` are kind-preserving by
construction, not by luck.

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
