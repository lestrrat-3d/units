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

feed, err := units.MillimetersPerSecond(60).In(units.MillimeterPerMinute) // 3600
k, err := units.DegreesCelsius(210).ToRatio(units.Kelvin)                 // "483.15 K"
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
  over length, mass, angle, time and temperature, so kinds compose: `Mul` adds
  them and `Div` subtracts them, and `Area`, `Volume`, `Density` (M·L⁻³),
  `MomentOfInertia` (M·L²), `SecondMomentOfArea` (L⁴), `Velocity` (L·T⁻¹) and
  `Acceleration` (L·T⁻²) fall out for free. A kind nobody named — an inverse
  length, say — is still a kind: it compares and prints (`L⁻¹`), though no base
  unit is registered for it.
- **A scale whose zero moved is a different type.** `Celsius` and `Fahrenheit` are
  `AffineUnit`s carrying an `AffineValue`, because their zero is not the kelvin's.
  An `AffineValue` converts, compares, prints and persists, and has no `Add`,
  `Sub`, `Mul`, `Div`, `Scale` or `Neg` at all — not a version that returns an
  error, no version — because `20 °C × 2` is not `40 °C` and two absolute
  temperatures do not add. `units.New(20, units.Celsius)` does not compile. Cross
  over with `ToRatio` and the arithmetic is there and correct. Every `Unit` is a
  pure ratio, so every method on `Value` is meaningful for every `Value`.
- **An angle is its own dimension**, even though a radian is physically a ratio
  of two lengths, so a bare number can never pass as an angle. The one carve-out
  is that `Add`/`Sub` accept an angle and a dimensionless value together.
- **An exponent that overflows says so, and keeps saying so.** Exponents are
  `int8`; a composition that runs off the end saturates and marks the kind
  **overflowed** (`Kind.Overflowed()`). The mark is sticky: `Mul`, `Div` and `Pow`
  propagate it, so an overflowed kind never equals a named kind, prints as
  `overflowed`, has no base unit, and carries the reserved `[overflow]` symbol.
  A saturated exponent is a lie about the number; without the mark, dividing an
  overflowed `L¹²⁷` by `L¹²⁶` would hand back a perfectly plausible `Length`.
- **Every named kind has a base unit**: the millimetre (`Length`), the square
  millimetre (`Area`), the cubic millimetre (`Volume`), the kilogram (`Mass`),
  the kilogram per cubic millimetre (`Density`), the kilogram square millimetre
  (`MomentOfInertia`), the quartic millimetre (`SecondMomentOfArea`), the
  radian (`Angle`), the second (`Time`), the millimetre per second (`Velocity`),
  the millimetre per second squared (`Acceleration`) and the kelvin
  (`Temperature`). Every unit stores its factor to its kind's base. Symbols are
  printable ASCII without the space, with a caret for an exponent: `mm^2`,
  `in^3`, `kg/m^3`. A unit whose conventional symbol is not ASCII (µm, °, Å)
  registers under an ASCII spelling (`um`, `deg`, `angstrom`), as the built-ins
  do.
- **A value of an unnamed kind is transient.** It carries a synthetic,
  unregistered unit whose symbol is bracketed (`[L^-1]`) — `Lookup` will not
  resolve it, so it must not be persisted. Compose it back into a named kind
  first. It is still that kind everywhere else: a `System`'s default unit for it
  measures it, so presenting a value never changes what it measures. A `System`
  field left unset, or holding a unit of the wrong kind, is ignored in favour of
  the kind's base unit — the zero `System` presents every kind as itself.
- **A result is finite, or it is an error.** `Add`, `Sub`, `Mul`, `Div`, `In` and
  `Convert` never hand back an `+Inf` or a `NaN` with a nil error: a zero divisor
  is `ErrDivideByZero`, and an overflowing or NaN result is `ErrNotFinite`. The
  operations that have no error to return — `New`, `FromBase`, `Scale`, `Neg` —
  cannot check, and do not.
- **It is the result that must be finite, never an intermediate.** A base
  magnitude (`Value.Base()`, the magnitude in the kind's base unit) overflows for
  ordinary values — `Meters(1e307)` is `1e310 mm` — and underflows for others —
  `Grams(1e-322)` is `1e-325 kg`. No operation forms one. So `Meters(1e307)`
  converts to metres, divides by itself to `1`, multiplies by `Millimeters(1e-300)`
  to `1e10 mm²`, and equals itself, and `Grams(1e-322)` is an ordinary divisor
  rather than a zero one. `Base()` is an accessor and reports that infinity — or
  that zero — honestly; `System.In`, which answers in the system's unit for the
  kind, returns the infinity rather than a finite number in another unit.
- **The range costs no accuracy.** Conversions are exact where the arithmetic is:
  `25.4 mm` is exactly `1 in`, `1000 g/cm³` exactly `1e6 kg/m³`, a value in its own
  unit is its own magnitude, and a value divided by itself is exactly `1`. The
  arithmetic rounds where the plain expression rounds and never once more —
  subnormal results included, where a second rounding would be worse than the
  plain expression: `Scalar(1.25).Div(Centimeters(1e307))` is `1.25e-308`.
- **A quantity serializes with its unit.** `Value` implements
  `encoding.TextMarshaler`/`TextUnmarshaler`, so `encoding/json` — and any other
  text-based encoder — writes it as `"<magnitude> <symbol>"`: `"10 mm"`,
  `"7850 kg/m^3"`, `"90 deg"`. A dimensionless value is the bare number (`"1.5"`),
  since `One`'s symbol is empty. The round trip is **exact** — the same unit, and
  the same magnitude bit for bit — and the symbol is resolved through `Lookup`: an
  unregistered symbol is an error, never a guess and never a silently dimensionless
  value. What cannot be read back is not written: a value of an unnamed kind, of an
  overflowed kind, or with a non-finite magnitude has **no text form** and is an
  error rather than `{}` with a nil error.

  ```go
  type Step struct{ Distance units.Value `json:"distance"` }
  b, err := json.Marshal(Step{Distance: units.Millimeters(10)})  // {"distance":"10 mm"}
  ```
- **The zero `Value` is 0 of `One`**, so a `Value` declared with `var` behaves as
  a plain 0 in every operation.
- **Extensible.** `Define` registers a new unit against a kind; a symbol must be
  printable ASCII without the space and may not be redefined, symbols opening
  with `[` are reserved for the library, and the factor to the kind's base must
  be positive and finite. `Define`, `Lookup` and `BaseUnit` are safe to call
  from multiple goroutines.

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
