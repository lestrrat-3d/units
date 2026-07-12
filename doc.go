// Package units is a small, self-contained units-of-measure library: a [Unit]
// is a symbol, a [Kind] and a conversion factor; a [Value] pairs a magnitude
// with the unit it is expressed in.
//
// Units are explicitly typed: you refer to them through the package's [Unit]
// constants (e.g. [Millimeter], [Inch], [Degree]) rather than by string. A
// [Value] knows how to convert itself to any compatible unit, and refuses to
// convert across kinds:
//
//	w := units.Millimeters(100)
//	in, _ := w.In(units.Inch) // 3.937...
//	fmt.Println(w)            // "100 mm"
//
// A [System] records the current default length and angle units, used for
// presenting base-unit quantities back in a chosen unit.
//
// # Scope
//
// The package holds quantities and the conversions between them, and nothing
// else. It carries no document state and knows nothing of any application
// layered on top of it. It depends only on the standard library.
//
// A magnitude stripped of its unit is a bug waiting to happen: a bare float64
// is how "2" silently becomes 2 radians when 2 degrees was meant. Anything
// crossing an API boundary should be a [Value], not a number and a convention.
//
// # Kinds
//
// [Dimensionless] covers ratios, counts and multipliers. Base units are the
// millimetre for [Length] and the radian for [Angle]; every unit stores its
// conversion factor to its kind's base.
package units
