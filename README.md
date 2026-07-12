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
- **Base units are the millimetre (`Length`) and the radian (`Angle`).** Every
  unit stores its factor to its kind's base.
- **Extensible.** `Define` registers a new unit against an existing kind.

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
