package units

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
)

// ErrIncompatible is returned when an operation mixes units of different kinds
// (for example adding a length to an angle).
var ErrIncompatible = errors.New("units: incompatible kinds")

// ErrDivideByZero is returned by [Value.Div] when the divisor is zero. A unit's
// factor is positive and finite, so a zero magnitude is the whole of it: a
// divisor that is nonzero in its own unit is a real divisor, however small.
var ErrDivideByZero = errors.New("units: division by zero")

// ErrNotFinite is returned by [Value.Add], [Value.Sub], [Value.Mul], [Value.Div],
// [Value.In] and [Value.Convert] when the result is not finite: a sum, product or
// quotient that overflows to an infinity, or one — Inf × 0, say — that is a NaN.
// Every one of those operations yields a finite result or an error, never an
// infinity and never a NaN, because a non-finite magnitude carries a registered
// symbol and so would persist as if it were a real quantity.
//
// It is the result that must be representable, never an intermediate: an operand
// whose own base magnitude overflows — [Meters](1e307), whose [Value.Base] is
// +Inf — is an ordinary operand, and an operation on it whose result is in range
// returns that result.
//
// And it is the true result that decides, at both ends of the range and across a
// cancellation: an error exactly when the exact value of the operation is past the
// last float64, and the correctly rounded value — MaxFloat64 itself included, and
// the subnormals — when it is not. No end of the range, and no sum whose operands
// nearly annihilate, is a matter of which way an intermediate happened to round.
//
// The operations that cannot report an error — [New], [FromBase], [Value.Scale]
// and [Value.Neg] — do not check: a caller that hands them an infinity, or that
// scales a value up past the float64 range, gets a non-finite Value back.
var ErrNotFinite = errors.New("units: result is not finite")

// Value is a magnitude paired with the [Unit] it is expressed in. The zero
// Value is 0 of the dimensionless unit [One]: the zero [Unit] is read as One, so
// a Value declared with var behaves as a plain 0 in every operation.
type Value struct {
	mag  float64
	unit Unit
}

// New returns a Value of mag in unit u. It reports no error, so it does not
// check mag: an infinite or NaN mag yields a Value carrying it. The arithmetic
// that can report an error ([Value.Add], [Value.Sub], [Value.Mul], [Value.Div])
// refuses to produce one.
func New(mag float64, u Unit) Value { return Value{mag: mag, unit: u} }

// FromBase returns a Value equal to base (expressed in u's base unit), but
// carried in unit u. For example FromBase(1000, Meter) is 1 m. Like [New] it
// reports no error and so does not check base: an infinite or NaN base yields a
// non-finite Value.
func FromBase(base float64, u Unit) Value {
	u = u.normalize()
	return Value{mag: base / u.factor, unit: u}
}

// Millimeters, and the constructors below it, build a Value of x in each of the
// built-in units.
func Millimeters(x float64) Value { return Value{x, Millimeter} }
func Centimeters(x float64) Value { return Value{x, Centimeter} }
func Meters(x float64) Value      { return Value{x, Meter} }
func Inches(x float64) Value      { return Value{x, Inch} }
func Feet(x float64) Value        { return Value{x, Foot} }
func Thous(x float64) Value       { return Value{x, Thou} }
func Degrees(x float64) Value     { return Value{x, Degree} }
func Radians(x float64) Value     { return Value{x, Radian} }
func Scalar(x float64) Value      { return Value{x, One} }

func SquareMillimeters(x float64) Value { return Value{x, SquareMillimeter} }
func SquareCentimeters(x float64) Value { return Value{x, SquareCentimeter} }
func SquareMeters(x float64) Value      { return Value{x, SquareMeter} }
func SquareInches(x float64) Value      { return Value{x, SquareInch} }

func CubicMillimeters(x float64) Value { return Value{x, CubicMillimeter} }
func CubicCentimeters(x float64) Value { return Value{x, CubicCentimeter} }
func CubicMeters(x float64) Value      { return Value{x, CubicMeter} }
func CubicInches(x float64) Value      { return Value{x, CubicInch} }
func Liters(x float64) Value           { return Value{x, Liter} }

func Kilograms(x float64) Value { return Value{x, Kilogram} }
func Grams(x float64) Value     { return Value{x, Gram} }
func Pounds(x float64) Value    { return Value{x, Pound} }

func KilogramSquareMillimeters(x float64) Value { return Value{x, KilogramSquareMillimeter} }
func QuarticMillimeters(x float64) Value        { return Value{x, QuarticMillimeter} }

func KilogramsPerCubicMillimeter(x float64) Value { return Value{x, KilogramPerCubicMillimeter} }
func KilogramsPerCubicMeter(x float64) Value      { return Value{x, KilogramPerCubicMeter} }
func GramsPerCubicCentimeter(x float64) Value     { return Value{x, GramPerCubicCentimeter} }

// Mag returns the magnitude in the value's own unit.
func (v Value) Mag() float64 { return v.mag }

// Unit returns the value's unit; for the zero Value that is [One].
func (v Value) Unit() Unit { return v.unit.normalize() }

// Kind returns the kind of quantity the value measures.
func (v Value) Kind() Kind { return v.unit.kind }

// Base returns the magnitude expressed in the kind's base unit (mm for a
// length, mm² for an area, rad for an angle).
//
// It is an accessor, not an operation, and it is the one place a base magnitude
// is formed: the product of an ordinary magnitude and its unit's factor can
// overflow on its own — [Meters](1e307) is an ordinary length whose base
// magnitude is +Inf — so Base honestly reports that infinity. No operation on a
// [Value] forms one, so none of them inherits that overflow; see [rescale].
func (v Value) Base() float64 { return v.mag * v.Unit().factor }

// In returns the magnitude expressed in unit u, or [ErrIncompatible] if u
// measures a different kind. A magnitude that is not finite in u — a value built
// from an infinity, or one whose conversion genuinely overflows — is
// [ErrNotFinite]. A value is always expressible in the unit it already carries,
// however large its base magnitude.
func (v Value) In(u Unit) (float64, error) {
	if v.unit.kind != u.kind {
		return 0, fmt.Errorf("%w: cannot express %s in %s", ErrIncompatible, v.unit.kind, u.kind)
	}
	m := rescale(v.mag, v.Unit().factor, u.Factor())
	if !isFinite(m) {
		return 0, fmt.Errorf("%w: cannot express %s in %s", ErrNotFinite, v, u)
	}
	return m, nil
}

// Convert returns the same quantity carried in unit u, under the same rules as
// [Value.In]: a different kind is [ErrIncompatible], a non-finite magnitude is
// [ErrNotFinite].
func (v Value) Convert(u Unit) (Value, error) {
	m, err := v.In(u)
	if err != nil {
		return Value{}, err
	}
	return Value{m, u.normalize()}, nil
}

// Add returns v + o. The operands must be the same kind, with one carve-out: an
// [Angle] may be added to a [Dimensionless] value, because a radian really is a
// ratio of two lengths and theta + pi/2 is an angle. The result is expressed in
// v's unit, or — for that carve-out — in whichever operand's unit is the angle,
// so the sum is an angle whichever side it appeared on.
//
// The sum is the true sum, correctly rounded: it is decided on the exact value of
// v + o, never on a rounding of either operand on its way into the result's unit,
// so a difference that cancels keeps whatever the cancellation leaves —
// [ErrNotFinite] exactly when the true sum is past the last float64, and the
// nearest float64, subnormals included, when it is not.
func (v Value) Add(o Value) (Value, error) { return v.combine(o, 1) }

// Sub returns v − o, under the same rules as [Value.Add], including the finite,
// correctly rounded result: a difference that overflows or is a NaN is
// [ErrNotFinite], and one that cancels is the true difference.
func (v Value) Sub(o Value) (Value, error) { return v.combine(o, -1) }

// combine adds sign*o to v.
func (v Value) combine(o Value, sign float64) (Value, error) {
	vu, ou := v.Unit(), o.Unit()

	// The sum is carried in v's unit, except in the angle/dimensionless carve-out
	// entered from the dimensionless side, where it is carried in o's — so the
	// sum is an angle whichever operand the angle was.
	u := vu
	if vu.kind != ou.kind {
		if !isAngleScalarPair(vu.kind, ou.kind) {
			return Value{}, fmt.Errorf("%w: cannot combine %s with %s", ErrIncompatible, v.unit.kind, o.unit.kind)
		}
		if vu.kind == Dimensionless {
			u = ou
		}
	}

	// The whole sum is formed by one helper, never as a rescale of each operand
	// followed by an addition: neither operand goes through its own base magnitude
	// — either magnitude times its own factor can overflow on its own, even where
	// the sum is representable — and neither rescaled operand is rounded before the
	// addition, which is what cancellation lives on. See [sum].
	m := sum(v.mag, vu.factor, sign, o.mag, ou.factor, u.factor)
	if !isFinite(m) {
		return Value{}, fmt.Errorf("%w: cannot combine %s with %s", ErrNotFinite, v, o)
	}
	return Value{m, u}, nil
}

// isAngleScalarPair reports whether a and b are an angle and a dimensionless
// value, in either order — the one pair Add and Sub accept across kinds.
func isAngleScalarPair(a, b Kind) bool {
	return (a == Angle && b == Dimensionless) || (a == Dimensionless && b == Angle)
}

// Mul returns v × o: the magnitudes multiplied in base units, and the kinds
// composed. Millimeters(2).Mul(Millimeters(3)) is 6 mm², an [Area]. The result
// is carried in the base unit of the resulting kind (whose factor is 1, so the
// product is the result's magnitude).
//
// The product is finite whenever the product itself is representable — an
// operand whose own base magnitude overflows is no obstacle, and a true product
// that lands on the last float64 is that float64. A product whose true value is
// past the last float64, or that is a NaN, is [ErrNotFinite].
func (v Value) Mul(o Value) (Value, error) {
	p := product(v.mag, v.Unit().factor, o.mag, o.Unit().factor)
	if !isFinite(p) {
		return Value{}, fmt.Errorf("%w: cannot multiply %s by %s", ErrNotFinite, v, o)
	}
	return Value{p, baseUnitFor(v.unit.kind.Mul(o.unit.kind))}, nil
}

// Div returns v ÷ o: the magnitudes divided in base units, and the kinds
// composed. Volume divided by Area is a [Length]. The result is carried in the
// base unit of the resulting kind (whose factor is 1, so the quotient is the
// result's magnitude).
//
// The quotient is finite whenever the quotient itself is representable — a value
// divided by itself is 1 however large or small its base magnitude, and a true
// quotient that lands on the last float64 is that float64. A zero divisor is
// [ErrDivideByZero], and a divisor small enough — or a dividend large enough — to
// carry the true quotient past the last float64, or to make it a NaN, is
// [ErrNotFinite].
func (v Value) Div(o Value) (Value, error) {
	// The divisor's own magnitude is the guard, never its base magnitude: a unit's
	// factor is positive and finite, so a magnitude is zero exactly when the
	// quantity is. A base magnitude would say zero for an ordinary small divisor
	// whose product with its factor underflows, and report a divide-by-zero for a
	// quotient that is perfectly ordinary.
	if o.mag == 0 {
		return Value{}, fmt.Errorf("%w: cannot divide %s by %s", ErrDivideByZero, v, o)
	}
	q := quotient(v.mag, v.Unit().factor, o.mag, o.Unit().factor)
	if !isFinite(q) {
		return Value{}, fmt.Errorf("%w: cannot divide %s by %s", ErrNotFinite, v, o)
	}
	return Value{q, baseUnitFor(v.unit.kind.Div(o.unit.kind))}, nil
}

// isFinite reports whether x is a real number: neither an infinity nor a NaN.
func isFinite(x float64) bool { return !math.IsInf(x, 0) && !math.IsNaN(x) }

// sameInfinity reports whether a and b are the same signed infinity. A NaN is
// neither, so it is the same as nothing — itself included.
func sameInfinity(a, b float64) bool {
	return (math.IsInf(a, 1) && math.IsInf(b, 1)) || (math.IsInf(a, -1) && math.IsInf(b, -1))
}

// The four helpers below are how every operation on a [Value] does its
// arithmetic, and no operation may bypass them by forming a base magnitude
// ([Value.Base], a magnitude times its unit's factor) as an intermediate: that
// product overflows for an ordinary operand such as [Meters](1e307), and
// underflows for one such as [Grams](1e-322), even where the operation's own
// result is perfectly representable.
//
// An operation is one helper, never a composition of them. A sum is not a rescale
// of each operand followed by an addition: each step can be correct on its own
// while the composition is not, because the rounding between them happens before
// the addition can use the bits it destroys. [sum] is that whole operation, and it
// decides finiteness and rounding on the true sum.
//
// Each has two paths. The fast one splits its operands with [math.Frexp] into a
// mantissa in [0.5, 1) and a binary exponent, combines the mantissas — a bounded
// handful of them, so their product can neither overflow nor underflow, and every
// intermediate keeps its full 53 bits — sums the exponents as ints, and puts the
// scale back with [math.Ldexp]. The mantissas are combined in the same order, and
// grouped the same way, as the plain arithmetic would combine the operands, so
// each helper rounds where the plain expression rounds and no conversion pays for
// the extra range. Mantissas that cancel — the same factor above and below —
// cancel exactly, so a value divided by itself is exact, and [rescale] returns the
// magnitude untouched when the two factors are the same.
//
// That path is trustworthy only in the middle of the float64 range, and [atTheEnds]
// is where it stops being so. Every mantissa it combines is rounded to 53 bits, and
// a rounded mantissa cannot report what happened at a boundary it was never near:
// the exponent is still carried separately, so a product whose true value is past
// the last float64 comes back as a finite MaxFloat64, and a quotient whose mantissa
// rounds up over the boundary comes back as an infinity though its true value is
// representable. The same rounding at the other end lands in the subnormals, where
// [math.Ldexp] rounds it a second time — a double rounding, worse than the plain
// expression, which rounds once.
//
// So at either end the arithmetic is redone in exact rationals and rounded once, by
// [exact]. A float64 magnitude and a float64 factor are exact rationals, so their
// products and quotients are the true quantity, whatever its size; rounding that
// rational to float64 once gives the correctly rounded result — an infinity exactly
// when the true result overflows, the nearest float64 when it does not, and the
// nearest subnormal at the bottom. It is never further from the true result than the
// plain expression, because no float64 is. The ends are where results are rare and
// boundaries are decided, so the cost is paid nowhere it is felt.
//
// An infinity or a NaN operand has no exact rational, and takes the fast path
// whatever it lands on: it survives Frexp unchanged and propagates as it would have
// through the plain arithmetic, so Inf × 0 is still a NaN.

// nearTheEnds is the binary exponent past which the fast path is not trusted.
// float64 runs out at 2¹⁰²⁴ and its normals at 2⁻¹⁰²², so ±1000 leaves twenty-odd
// binades of margin at each end.
const nearTheEnds = 1000

// atTheEnds reports whether a result assembled at binary exponent e is close enough
// to an end of the float64 range that the mantissa roundings on the way to it could
// have decided the wrong side of a boundary.
//
// It reads the exponent rather than the assembled result, so it is settled before
// any rounding happens and cannot itself be fooled by one. Every mantissa a helper
// combines is in [0.5, 1), so the mantissa it assembles is within four powers of two
// of 1 either way, and the true result is between 2ᵉ⁻⁴ and 2ᵉ⁺². Every boundary is
// therefore inside the window — an overflow needs e ≥ 1022, a subnormal e ≤ −1018 —
// and no ordinary result is.
func atTheEnds(e int) bool { return e >= nearTheEnds || e <= -nearTheEnds }

// exact rounds the true rational value of an operation to float64, once: an
// infinity exactly when it overflows the range, and the nearest float64 — subnormals
// included — when it does not.
//
// A rational carries no signed zero, so a result that underflows takes its sign from
// neg: the sign the plain expression would have given it.
func exact(r *big.Rat, neg bool) float64 {
	f, _ := r.Float64()
	if f == 0 && neg {
		return math.Copysign(0, -1)
	}
	return f
}

// baseRat returns m × factor exactly: the base magnitude as a rational, which —
// unlike the float64 [Value.Base] — cannot overflow or underflow.
func baseRat(m, factor float64) *big.Rat {
	return new(big.Rat).Mul(new(big.Rat).SetFloat64(m), new(big.Rat).SetFloat64(factor))
}

// exactly reports whether the operands have an exact rational rendering, which
// every float64 but an infinity and a NaN has.
func exactly(xs ...float64) bool {
	for _, x := range xs {
		if !isFinite(x) {
			return false
		}
	}
	return true
}

// rescale returns m × (from / to): a magnitude carried in a unit of factor from,
// expressed in a unit of factor to. from == to returns m exactly, so a value is
// always expressible in the unit it already carries.
//
// The mantissas are combined in the same order as the plain m × from ÷ to, so the
// rounding is the plain expression's — the exponent split costs no accuracy, it only
// keeps the intermediate in range — and at the ends of the range the true value is
// rounded once instead.
func rescale(m, from, to float64) float64 {
	if from == to {
		return m
	}
	fm, em := math.Frexp(m)
	ffrom, efrom := math.Frexp(from)
	fto, eto := math.Frexp(to)
	e := em + efrom - eto
	if !atTheEnds(e) || !exactly(m, from, to) || to == 0 {
		return math.Ldexp(fm*ffrom/fto, e)
	}
	return exact(new(big.Rat).Quo(baseRat(m, from), new(big.Rat).SetFloat64(to)), math.Signbit(m))
}

// product returns (a × af) × (b × bf): two magnitudes multiplied in base units.
// The mantissas are grouped as the plain expression groups them, so the rounding is
// the plain expression's; at the ends of the range the true product is rounded once
// instead, so it overflows exactly when the true product does.
func product(a, af, b, bf float64) float64 {
	fa, ea := math.Frexp(a)
	faf, eaf := math.Frexp(af)
	fb, eb := math.Frexp(b)
	fbf, ebf := math.Frexp(bf)
	e := ea + eaf + eb + ebf
	if !atTheEnds(e) || !exactly(a, af, b, bf) {
		return math.Ldexp((fa*faf)*(fb*fbf), e)
	}
	return exact(new(big.Rat).Mul(baseRat(a, af), baseRat(b, bf)), math.Signbit(a) != math.Signbit(b))
}

// quotient returns (a × af) ÷ (b × bf): two magnitudes divided in base units, under
// the same rule as [product] — and a zero divisor, which [Value.Div] refuses before
// it gets here, keeps the fast path, where it is an infinity or a NaN as it would be
// in the plain expression.
func quotient(a, af, b, bf float64) float64 {
	fa, ea := math.Frexp(a)
	faf, eaf := math.Frexp(af)
	fb, eb := math.Frexp(b)
	fbf, ebf := math.Frexp(bf)
	e := ea + eaf - eb - ebf
	if !atTheEnds(e) || !exactly(a, af, b, bf) || b == 0 || bf == 0 {
		return math.Ldexp((fa*faf)/(fb*fbf), e)
	}
	return exact(new(big.Rat).Quo(baseRat(a, af), baseRat(b, bf)), math.Signbit(a) != math.Signbit(b))
}

// sum returns (a × af + sign × b × bf) ÷ to: two magnitudes, each carried in a unit
// of its own factor, added — sign is +1 or −1 — in the unit of factor to, which is
// af or bf. It is the whole of an addition, not a step in one, and that is the point.
//
// A sum is the one operation the [atTheEnds] rule cannot be applied a step at a time.
// Rescaling each operand into to and then adding the two results is a *composition*:
// each rescale can be correctly rounded on its own and the sum still be wrong, because
// the bits the addition would have cancelled against are gone before it sees them. Two
// operands whose factors are far apart — a magnitude at the top of the range in a unit
// of factor 1e-300, and its near-negation in one of factor 1e300 — rescale to two
// 53-bit numbers that annihilate, and the true difference, which float64 holds without
// difficulty, is lost entirely. So the true sum is what decides, and it is rounded
// once.
//
// The fast path is therefore kept only where the addition is the sole rounding:
// where both operands are already carried in to (af == bf, so to is both, and each
// term is the operand's own magnitude), an IEEE addition of two exact terms is
// correctly rounded by definition — an infinity exactly when the true sum overflows,
// and the nearest float64, subnormals included, when it does not. Wherever a rescale
// is needed the rounding it would do is a rounding the sum has not authorised, so the
// arithmetic is redone in exact rationals: a float64 magnitude and a float64 factor
// are exact rationals, and so is their sum, whatever the two units are.
//
// An infinity or a NaN operand has no exact rational and keeps the fast path, where it
// propagates as it would through the plain arithmetic.
func sum(a, af, sign, b, bf, to float64) float64 {
	if af == bf || to == 0 || !exactly(a, af, b, bf, to) {
		return rescale(a, af, to) + sign*rescale(b, bf, to)
	}
	t := baseRat(b, bf)
	if sign < 0 {
		t.Neg(t)
	}
	t.Add(baseRat(a, af), t)
	t.Quo(t, new(big.Rat).SetFloat64(to))

	// A sum that underflows takes the sign of the true sum; one that is exactly zero
	// is +0, as x + (−x) is in the plain arithmetic.
	return exact(t, t.Sign() < 0)
}

// Scale returns v multiplied by a dimensionless factor. It reports no error, so
// — unlike [Value.Mul] — it does not guarantee a finite result: an f large
// enough to overflow the magnitude, an infinite f, or a NaN f yields a
// non-finite Value. Use Mul with a [Scalar] where the result must be checked.
//
// Accuracy is not at stake in it, only finiteness: the magnitude is scaled in v's
// own unit, so there is one multiplication and one rounding — no unit factor, no
// rescale, and nothing to cancel — and the result is the correctly rounded one
// whenever float64 holds it.
func (v Value) Scale(f float64) Value { return Value{v.mag * f, v.unit} }

// Neg returns −v. It reports no error and cannot make a finite magnitude
// non-finite, and negation is exact; a v that is already non-finite negates to a
// non-finite Value.
func (v Value) Neg() Value { return Value{-v.mag, v.unit} }

// Equal reports whether v and o represent the same quantity to within tol of the
// kind's base unit: it is true exactly when |v − o| <= tol, where v − o is the
// true difference of the two quantities in base units. Values of different kinds
// are never equal. The answer does not depend on which operand is the receiver.
//
// The difference is the *true* one, never a rounding of it. Rescaling both
// operands into a common unit and subtracting there is a composition, and the
// rounding in between is one the comparison never authorised: a difference of
// 1e-300 mm rescaled into a unit of factor 1e300 underflows to zero, and two
// quantities that genuinely differ come back equal at a tolerance of zero. So
// wherever the float64 arithmetic cannot hold the difference the comparison is
// made in exact rationals; see [sum], which reads the same rule.
//
// # A non-finite magnitude is not a quantity
//
// [New], [FromBase], [Value.Scale] and [Value.Neg] have no error to report, so a
// Value can be built whose magnitude is an infinity or a NaN. It is not a
// quantity, and it has no difference from one:
//
//   - Exactly one magnitude non-finite: false, at every tol, +Inf included. An
//     infinite length is not within any tolerance of a finite one; a tolerance is
//     a bound on the difference of two real numbers, and there is none here.
//   - Both magnitudes infinite: true exactly when they are the same signed
//     infinity (a factor is positive, so the sign of the magnitude is the sign of
//     the quantity) — so a value built with New(math.Inf(1), …) still equals
//     itself, and +Inf never equals −Inf.
//   - Either magnitude a NaN: false, always, including against itself. A NaN is
//     equal to nothing, which is the IEEE rule.
//
// A tol that is negative or a NaN is no bound on any difference and admits
// nothing, non-finite magnitudes included; tol == +Inf admits every pair of
// finite quantities of the same kind, and nothing else.
//
// # What tol == 0 means
//
// A tolerance of zero asks whether the two quantities are exactly the same real
// number — their true base-unit magnitudes coincide — and nothing weaker. Two
// values written in different units generally do NOT coincide, because a unit's
// factor is itself a rounded float64: [Degree]'s factor is a rounded π/180, so
// 180 × factor and math.Pi are different quantities and Degrees(180).Equal(
// Radians(math.Pi), 0) is false. They differ, and Equal says so.
//
// So tol == 0 is for values carried in the same unit, and for asking whether two
// quantities are exactly the same real number. Pass a tolerance — in base units —
// for any comparison across units.
func (v Value) Equal(o Value, tol float64) bool {
	if v.unit.kind != o.unit.kind {
		return false
	}
	vu, ou := v.Unit(), o.Unit()

	// A magnitude that is not finite is not a quantity: it has no true difference
	// from one, and so no tolerance — not even an infinite one — can admit it beside
	// one. It is answered here, before any arithmetic, rather than left to a float
	// path where |Inf − Inf| is a NaN and |Inf − 1| is the +Inf that an infinite tol
	// would let through. The one pair that is equal is a value and the same signed
	// infinity: reflexivity, for a value someone built from an infinity on purpose.
	// A factor is positive, so the sign of the magnitude is the sign of the quantity,
	// and the answer reads the two magnitudes alone — nothing about which operand is
	// the receiver.
	if !exactly(v.mag, o.mag) {
		return sameInfinity(v.mag, o.mag) && tol >= 0
	}

	// The fast path, kept where it is provably lossless: two operands carried in the
	// same unit differ by exactly v.mag − o.mag of it, and a factor is positive and
	// finite, so they are the same quantity exactly when their magnitudes are — no
	// arithmetic, and so no rounding, stands between the operands and that answer. A
	// value therefore equals itself however large its base magnitude, and equal
	// magnitudes in one unit are equal at every tolerance a real number can take.
	if vu.factor == ou.factor {
		if v.mag == o.mag {
			return tol >= 0
		}
		if tol == 0 {
			return false
		}
	}

	// A tolerance that is not a real number is no bound on a real difference: every
	// difference of two finite quantities is within an infinite tolerance, and none
	// is within a NaN or a negative infinity.
	if !exactly(tol) {
		return math.IsInf(tol, 1)
	}

	// A float64 magnitude and a float64 factor are exact rationals, so the true
	// difference in base units is one too, whatever the two units are and however far
	// apart their factors. It is compared with tol there, exactly: nothing is rounded,
	// and swapping the operands only negates it.
	d := new(big.Rat).Sub(baseRat(v.mag, vu.factor), baseRat(o.mag, ou.factor))
	return d.Abs(d).Cmp(new(big.Rat).SetFloat64(tol)) <= 0
}

// String renders the value as "<magnitude> <symbol>" (just the number for
// dimensionless values).
func (v Value) String() string {
	n := strconv.FormatFloat(v.mag, 'g', -1, 64)
	if v.unit.kind == Dimensionless {
		return n
	}
	return n + " " + v.Unit().symbol
}
