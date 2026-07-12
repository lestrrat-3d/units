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
// Kind is comparable: two kinds are equal exactly when their exponents match and
// neither has overflowed (see [Kind.Overflowed]). The zero value is
// [Dimensionless]. A [Value] never converts across kinds: a length is not an
// angle, and asking for one as the other is an error rather than a coercion.
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
// approaches that: L⁴ (a second moment of area) is as exotic as it gets. A
// composition that would exceed the range is a programming error rather than a
// meaningful kind, and it is recorded as one: the exponent is clamped to the
// endpoint and the kind is marked overflowed. See [Kind.Overflowed].
type Kind struct {
	l, m, a int8 // exponents of length, mass and angle
	ovf     bool // an exponent saturated; sticky, and never cleared
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
// from it is unnamed and prints in exponent form; an overflowed kind is absent
// from it by construction, and prints as "overflowed".
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

// Overflowed reports whether an exponent of this kind saturated: the kind is a
// clamped stand-in for a composition the int8 exponents cannot hold, and its
// exponents are not the true ones.
//
// The state is sticky. [Kind.Mul], [Kind.Div] and [Kind.Pow] propagate it from
// either operand, so an overflowed kind can never be composed back into an
// ordinary one: it is never equal to a named kind, [BaseUnit] reports no base
// unit for it, and it prints as "overflowed". Without that, dividing a saturated
// L¹²⁷ by L¹²⁶ would hand back a plain [Length] — a lie about the exponent, in
// the shape of a perfectly plausible kind.
//
// An overflowed kind is a programming error made visible, not a kind to compute
// with. Values of one still compose and print; nothing panics.
func (k Kind) Overflowed() bool { return k.ovf }

// Mul returns the kind of a product: the exponents of k and o added. The result
// is overflowed if either operand is, or if the sum saturates.
func (k Kind) Mul(o Kind) Kind {
	l, lo := clampExp(int64(k.l) + int64(o.l))
	m, mo := clampExp(int64(k.m) + int64(o.m))
	a, ao := clampExp(int64(k.a) + int64(o.a))
	return Kind{l: l, m: m, a: a, ovf: k.ovf || o.ovf || lo || mo || ao}
}

// Div returns the kind of a quotient: the exponents of o subtracted from those
// of k. The result is overflowed if either operand is, or if the difference
// saturates.
func (k Kind) Div(o Kind) Kind {
	l, lo := clampExp(int64(k.l) - int64(o.l))
	m, mo := clampExp(int64(k.m) - int64(o.m))
	a, ao := clampExp(int64(k.a) - int64(o.a))
	return Kind{l: l, m: m, a: a, ovf: k.ovf || o.ovf || lo || mo || ao}
}

// Pow returns the kind of k raised to the n-th power: the exponents of k scaled
// by n. Pow(0) is [Dimensionless] and Pow(1) is k. An n that scales an exponent
// out of range saturates to the endpoint the sign points at, rather than
// wrapping, and the result is overflowed — as it is if k already was. A power of
// [Dimensionless] has no exponent to scale, so it is dimensionless for every n.
func (k Kind) Pow(n int) Kind {
	l, lo := scaleExp(k.l, n)
	m, mo := scaleExp(k.m, n)
	a, ao := scaleExp(k.a, n)
	return Kind{l: l, m: m, a: a, ovf: k.ovf || lo || mo || ao}
}

// String returns the name of a named kind ("area", "density", …), or the
// exponent form of an unnamed one ("L⁻¹", "L²·M"). An overflowed kind prints as
// "overflowed": its exponents are clamped stand-ins, and printing them would
// state a dimension it does not have. It is display text, never a unit symbol —
// see [Kind.canonicalSymbol] for that. It never panics and never returns the
// empty string.
func (k Kind) String() string {
	if k.ovf {
		return "overflowed"
	}
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
// "[L^2*M]". An overflowed kind has no exponents worth writing and gets
// "[overflow]", which no dimension form can collide with. The brackets are a
// reserved namespace ([Define] rejects a symbol that opens with one), so a
// synthetic symbol can never be confused with, or hijacked by, a real unit. It
// is never the kind's display name: a unit symbol is a key, not prose.
func (k Kind) canonicalSymbol() string {
	if k.ovf {
		return "[overflow]"
	}

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

// clampExp narrows a computed exponent to the int8 range that a [Kind] stores,
// and reports whether it had to. Saturating beats wrapping — a wrapped exponent
// lands on a plausible-looking wrong kind — but a clamped exponent is still a
// lie about the number, so the bool is what keeps it from masquerading: the
// caller records it, stickily, on the kind.
func clampExp(x int64) (int8, bool) {
	if x > math.MaxInt8 {
		return math.MaxInt8, true
	}
	if x < math.MinInt8 {
		return math.MinInt8, true
	}
	return int8(x), false
}

// scaleExp multiplies an exponent by n, clamps the product to the int8 range,
// and reports whether the true product is out of that range.
//
// An int is 64 bits wide, so int64(e) * int64(n) can overflow int64 and wrap —
// turning an absurd power into a plausible-looking kind — before clampExp ever
// sees it. An |n| above 128 is therefore answered before the multiplication: the
// product of it and any nonzero exponent is at least |n| in magnitude, so it is
// out of range whatever the exponent, and the endpoint the two signs point at is
// the answer. Below that the product is bounded by 128·128 and int64 holds it
// exactly, so clampExp decides. A zero exponent stays zero for every n, which is
// why every power of [Dimensionless] is dimensionless.
func scaleExp(e int8, n int) (int8, bool) {
	if e == 0 {
		return 0, false
	}
	if n > 128 || n < -128 {
		if (e > 0) == (n > 0) {
			return math.MaxInt8, true
		}
		return math.MinInt8, true
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
