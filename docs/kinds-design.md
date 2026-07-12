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

Constants, so nobody writes `Kind{l: 3}` by hand:

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
Division by a zero-magnitude value is `ErrDivideByZero`, not `+Inf`.

## 5. Units for derived kinds

A `Unit` is unchanged: symbol + kind + factor to that kind's base. Derived kinds
need base units and a starter set:

| Kind | Base | Built-ins |
|---|---|---|
| Length | mm | mm, cm, m, in, ft, thou |
| Area | mm² | mm², cm², m², in² |
| Volume | mm³ | mm³, cm³ (= mL), m³, in³, L |
| Mass | kg | kg, g, lb |
| Density | kg/mm³ | kg/m³, g/cm³ |
| Angle | rad | rad, deg |

`BaseUnit(k)` becomes a lookup rather than a switch, and returns `(Unit, bool)` —
an unnamed kind like `L⁻¹` has no base unit registered, and fabricating one would
be a lie. **This is a signature change** (today it returns a bare `Unit`), and it
is the honest one: the function now has an answer it cannot always give.

## 6. What this breaks

Almost nothing, because of one lucky fact: **`Kind` is never serialized.** JSON
stores the unit *symbol* (`"mm"`) and re-derives the kind through `Lookup`, so
changing `Kind`'s underlying type from `int` to a struct changes **no wire
format** and no persisted document.

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
