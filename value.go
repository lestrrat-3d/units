package units

import (
	"errors"
	"fmt"
	"math"
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
// The sum is always finite: operands large enough to overflow it to an infinity,
// or to make it a NaN, are [ErrNotFinite].
func (v Value) Add(o Value) (Value, error) { return v.combine(o, 1) }

// Sub returns v − o, under the same rules as [Value.Add], including the finite
// result: a difference that overflows or is a NaN is [ErrNotFinite].
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

	// Each operand is rescaled into u by the ratio of the factors, never through
	// its own base magnitude: either magnitude times its own factor can overflow
	// on its own, even where the sum is representable. The operand already
	// carried in u rescales by exactly 1.
	m := rescale(v.mag, vu.factor, u.factor) + sign*rescale(o.mag, ou.factor, u.factor)
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
// operand whose own base magnitude overflows is no obstacle. A product that
// genuinely overflows to an infinity, or that is a NaN, is [ErrNotFinite].
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
// divided by itself is 1 however large or small its base magnitude. A zero
// divisor is [ErrDivideByZero], and a divisor small enough — or a dividend large
// enough — to blow the quotient up to an infinity or a NaN is [ErrNotFinite].
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

// The three helpers below are how every operation on a [Value] does its
// arithmetic, and no operation may bypass them by forming a base magnitude
// ([Value.Base], a magnitude times its unit's factor) as an intermediate: that
// product overflows for an ordinary operand such as [Meters](1e307), and
// underflows for one such as [Grams](1e-322), even where the operation's own
// result is perfectly representable.
//
// Each splits its operands with [math.Frexp] into a mantissa in [0.5, 1) and a
// binary exponent, combines the mantissas — a bounded handful of them, so their
// product can neither overflow nor underflow, and every intermediate keeps its
// full 53 bits — sums the exponents as ints, and puts the scale back with
// [math.Ldexp]. Only the last step can leave the float64 range, and it does so
// exactly when the result does. The mantissas are combined in the same order,
// and grouped the same way, as the plain arithmetic would combine the operands,
// so each helper rounds where the plain expression rounds and no conversion pays
// for the extra range. Mantissas that cancel — the same factor above and below —
// cancel exactly, so a value divided by itself is exact, and [rescale] returns
// the magnitude untouched when the two factors are the same.
//
// The scale goes back on through [assembleMul] and [assembleDiv], never a bare
// Ldexp of the fully combined mantissa: rounding the combined mantissa to 53
// bits and then rounding again into the subnormal range is a double rounding,
// and it is worse than the plain expression exactly where the plain expression
// still works — [Scalar](1.25) divided by [Centimeters](1e307) is 1.25e-308, not
// a bit less. The assemblers put the two sides back at their own scales, so the
// last operation rounds once, into whatever range the result lands in.
//
// An infinity or a NaN operand survives Frexp unchanged and propagates as it
// would have through the plain arithmetic, so Inf × 0 is still a NaN.

// smallestNormal is the smallest positive normal float64, 2⁻¹⁰²². Below it a
// float64 has fewer than 53 bits of significand, so a value that lands there has
// been rounded a second time.
const smallestNormal = 2.2250738585072014e-308

// roundedTwice reports whether p — a result formed as Ldexp(mantissa, e), where
// the mantissa was already rounded to 53 bits — has been rounded again. That is
// so exactly when it lands in the subnormal range, zero included: the true
// result may be a nonzero subnormal that a first rounding pushed onto the wrong
// side of a tie, or off the bottom of the range entirely.
func roundedTwice(p float64) bool {
	return math.Abs(p) < smallestNormal // false for an infinity and for a NaN
}

// assembleMul returns x × y × 2ᵉ, with x and y mantissa-scale (in [0.25, 1] in
// magnitude, or zero) and each already carrying its full significand.
//
// The straight Ldexp of x × y is exact whenever the result is normal: the
// product rounds once, and scaling by a power of two is then lossless. A
// subnormal result is where that stops being true — Ldexp would round the
// already-rounded product a second time — so the scale goes back onto the two
// sides instead, splitting the exponent between them, and the multiplication
// that follows rounds once, straight into the range the result belongs in.
func assembleMul(x, y float64, e int) float64 {
	p := math.Ldexp(x*y, e)
	if !roundedTwice(p) || x == 0 || y == 0 {
		return p
	}
	// Half the exponent on each side. A mantissa is within two powers of two of
	// 1 and a subnormal result puts e near −1022, so both halves stay well inside
	// the normal range, both Ldexps are exact, and the product is correctly
	// rounded. An e low enough to push a half out of that range is one whose true
	// result is orders of magnitude below the smallest subnormal, and it is zero
	// whichever way it is assembled.
	h := e / 2
	return math.Ldexp(x, h) * math.Ldexp(y, e-h)
}

// assembleDiv returns x ÷ y × 2ᵉ, under the same rule as [assembleMul]: exact
// while the quotient is normal, and split across numerator and denominator where
// it is subnormal — the numerator scaled down by half the exponent and the
// denominator up by the other half — so that the division rounds once rather
// than twice.
func assembleDiv(x, y float64, e int) float64 {
	q := math.Ldexp(x/y, e)
	if !roundedTwice(q) || x == 0 || y == 0 {
		return q
	}
	h := e / 2
	return math.Ldexp(x, h) / math.Ldexp(y, h-e)
}

// rescale returns m × (from / to): a magnitude carried in a unit of factor from,
// expressed in a unit of factor to. from == to returns m exactly, so a value is
// always expressible in the unit it already carries.
//
// The mantissas are combined in the same order as the plain m × from ÷ to, so
// the rounding is the plain expression's — the exponent split costs no accuracy,
// it only keeps the intermediate in range.
func rescale(m, from, to float64) float64 {
	if from == to {
		return m
	}
	fm, em := math.Frexp(m)
	ffrom, efrom := math.Frexp(from)
	fto, eto := math.Frexp(to)
	return assembleDiv(fm*ffrom, fto, em+efrom-eto)
}

// product returns (a × af) × (b × bf): two magnitudes multiplied in base units.
// The mantissas are grouped as the plain expression groups them, so the rounding
// is the plain expression's.
func product(a, af, b, bf float64) float64 {
	fa, ea := math.Frexp(a)
	faf, eaf := math.Frexp(af)
	fb, eb := math.Frexp(b)
	fbf, ebf := math.Frexp(bf)
	return assembleMul(fa*faf, fb*fbf, ea+eaf+eb+ebf)
}

// quotient returns (a × af) ÷ (b × bf): two magnitudes divided in base units.
func quotient(a, af, b, bf float64) float64 {
	fa, ea := math.Frexp(a)
	faf, eaf := math.Frexp(af)
	fb, eb := math.Frexp(b)
	fbf, ebf := math.Frexp(bf)
	return assembleDiv(fa*faf, fb*fbf, ea+eaf-eb-ebf)
}

// Scale returns v multiplied by a dimensionless factor. It reports no error, so
// — unlike [Value.Mul] — it does not guarantee a finite result: an f large
// enough to overflow the magnitude, an infinite f, or a NaN f yields a
// non-finite Value. Use Mul with a [Scalar] where the result must be checked.
func (v Value) Scale(f float64) Value { return Value{v.mag * f, v.unit} }

// Neg returns −v. It reports no error and cannot make a finite magnitude
// non-finite; a v that is already non-finite negates to a non-finite Value.
func (v Value) Neg() Value { return Value{-v.mag, v.unit} }

// Equal reports whether v and o represent the same quantity to within tol of
// the kind's base unit. Values of different kinds are never equal.
//
// Equal has no error to report, so it takes the difference before the base unit:
// it subtracts in a unit common to both operands and rescales only the
// difference. Two values whose base magnitudes both overflow have an ordinary
// difference, and a value is equal to itself whatever its magnitude.
//
// The answer does not depend on which operand is the receiver: the common unit
// is chosen by a property of the pair — the larger of the two factors — so
// swapping v and o negates the difference and leaves its magnitude alone.
func (v Value) Equal(o Value, tol float64) bool {
	if v.unit.kind != o.unit.kind {
		return false
	}
	vu, ou := v.Unit(), o.Unit()

	// The common unit is chosen by a rule that depends on the pair, not on the
	// order it is read in: the larger factor. Both operands rescale into it, so
	// swapping v and o negates the difference and leaves its magnitude alone.
	// Equal factors leave nothing to choose: both rescales are the identity and
	// the final rescale is by that same factor, whichever unit is named.
	cu := vu
	if ou.factor > vu.factor {
		cu = ou
	}
	d := rescale(v.mag, vu.factor, cu.factor) - rescale(o.mag, ou.factor, cu.factor)
	return math.Abs(rescale(d, cu.factor, 1)) <= tol
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
