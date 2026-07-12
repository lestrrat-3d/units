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

// ErrDivideByZero is returned by [Value.Div] when the divisor's base magnitude
// is zero, or when the quotient is not finite. [Value.Div] yields a finite
// quotient or an error, never an infinity or a NaN.
var ErrDivideByZero = errors.New("units: division by zero")

// Value is a magnitude paired with the [Unit] it is expressed in. The zero
// Value is 0 of the dimensionless unit [One].
type Value struct {
	mag  float64
	unit Unit
}

// New returns a Value of mag in unit u.
func New(mag float64, u Unit) Value { return Value{mag: mag, unit: u} }

// FromBase returns a Value equal to base (expressed in u's base unit), but
// carried in unit u. For example FromBase(1000, Meter) is 1 m.
func FromBase(base float64, u Unit) Value { return Value{mag: base / u.factor, unit: u} }

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

// Unit returns the value's unit.
func (v Value) Unit() Unit { return v.unit }

// Kind returns the kind of quantity the value measures.
func (v Value) Kind() Kind { return v.unit.kind }

// Base returns the magnitude expressed in the kind's base unit (mm for a
// length, mm² for an area, rad for an angle).
func (v Value) Base() float64 { return v.mag * v.unit.factor }

// In returns the magnitude expressed in unit u, or [ErrIncompatible] if u
// measures a different kind.
func (v Value) In(u Unit) (float64, error) {
	if v.unit.kind != u.kind {
		return 0, fmt.Errorf("%w: cannot express %s in %s", ErrIncompatible, v.unit.kind, u.kind)
	}
	return v.mag * v.unit.factor / u.factor, nil
}

// Convert returns the same quantity carried in unit u.
func (v Value) Convert(u Unit) (Value, error) {
	m, err := v.In(u)
	if err != nil {
		return Value{}, err
	}
	return Value{m, u}, nil
}

// Add returns v + o. The operands must be the same kind, with one carve-out: an
// [Angle] may be added to a [Dimensionless] value, because a radian really is a
// ratio of two lengths and theta + pi/2 is an angle. The result is expressed in
// v's unit, or — for that carve-out — in whichever operand's unit is the angle,
// so the sum is an angle whichever side it appeared on.
func (v Value) Add(o Value) (Value, error) { return v.combine(o, 1) }

// Sub returns v − o, under the same rules as [Value.Add].
func (v Value) Sub(o Value) (Value, error) { return v.combine(o, -1) }

// combine adds sign*o to v.
func (v Value) combine(o Value, sign float64) (Value, error) {
	if v.unit.kind == o.unit.kind {
		return Value{v.mag + sign*o.mag*o.unit.factor/v.unit.factor, v.unit}, nil
	}

	if isAngleScalarPair(v.unit.kind, o.unit.kind) {
		u := v.unit
		if v.unit.kind == Dimensionless {
			u = o.unit
		}
		return FromBase(v.Base()+sign*o.Base(), u), nil
	}

	return Value{}, fmt.Errorf("%w: cannot combine %s with %s", ErrIncompatible, v.unit.kind, o.unit.kind)
}

// isAngleScalarPair reports whether a and b are an angle and a dimensionless
// value, in either order — the one pair Add and Sub accept across kinds.
func isAngleScalarPair(a, b Kind) bool {
	return (a == Angle && b == Dimensionless) || (a == Dimensionless && b == Angle)
}

// Mul returns v × o: the magnitudes multiplied in base units, and the kinds
// composed. Millimeters(2).Mul(Millimeters(3)) is 6 mm², an [Area]. The result
// is carried in the base unit of the resulting kind.
func (v Value) Mul(o Value) (Value, error) {
	u := baseUnitFor(v.unit.kind.Mul(o.unit.kind))
	return Value{v.Base() * o.Base(), u}, nil
}

// Div returns v ÷ o: the magnitudes divided in base units, and the kinds
// composed. Volume divided by Area is a [Length]. The result is carried in the
// base unit of the resulting kind. The quotient is always finite: a zero base
// magnitude in the divisor is [ErrDivideByZero], and so is a divisor so small
// that the quotient would be an infinity or a NaN.
func (v Value) Div(o Value) (Value, error) {
	q := v.Base() / o.Base()
	if o.Base() == 0 || math.IsInf(q, 0) || math.IsNaN(q) {
		return Value{}, fmt.Errorf("%w: cannot divide %s by %s", ErrDivideByZero, v, o)
	}
	return Value{q, baseUnitFor(v.unit.kind.Div(o.unit.kind))}, nil
}

// Scale returns v multiplied by a dimensionless factor.
func (v Value) Scale(f float64) Value { return Value{v.mag * f, v.unit} }

// Neg returns −v.
func (v Value) Neg() Value { return Value{-v.mag, v.unit} }

// Equal reports whether v and o represent the same quantity to within tol of
// the kind's base unit. Values of different kinds are never equal.
func (v Value) Equal(o Value, tol float64) bool {
	if v.unit.kind != o.unit.kind {
		return false
	}
	return math.Abs(v.Base()-o.Base()) <= tol
}

// String renders the value as "<magnitude> <symbol>" (just the number for
// dimensionless values).
func (v Value) String() string {
	n := strconv.FormatFloat(v.mag, 'g', -1, 64)
	if v.unit.kind == Dimensionless {
		return n
	}
	return n + " " + v.unit.symbol
}
