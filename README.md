# units

A small, self-contained **units-of-measure** library for Go: a `Unit` is a
symbol, a `Kind` and a conversion factor; a `Value` pairs a magnitude with the
unit it is expressed in.

```go
import "github.com/lestrrat-3d/units"

w := units.Millimeters(100)
in, err := w.In(units.Inch)   // 3.937...
fmt.Println(w)                // "100 mm"

_, err = w.Convert(units.Degree)  // error: a length is not an angle

a, err := units.Millimeters(2).Mul(units.Millimeters(3))  // "6 mm^2", an Area
v, err := a.Mul(units.Millimeters(4))                     // "24 mm^3", a Volume
```

## Why

A magnitude stripped of its unit is a bug waiting to happen. A bare `float64` is
how `2` silently becomes 2 *radians* when 2 *degrees* was meant — the classic
CAD-API trap, where every number crossing the boundary is in some ambient
internal unit the caller has to remember.

Anything crossing an API boundary should be a `Value`: a number **and** its
unit, together, checked. Kinds never coerce — asking for a length as an angle is
an `error`, not a silent reinterpretation.

## Scope

The package holds **quantities and the conversions between them**, and nothing
else. It carries no document state, knows nothing of any application layered on
top of it, and depends only on the standard library.

- **Units are typed, never stringly.** You name a unit through a `Unit` constant
  (`Millimeter`, `Inch`, `Degree`), not by passing `"mm"` around. `Lookup` exists
  for deserialization; it is not the normal way to build a value.
- **Kinds are dimensions, not an enumeration.** A `Kind` is a vector of exponents
  over length, mass and angle, so kinds compose: `Mul` adds them and `Div`
  subtracts them, and `Area`, `Volume`, `Density` (M·L⁻³), `MomentOfInertia`
  (M·L²) and `SecondMomentOfArea` (L⁴) fall out for free. A kind nobody named —
  an inverse length, say — is still a kind: it compares and prints (`L⁻¹`),
  though no base unit is registered for it.
- **An angle is its own dimension**, even though a radian is physically a ratio
  of two lengths, so a bare number can never pass as an angle. The one carve-out
  is that `Add`/`Sub` accept an angle and a dimensionless value together.
- **Every named kind has a base unit**: the millimetre (`Length`), the square
  millimetre (`Area`), the cubic millimetre (`Volume`), the kilogram (`Mass`),
  the kilogram per cubic millimetre (`Density`), the kilogram square millimetre
  (`MomentOfInertia`), the quartic millimetre (`SecondMomentOfArea`) and the
  radian (`Angle`). Every unit stores its factor to its kind's base. Symbols are
  ASCII, with a caret for an exponent: `mm^2`, `in^3`, `kg/m^3`.
- **A value of an unnamed kind is transient.** It carries a synthetic,
  unregistered unit whose symbol is bracketed (`[L^-1]`) — `Lookup` will not
  resolve it, so it must not be persisted. Compose it back into a named kind
  first.
- **Extensible.** `Define` registers a new unit against a kind; a symbol may not
  be redefined, and symbols opening with `[` are reserved for the library.

## License

This project is **source-available**, and is licensed under the
[PolyForm Noncommercial License 1.0.0](LICENSE).

* **Noncommercial use is free.** Individuals, hobby and personal projects,
  research, education, nonprofits, and government may use, modify, and
  redistribute it at no cost, subject to the license terms.
* **Commercial / business use requires a separate license.** Any use by or for
  a business, or for commercial advantage, is not permitted under the
  noncommercial license. To obtain a commercial license, reach out on Bluesky
  at [@lestrrat.bsky.social](https://bsky.app/profile/lestrrat.bsky.social).

### Contributions

This repository does **not** accept external pull requests.
