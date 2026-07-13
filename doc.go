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
// presenting base-unit quantities back in a chosen unit. Its default unit for a
// kind always measures that kind, so presenting a value never changes what it
// measures: a System field left unset, or holding a unit of the wrong kind, is
// ignored in favour of the kind's base unit.
//
// The zero [Value] is 0 of the dimensionless unit [One], and arithmetic on it
// behaves accordingly.
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
// Every operation that can report an error yields a finite result or that error,
// never an infinity and never a NaN: a zero divisor is [ErrDivideByZero], and a
// sum, difference, product, quotient or conversion that
// overflows or is a NaN is [ErrNotFinite]. That covers [Value.Add], [Value.Sub],
// [Value.Mul], [Value.Div], [Value.In] and [Value.Convert]. The operations with
// no error to report — [New], [FromBase], [Value.Scale] and [Value.Neg] — do not
// check: hand them an infinity, or scale past the float64 range, and a non-finite
// Value comes back.
//
// The error is about the result, never an intermediate. A base magnitude — a
// magnitude times its unit's factor, what [Value.Base] returns — overflows for
// ordinary values ([Meters](1e307) is 1e310 mm, which no float64 holds) and
// underflows for others ([Grams](1e-322) is 1e-325 kg, which no float64 holds
// either). No operation forms one, so [Meters](1e307) still converts to metres,
// still divides by itself to 1, and still equals itself, and [Grams](1e-322) is
// an ordinary divisor rather than a zero one. [Value.Base] is an accessor, not an
// operation, and reports the infinity — or the zero — honestly.
//
// The range is not paid for in accuracy: the arithmetic rounds where the plain
// expression rounds and never once more, down among the subnormals as anywhere
// else. [Scalar](1.25) divided by [Centimeters](1e307) is 1.25e-308. [Value.Add]
// and [Value.Sub] round once and no sooner — they are decided on the exact value of
// the sum, so a difference that all but cancels keeps what the cancellation leaves,
// however far apart the two units' factors are.
//
// [System.In] presents a magnitude in the system's unit for the value's kind and
// has no error to report, so a magnitude that unit cannot hold comes back as the
// infinity it is — never as a finite number that would be read as a quantity it
// is not.
//
// The named kinds are [Dimensionless], [Length], [Area], [Volume], [Angle],
// [Mass], [Density], [MomentOfInertia] and [SecondMomentOfArea]. Every one of
// them has a registered base unit. A kind need not be named to be usable:
// dividing a dimensionless value by a length yields an inverse length, which
// compares and prints ("L⁻¹") like any other kind, though [BaseUnit] reports
// that no base unit is registered for it.
//
// A dimension exponent is an int8. A composition that runs past that range —
// [Length].Pow(math.MaxInt64) — saturates at the endpoint and marks the kind
// overflowed ([Kind.Overflowed]). The mark is sticky: [Kind.Mul], [Kind.Div] and
// [Kind.Pow] propagate it, so an overflowed kind never composes back into an
// ordinary one. It equals no named kind, prints as "overflowed", has no base
// unit, and carries the reserved synthetic symbol "[overflow]" — a saturated
// exponent is a lie about the number, and it must not be able to pass for a
// plausible kind.
//
// Base units are the millimetre ([Length]), the square millimetre ([Area]), the
// cubic millimetre ([Volume]), the kilogram ([Mass]), the kilogram per cubic
// millimetre ([Density]), the kilogram square millimetre ([MomentOfInertia]),
// the quartic millimetre ([SecondMomentOfArea]) and the radian ([Angle]); every
// unit stores its conversion factor to its kind's base. Unit symbols are
// printable ASCII without the space and write an exponent with a caret: "mm^2",
// "in^3", "kg/m^3". A unit whose conventional symbol is not ASCII is registered
// under an ASCII spelling, as the built-in "deg" is for the degree sign.
//
// # Unnamed kinds are transient
//
// A [Value] of an unnamed kind carries a synthetic, unregistered unit whose
// symbol is bracketed ("[L^-1]"): [Lookup] does not resolve it, and [Define]
// panics on any symbol opening with "[", so the bracketed namespace stays the
// library's. Such a value is a transient intermediate and MUST NOT be persisted;
// compose it back into a named kind — every kind an application actually
// measures has one — before writing it anywhere.
//
// An angle is a dimension of its own even though a radian is physically a ratio
// of two lengths, so that a bare number can never be mistaken for an angle. The
// single carve-out is that [Value.Add] and [Value.Sub] accept an angle and a
// dimensionless value together, because theta + pi/2 is an angle.
//
// # The text form
//
// A [Value] serializes as text: [Value.MarshalText] and [Value.UnmarshalText]
// implement [encoding.TextMarshaler] and [encoding.TextUnmarshaler], so
// encoding/json — and every other text-based encoder — carries a quantity as
// "<magnitude> <symbol>":
//
//	type Step struct {
//		Distance units.Value `json:"distance"`
//	}
//	b, _ := json.Marshal(Step{Distance: units.Millimeters(10)}) // {"distance":"10 mm"}
//
// The magnitude is the shortest float64 rendering and the symbol is the unit's
// registered one, so the round trip is exact: an unmarshalled value is the
// marshalled one, the same unit and the same magnitude bit for bit, subnormals
// and MaxFloat64 included. A dimensionless value has no symbol to write ([One]'s
// is empty) and is the bare number ("1.5"); nothing else parses as one, since a
// trailing space, a doubled space or a token after the symbol is
// [ErrMalformedText].
//
// The symbol is resolved with [Lookup], and an unregistered one is
// [ErrUnknownUnit] — never a guess, never the kind's base unit, never a silently
// dimensionless value. A value that cannot be read back is not written: an
// unnamed kind is [ErrUnnamedKind], an overflowed one [ErrOverflowedKind], and a
// magnitude that is not finite [ErrNotFinite], which is also what a literal
// infinity, NaN or past-the-range magnitude in a document reads as.
//
// # Extending the unit set
//
// [Define] registers a new unit against a kind. A symbol must be printable
// ASCII without the space and may not be redefined, symbols opening with "["
// are reserved, and a unit's factor to its base must be positive and finite;
// each violation is a panic. The registry is guarded, so [Define], [Lookup] and
// [BaseUnit] may be called from multiple goroutines.
package units
