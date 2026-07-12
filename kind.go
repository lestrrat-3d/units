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
// by n. Pow(0) is [Dimensionless] and Pow(1) is k.
func (k Kind) Pow(n int) Kind {
	return Kind{
		l: clampExp(int64(k.l) * int64(n)),
		m: clampExp(int64(k.m) * int64(n)),
		a: clampExp(int64(k.a) * int64(n)),
	}
}

// String returns the name of a named kind ("area", "density", …), or the
// exponent form of an unnamed one ("L⁻¹", "L²·M"). It never panics and never
// returns the empty string.
func (k Kind) String() string {
	if name, ok := kindNames[k]; ok {
		return name
	}

	parts := make([]string, 0, 3)
	for _, d := range []struct {
		symbol string
		exp    int8
	}{
		{"L", k.l},
		{"M", k.m},
		{"A", k.a},
	} {
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
