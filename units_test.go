package units_test

import (
	"math"
	"testing"

	"github.com/lestrrat-3d/units"
	"github.com/stretchr/testify/require"
)

func TestConversionLength(t *testing.T) {
	in, err := units.Millimeters(100).In(units.Inch)
	require.NoError(t, err)
	require.InDelta(t, 100/25.4, in, 1e-9, "100mm in inch")

	m, err := units.Meters(1).In(units.Millimeter)
	require.NoError(t, err)
	require.InDelta(t, 1000, m, 1e-9, "1m in mm")

	ft, err := units.Inches(12).In(units.Foot)
	require.NoError(t, err)
	require.InDelta(t, 1, ft, 1e-9, "12in in ft")

	require.InDelta(t, 0.0254, units.Thous(1).Base(), 1e-9, "1 thou base")
}

func TestConversionAngle(t *testing.T) {
	r, err := units.Degrees(180).In(units.Radian)
	require.NoError(t, err)
	require.InDelta(t, math.Pi, r, 1e-9, "180deg in rad")

	d, err := units.Radians(math.Pi / 2).In(units.Degree)
	require.NoError(t, err)
	require.InDelta(t, 90, d, 1e-9, "pi/2 in deg")
}

func TestIncompatibleKinds(t *testing.T) {
	_, err := units.Millimeters(1).In(units.Degree)
	require.ErrorIs(t, err, units.ErrIncompatible)

	_, err = units.Millimeters(1).Add(units.Degrees(1))
	require.ErrorIs(t, err, units.ErrIncompatible, "add across kinds")

	_, err = units.Millimeters(1).Sub(units.SquareMillimeters(1))
	require.ErrorIs(t, err, units.ErrIncompatible, "sub a length and an area")

	_, err = units.Millimeters(1).Add(units.Scalar(1))
	require.ErrorIs(t, err, units.ErrIncompatible, "a length is not a bare number")

	_, err = units.SquareMillimeters(1).In(units.CubicMillimeter)
	require.ErrorIs(t, err, units.ErrIncompatible, "an area is not a volume")
}

func TestAngleScalarCarveOut(t *testing.T) {
	// Radians are physically dimensionless, so theta + pi/2 is an angle. That is
	// the one pair Add and Sub accept across kinds.
	sum, err := units.Radians(math.Pi).Add(units.Scalar(math.Pi / 2))
	require.NoError(t, err)
	require.Equal(t, units.Angle, sum.Kind(), "angle + scalar is an angle")
	require.InDelta(t, 1.5*math.Pi, sum.Base(), 1e-9, "sum base (rad)")

	// …and in the other order, the sum is still an angle.
	sum, err = units.Scalar(math.Pi).Add(units.Radians(math.Pi))
	require.NoError(t, err)
	require.Equal(t, units.Angle, sum.Kind(), "scalar + angle is an angle")
	require.InDelta(t, 2*math.Pi, sum.Base(), 1e-9, "sum base (rad)")

	diff, err := units.Degrees(180).Sub(units.Scalar(math.Pi / 2))
	require.NoError(t, err)
	require.Equal(t, units.Angle, diff.Kind())
	require.InDelta(t, 90, diff.Mag(), 1e-9, "expressed in the angle's own unit (deg)")
}

func TestArithmetic(t *testing.T) {
	sum, err := units.Millimeters(50).Add(units.Centimeters(5)) // 50mm + 50mm
	require.NoError(t, err)
	require.InDelta(t, 100, sum.Base(), 1e-9, "sum base")
	require.Equal(t, units.Millimeter, sum.Unit(), "sum keeps left-hand unit")

	diff, err := units.Meters(1).Sub(units.Millimeters(250))
	require.NoError(t, err)
	require.InDelta(t, 750, diff.Base(), 1e-9, "diff base")
	require.InDelta(t, 0.75, diff.Mag(), 1e-9, "diff mag (m)")

	require.InDelta(t, 30, units.Millimeters(10).Scale(3).Base(), 1e-9, "scale")
}

func TestFromBaseAndKind(t *testing.T) {
	v := units.FromBase(1000, units.Meter)
	require.InDelta(t, 1, v.Mag(), 1e-9, "from base mag")
	require.Equal(t, units.Length, v.Kind(), "kind")
	require.InDelta(t, 1000, v.Base(), 1e-9, "base magnitude")
}

func TestString(t *testing.T) {
	require.Equal(t, "100 mm", units.Millimeters(100).String())
	require.Equal(t, "1.5", units.Scalar(1.5).String())
}

func TestEqual(t *testing.T) {
	require.True(t, units.Meters(1).Equal(units.Millimeters(1000), 1e-9), "1m should equal 1000mm")
	require.False(t, units.Millimeters(1).Equal(units.Degrees(1), 1e-9), "length must never equal angle")
}

func TestSystem(t *testing.T) {
	m := units.Metric()
	require.Equal(t, units.Millimeter, m.UnitFor(units.Length), "metric length default")
	require.Equal(t, units.Degree, m.UnitFor(units.Angle), "metric angle default")

	imp := units.Imperial()
	require.InDelta(t, 1, imp.LengthFromBase(25.4).Mag(), 1e-9, "imperial length-from-base") // 25.4mm = 1in
	require.InDelta(t, 180, units.Metric().AngleFromBase(math.Pi).Mag(), 1e-9, "metric angle-from-base")
	require.InDelta(t, 2000, units.Metric().In(units.Meters(2)), 1e-9, "system In") // displayed in mm
}

func TestLookupAndDefine(t *testing.T) {
	u, ok := units.Lookup("mm")
	require.True(t, ok, "lookup mm")
	require.Equal(t, units.Millimeter, u)

	yard := units.Define("yd", units.Length, 914.4)
	u, ok = units.Lookup("yd")
	require.True(t, ok, "lookup yd")
	require.Equal(t, yard, u)

	ft, err := units.New(1, yard).In(units.Foot)
	require.NoError(t, err)
	require.InDelta(t, 3, ft, 1e-9, "1 yd in ft")

	// Redefining a symbol would change the meaning of every value naming it.
	require.Panics(t, func() { units.Define("mm", units.Length, 2) }, "redefining a built-in")
	require.Panics(t, func() { units.Define("yd", units.Length, 914.4) }, "redefining a custom unit")
}

func TestLookupRoundTrip(t *testing.T) {
	for _, u := range []units.Unit{
		units.One,
		units.Millimeter, units.Centimeter, units.Meter, units.Inch, units.Foot, units.Thou,
		units.SquareMillimeter, units.SquareCentimeter, units.SquareMeter, units.SquareInch,
		units.CubicMillimeter, units.CubicCentimeter, units.CubicMeter, units.CubicInch, units.Liter,
		units.Kilogram, units.Gram, units.Pound,
		units.KilogramPerCubicMillimeter, units.KilogramPerCubicMeter, units.GramPerCubicCentimeter,
		units.Radian, units.Degree,
	} {
		t.Run(u.String(), func(t *testing.T) {
			got, ok := units.Lookup(u.Symbol())
			require.True(t, ok, "every built-in symbol must round-trip through Lookup")
			require.Equal(t, u, got)
		})
	}
}

func TestBaseUnits(t *testing.T) {
	for _, tc := range []struct {
		kind units.Kind
		want units.Unit
	}{
		{units.Dimensionless, units.One},
		{units.Length, units.Millimeter},
		{units.Area, units.SquareMillimeter},
		{units.Volume, units.CubicMillimeter},
		{units.Mass, units.Kilogram},
		{units.Density, units.KilogramPerCubicMillimeter},
		{units.Angle, units.Radian},
	} {
		t.Run(tc.kind.String(), func(t *testing.T) {
			u, ok := units.BaseUnit(tc.kind)
			require.True(t, ok)
			require.Equal(t, tc.want, u)
			require.InDelta(t, 1, u.Factor(), 0, "a base unit has factor 1")
		})
	}
}

func TestMul(t *testing.T) {
	area, err := units.Millimeters(2).Mul(units.Millimeters(3))
	require.NoError(t, err)
	require.Equal(t, units.Area, area.Kind(), "length x length is an area")
	require.Equal(t, units.SquareMillimeter, area.Unit(), "carried in the base unit")
	require.InDelta(t, 6, area.Mag(), 1e-9)
	require.Equal(t, "6 mm^2", area.String())

	// Magnitudes multiply in base units: 1 m x 1 m is 1e6 mm².
	sq, err := units.Meters(1).Mul(units.Meters(1))
	require.NoError(t, err)
	require.Equal(t, units.Area, sq.Kind())
	require.InDelta(t, 1e6, sq.Base(), 1e-6, "1 m^2 in mm^2")
	m2, err := sq.In(units.SquareMeter)
	require.NoError(t, err)
	require.InDelta(t, 1, m2, 1e-9, "1 m^2")

	vol, err := units.SquareCentimeters(1).Mul(units.Centimeters(1))
	require.NoError(t, err)
	require.Equal(t, units.Volume, vol.Kind(), "area x length is a volume")
	require.InDelta(t, 1000, vol.Base(), 1e-9, "1 cm^3 in mm^3")

	// Scaling by a dimensionless value leaves the kind alone.
	twice, err := units.Millimeters(3).Mul(units.Scalar(2))
	require.NoError(t, err)
	require.Equal(t, units.Length, twice.Kind())
	require.InDelta(t, 6, twice.Base(), 1e-9)

	mass, err := units.Grams(2).Mul(units.Scalar(3))
	require.NoError(t, err)
	require.Equal(t, units.Mass, mass.Kind())
	require.InDelta(t, 0.006, mass.Base(), 1e-12, "6 g in kg")
}

func TestDiv(t *testing.T) {
	length, err := units.CubicMillimeters(12).Div(units.SquareMillimeters(3))
	require.NoError(t, err)
	require.Equal(t, units.Length, length.Kind(), "volume / area is a length")
	require.Equal(t, units.Millimeter, length.Unit())
	require.InDelta(t, 4, length.Mag(), 1e-9)

	density, err := units.Kilograms(2).Div(units.CubicMeters(1))
	require.NoError(t, err)
	require.Equal(t, units.Density, density.Kind(), "mass / volume is a density")
	require.Equal(t, units.KilogramPerCubicMillimeter, density.Unit())
	kgm3, err := density.In(units.KilogramPerCubicMeter)
	require.NoError(t, err)
	require.InDelta(t, 2, kgm3, 1e-9, "2 kg/m^3")

	ratio, err := units.Meters(1).Div(units.Millimeters(1))
	require.NoError(t, err)
	require.Equal(t, units.Dimensionless, ratio.Kind(), "length / length is a bare number")
	require.InDelta(t, 1000, ratio.Base(), 1e-9)

	// An unnamed kind has no registered base unit, but a value can still carry
	// it: the ad-hoc unit has factor 1 and prints the kind's exponent form.
	curvature, err := units.Scalar(1).Div(units.Millimeters(4))
	require.NoError(t, err)
	require.Equal(t, units.Dimensionless.Div(units.Length), curvature.Kind())
	require.InDelta(t, 0.25, curvature.Base(), 1e-9, "1/4 mm^-1")
	require.Equal(t, "L⁻¹", curvature.Unit().Symbol())
	_, ok := units.BaseUnit(curvature.Kind())
	require.False(t, ok, "no base unit is registered for an unnamed kind")
	_, ok = units.Lookup("L⁻¹")
	require.False(t, ok, "an ad-hoc unit is not added to the registry")

	// Round trip: multiplying the inverse length back by a length is a number.
	back, err := curvature.Mul(units.Millimeters(8))
	require.NoError(t, err)
	require.Equal(t, units.Dimensionless, back.Kind())
	require.InDelta(t, 2, back.Base(), 1e-9)
}

func TestDivideByZero(t *testing.T) {
	_, err := units.Millimeters(1).Div(units.Millimeters(0))
	require.ErrorIs(t, err, units.ErrDivideByZero)

	_, err = units.Millimeters(1).Div(units.Scalar(0))
	require.ErrorIs(t, err, units.ErrDivideByZero, "a zero scalar divisor")

	_, err = units.SquareMeters(1).Div(units.SquareMillimeters(0))
	require.ErrorIs(t, err, units.ErrDivideByZero, "zero in another unit is still zero")
}

func TestConversionDerived(t *testing.T) {
	for _, tc := range []struct {
		name string
		from units.Value
		to   units.Unit
		want float64
	}{
		{"mm^2 to in^2", units.SquareMillimeters(645.16), units.SquareInch, 1},
		{"in^2 to mm^2", units.SquareInches(1), units.SquareMillimeter, 645.16},
		{"cm^2 to mm^2", units.SquareCentimeters(1), units.SquareMillimeter, 100},
		{"m^2 to cm^2", units.SquareMeters(1), units.SquareCentimeter, 10000},
		{"cm^3 to L", units.CubicCentimeters(1000), units.Liter, 1},
		{"L to cm^3", units.Liters(1), units.CubicCentimeter, 1000},
		{"m^3 to L", units.CubicMeters(1), units.Liter, 1000},
		{"in^3 to mm^3", units.CubicInches(1), units.CubicMillimeter, 16387.064},
		{"lb to g", units.Pounds(1), units.Gram, 453.59237},
		{"g to kg", units.Grams(500), units.Kilogram, 0.5},
		{"g/cm^3 to kg/m^3", units.GramsPerCubicCentimeter(1), units.KilogramPerCubicMeter, 1000},
		{"kg/m^3 to g/cm^3", units.KilogramsPerCubicMeter(7850), units.GramPerCubicCentimeter, 7.85},
		{"kg/mm^3 to kg/m^3", units.KilogramsPerCubicMillimeter(1), units.KilogramPerCubicMeter, 1e9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.from.In(tc.to)
			require.NoError(t, err)
			require.InDelta(t, tc.want, got, math.Abs(tc.want)*1e-9+1e-9)

			c, err := tc.from.Convert(tc.to)
			require.NoError(t, err)
			require.Equal(t, tc.to, c.Unit(), "Convert carries the target unit")
			require.True(t, c.Equal(tc.from, math.Abs(tc.from.Base())*1e-9+1e-9), "Convert preserves the quantity")
		})
	}
}

func TestSystemDerivedKinds(t *testing.T) {
	// A system has no configured area unit, so an area is presented in its base
	// unit rather than being silently reinterpreted as a bare number.
	m := units.Metric()
	require.Equal(t, units.SquareMillimeter, m.UnitFor(units.Area))
	require.Equal(t, units.Kilogram, m.UnitFor(units.Mass))
	require.Equal(t, units.One, m.UnitFor(units.Dimensionless.Div(units.Length)), "an unnamed kind has no default unit")

	area, err := units.Meters(1).Mul(units.Meters(1))
	require.NoError(t, err)
	require.InDelta(t, 1e6, m.In(area), 1e-6, "1 m^2 displayed in mm^2")
	require.Equal(t, units.SquareMillimeter, m.Display(area).Unit())
}
