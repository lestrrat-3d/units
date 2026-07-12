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
// A [Kind] is a vector of dimension exponents — length, mass and angle — so
// kinds compose instead of being enumerated. Multiplying two values multiplies
// their magnitudes in base units and adds the exponents:
//
//	a, _ := units.Millimeters(2).Mul(units.Millimeters(3)) // 6 mm², an Area
//	l, _ := units.Liters(1).Div(units.SquareMeters(1))     // a Length
//
// The named kinds are [Dimensionless], [Length], [Area], [Volume], [Angle],
// [Mass], [Density], [MomentOfInertia] and [SecondMomentOfArea]. A kind need
// not be named to be usable: dividing a dimensionless value by a length yields
// an inverse length, which compares and prints ("L⁻¹") like any other kind,
// though [BaseUnit] reports that no base unit is registered for it.
//
// Base units are the millimetre ([Length]), the square millimetre ([Area]), the
// cubic millimetre ([Volume]), the kilogram ([Mass]), the kilogram per cubic
// millimetre ([Density]) and the radian ([Angle]); every unit stores its
// conversion factor to its kind's base. Unit symbols are ASCII and write an
// exponent with a caret: "mm^2", "in^3", "kg/m^3".
//
// An angle is a dimension of its own even though a radian is physically a ratio
// of two lengths, so that a bare number can never be mistaken for an angle. The
// single carve-out is that [Value.Add] and [Value.Sub] accept an angle and a
// dimensionless value together, because theta + pi/2 is an angle.
package units
