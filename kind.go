package units

import (
	"math"
	"strconv"
	"strings"
)

// Kind is the physical dimension of a quantity: the exponents of the base
// dimensions it is built from. The base dimensions are length, mass and angle;
// there is no time, current or temperature, because this is a geometry library.
//
// Kind is comparable: two kinds are equal exactly when their exponents match.
// The zero value is [Dimensionless]. A [Value] never converts across kinds: a
// length is not an angle, and asking for one as the other is an error rather
// than a coercion.
//
// Angle is tracked as its own dimension even though a radian is physically a
// ratio of two lengths. That deviation is deliberate: it keeps [Radians](2) and
// [Scalar](2) from being the same kind. The one carve-out is in [Value.Add] and
// [Value.Sub], where an angle may be added to a dimensionless value.
//
// A kind need not have a name. Kinds produced by [Kind.Div] — an inverse length
// (curvature), say — compare correctly and print readably even though no
// constant names them and no base unit is registered for them.
//
// Exponents are stored as int8, so each ranges over [-128, 127]. Geometry never
// approaches that: L⁴ (a second moment of area) is as exotic as it gets. If a
// composition would exceed the range it is clamped to the endpoint, which is a
// programming error rather than a meaningful kind.
type Kind struct {
	l, m, a int8 // exponents of length, mass and angle
}

// Dimensionless, and the kinds below it, are the named kinds: the dimensions
// this library has base units for.
var (
	// Dimensionless is a pure number (ratios, counts, multipliers).
	Dimensionless = Kind{}
	// Length is a linear distance; its base unit is the millimetre.
	Length = Kind{l: 1}
	// Area is L²; its base unit is the square millimetre.
	Area = Kind{l: 2}
	// Volume is L³; its base unit is the cubic millimetre.
	Volume = Kind{l: 3}
	// Angle is a planar angle; its base unit is the radian.
	Angle = Kind{a: 1}
	// Mass is M; its base unit is the kilogram.
	Mass = Kind{m: 1}
	// Density is M·L⁻³; its base unit is the kilogram per cubic millimetre.
	Density = Kind{l: -3, m: 1}
	// MomentOfInertia is M·L².
	MomentOfInertia = Kind{l: 2, m: 1}
	// SecondMomentOfArea is L⁴.
	SecondMomentOfArea = Kind{l: 4}
)

// kindNames holds the human-readable name of every named kind. A kind absent
// from it is unnamed and prints in exponent form.
var kindNames = map[Kind]string{
	Dimensionless:      "dimensionless",
	Length:             "length",
	Area:               "area",
	Volume:             "volume",
	Angle:              "angle",
	Mass:               "mass",
	Density:            "density",
	MomentOfInertia:    "moment of inertia",
	SecondMomentOfArea: "second moment of area",
}

// Mul returns the kind of a product: the exponents of k and o added.
func (k Kind) Mul(o Kind) Kind {
	return Kind{
		l: clampExp(int64(k.l) + int64(o.l)),
		m: clampExp(int64(k.m) + int64(o.m)),
		a: clampExp(int64(k.a) + int64(o.a)),
	}
}

// Div returns the kind of a quotient: the exponents of o subtracted from those
// of k.
func (k Kind) Div(o Kind) Kind {
	return Kind{
		l: clampExp(int64(k.l) - int64(o.l)),
		m: clampExp(int64(k.m) - int64(o.m)),
		a: clampExp(int64(k.a) - int64(o.a)),
	}
}

// Pow returns the kind of k raised to the n-th power: the exponents of k scaled
// by n. Pow(0) is [Dimensionless] and Pow(1) is k. An n far outside the exponent
// range saturates to an endpoint, preserving sign, rather than wrapping.
func (k Kind) Pow(n int) Kind {
	return Kind{
		l: scaleExp(k.l, n),
		m: scaleExp(k.m, n),
		a: scaleExp(k.a, n),
	}
}

// String returns the name of a named kind ("area", "density", …), or the
// exponent form of an unnamed one ("L⁻¹", "L²·M"). It is display text, never a
// unit symbol — see [Kind.canonicalSymbol] for that. It never panics and never
// returns the empty string.
func (k Kind) String() string {
	if name, ok := kindNames[k]; ok {
		return name
	}

	parts := make([]string, 0, 3)
	for _, d := range k.dimensions() {
		if d.exp != 0 {
			parts = append(parts, d.symbol+superscript(d.exp))
		}
	}
	if len(parts) == 0 {
		// Unreachable: the all-zero kind is Dimensionless, which is named.
		return "dimensionless"
	}
	return strings.Join(parts, "·")
}

// canonicalSymbol returns the synthetic unit symbol a value of this kind is
// carried in when the kind has no registered base unit: an ASCII exponent form
// in the fixed order length, mass, angle, wrapped in brackets — "[L^-1]",
// "[L^2*M]". The brackets are a reserved namespace ([Define] rejects a symbol
// that opens with one), so a synthetic symbol can never be confused with, or
// hijacked by, a real unit. It is never the kind's display name: a unit symbol
// is a key, not prose.
func (k Kind) canonicalSymbol() string {
	parts := make([]string, 0, 3)
	for _, d := range k.dimensions() {
		switch d.exp {
		case 0:
		case 1:
			parts = append(parts, d.symbol)
		default:
			parts = append(parts, d.symbol+"^"+strconv.Itoa(int(d.exp)))
		}
	}
	if len(parts) == 0 {
		// Unreachable: the all-zero kind is Dimensionless, which has a base unit.
		return "[1]"
	}
	return "[" + strings.Join(parts, "*") + "]"
}

// dimension is one base dimension of a kind: its symbol and this kind's exponent
// of it.
type dimension struct {
	symbol string
	exp    int8
}

// dimensions returns k's exponents in the fixed order length, mass, angle. Both
// the display form and the canonical symbol iterate it, so the two never drift
// apart in ordering.
func (k Kind) dimensions() [3]dimension {
	return [3]dimension{{"L", k.l}, {"M", k.m}, {"A", k.a}}
}

// clampExp narrows a computed exponent to the int8 range that a [Kind] stores.
// Saturating beats wrapping: a kind that overflowed is a bug either way, and a
// clamped kind cannot silently masquerade as a different, plausible kind.
func clampExp(x int64) int8 {
	if x > math.MaxInt8 {
		return math.MaxInt8
	}
	if x < math.MinInt8 {
		return math.MinInt8
	}
	return int8(x)
}

// scaleExp multiplies an exponent by n and clamps the product to the int8 range.
// n is narrowed to that range first: an int is 64 bits wide, so the product of a
// raw n and an exponent would overflow int64 and wrap — turning an absurd power
// into a plausible-looking kind — before clampExp ever saw it. Narrowing first
// preserves the sign, so a huge n still saturates to the endpoint it points at,
// and bounds the product by 127*128.
func scaleExp(e int8, n int) int8 {
	switch {
	case n > math.MaxInt8:
		n = math.MaxInt8
	case n < math.MinInt8:
		n = math.MinInt8
	}
	return clampExp(int64(e) * int64(n))
}

// superDigits maps a decimal digit to its Unicode superscript.
var superDigits = [10]rune{'⁰', '¹', '²', '³', '⁴', '⁵', '⁶', '⁷', '⁸', '⁹'}

// superscript renders an exponent as Unicode superscript digits; an exponent of
// 1 renders as nothing, so "L" rather than "L¹".
func superscript(n int8) string {
	if n == 1 {
		return ""
	}

	var b strings.Builder
	for _, r := range strconv.Itoa(int(n)) {
		if r == '-' {
			b.WriteRune('⁻')
			continue
		}
		b.WriteRune(superDigits[r-'0'])
	}
	return b.String()
}
