package units_test

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"sync"
	"testing"
	"unicode/utf8"

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

// builtinUnits lists every unit the package itself registers.
func builtinUnits() []units.Unit {
	return []units.Unit{
		units.One,
		units.Millimeter, units.Centimeter, units.Meter, units.Inch, units.Foot, units.Thou,
		units.SquareMillimeter, units.SquareCentimeter, units.SquareMeter, units.SquareInch,
		units.CubicMillimeter, units.CubicCentimeter, units.CubicMeter, units.CubicInch, units.Liter,
		units.Kilogram, units.Gram, units.Pound,
		units.KilogramPerCubicMillimeter, units.KilogramPerCubicMeter, units.GramPerCubicCentimeter,
		units.KilogramSquareMillimeter, units.QuarticMillimeter,
		units.Radian, units.Degree,
	}
}

// everydayMags are the magnitudes a caller actually writes. The extremes have
// their own sweep; accuracy is judged here, where nothing can be excused as the
// float64 range running out.
func everydayMags() []float64 {
	return []float64{0.001, 0.5, 1, 2, 3.7, 25.4, 100, 1000, 7850, math.Pi}
}

func TestLookupRoundTrip(t *testing.T) {
	for _, u := range builtinUnits() {
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
		{units.MomentOfInertia, units.KilogramSquareMillimeter},
		{units.SecondMomentOfArea, units.QuarticMillimeter},
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
	// it: the synthetic unit has factor 1 and an ASCII, bracketed symbol.
	curvature, err := units.Scalar(1).Div(units.Millimeters(4))
	require.NoError(t, err)
	require.Equal(t, units.Dimensionless.Div(units.Length), curvature.Kind())
	require.InDelta(t, 0.25, curvature.Base(), 1e-9, "1/4 mm^-1")
	require.Equal(t, "[L^-1]", curvature.Unit().Symbol())
	_, ok := units.BaseUnit(curvature.Kind())
	require.False(t, ok, "no base unit is registered for an unnamed kind")
	_, ok = units.Lookup("[L^-1]")
	require.False(t, ok, "a synthetic unit is not added to the registry")

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

	// A divisor that is nonzero in its own unit is a real divisor, even where its
	// base magnitude underflows to zero: it is the quotient that overflows, so the
	// error says overflow rather than a zero divisor.
	_, err = units.Millimeters(1).Div(units.KilogramsPerCubicMeter(1e-320))
	require.ErrorIs(t, err, units.ErrNotFinite, "a divisor whose base magnitude underflows is not zero")
	require.NotErrorIs(t, err, units.ErrDivideByZero, "the divisor is not zero")

	// …and where the quotient is representable, it comes back: a value divided by
	// itself is 1 however small its base magnitude.
	q, err := units.Grams(1e-322).Div(units.Grams(1e-322))
	require.NoError(t, err, "a divisor whose base magnitude underflows to zero still divides")
	require.Equal(t, units.Scalar(1), q)

	// …and where both base magnitudes underflow, the quotient of the magnitudes
	// themselves is still the answer. 1e-320 is a subnormal, so it is the float64
	// beside it — not the decimal — that the quotient is taken against.
	tiny := units.KilogramsPerCubicMeter(1e-320)
	q, err = units.KilogramsPerCubicMeter(1e-300).Div(tiny)
	require.NoError(t, err)
	require.InEpsilon(t, 1e-300/tiny.Mag(), q.Mag(), 1e-15, "both base magnitudes underflow; the quotient does not")

	// A subnormal divisor does not underflow, but blows the quotient up to +Inf.
	// The contract is a finite quotient or an error, never an infinity — and the
	// error says what actually happened, which is an overflow, not a zero divisor.
	_, err = units.Millimeters(1).Div(units.Kilograms(5e-324))
	require.ErrorIs(t, err, units.ErrNotFinite, "a divisor that overflows the quotient")
	require.NotErrorIs(t, err, units.ErrDivideByZero, "the divisor is not zero")

	_, err = units.Millimeters(-1).Div(units.Kilograms(5e-324))
	require.ErrorIs(t, err, units.ErrNotFinite, "…and in the negative direction")

	// The finite quotients around it still divide.
	q, err = units.Millimeters(1).Div(units.Kilograms(2))
	require.NoError(t, err)
	require.InDelta(t, 0.5, q.Base(), 1e-9)
}

func TestMulNotFinite(t *testing.T) {
	// A product is finite or it is an error. A non-finite magnitude would carry a
	// registered symbol ("+Inf mm^2"), so — unlike a synthetic unit — it would be
	// perfectly persistable, and nothing downstream would ever know.
	for _, tc := range []struct {
		name string
		mul  func() (units.Value, error)
	}{
		{"overflow to +Inf", func() (units.Value, error) { return units.Meters(1e200).Mul(units.Meters(1e200)) }},
		{"overflow to -Inf", func() (units.Value, error) { return units.Meters(-1e200).Mul(units.Meters(1e200)) }},
		{"both negative", func() (units.Value, error) { return units.Meters(-1e200).Mul(units.Meters(-1e200)) }},
		{"an area squared", func() (units.Value, error) {
			return units.SquareMillimeters(1e300).Mul(units.SquareMillimeters(1e300))
		}},
		{"NaN from Inf x 0", func() (units.Value, error) {
			return units.Scalar(math.Inf(1)).Mul(units.Scalar(0))
		}},
		{"NaN from -Inf x 0", func() (units.Value, error) {
			return units.Millimeters(math.Inf(-1)).Mul(units.Scalar(0))
		}},
		{"a NaN operand", func() (units.Value, error) {
			return units.Millimeters(math.NaN()).Mul(units.Millimeters(2))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, err := tc.mul()
			require.ErrorIs(t, err, units.ErrNotFinite)
			require.Equal(t, units.Value{}, v, "no value escapes with the error")
		})
	}

	// The finite products around them still multiply.
	a, err := units.Meters(1e150).Mul(units.Meters(1e150))
	require.NoError(t, err)
	require.Equal(t, units.Area, a.Kind())
	require.False(t, math.IsInf(a.Base(), 0), "a finite product stays finite")
}

func TestZeroValue(t *testing.T) {
	// The zero Value is 0 of One: its unit's factor is 1, not 0, so it does not
	// blow every arithmetic it takes part in up to an infinity.
	var v units.Value
	require.Equal(t, units.One, v.Unit(), "the zero Value carries One")
	require.Equal(t, units.Dimensionless, v.Kind())
	require.InDelta(t, 1, v.Unit().Factor(), 0, "the zero Value's unit has factor 1")
	sameFloat64f(t, 0, v.Mag(), "the zero Value is a positive zero")
	sameFloat64f(t, 0, v.Base(), "…in base units too")
	require.Equal(t, "0", v.String())

	sum, err := v.Add(units.Scalar(5))
	require.NoError(t, err)
	require.InDelta(t, 5, sum.Base(), 1e-12, "0 + 5 is 5, not +Inf")
	require.Equal(t, units.Dimensionless, sum.Kind())

	sum, err = units.Scalar(5).Add(v)
	require.NoError(t, err)
	require.InDelta(t, 5, sum.Base(), 1e-12)

	diff, err := v.Sub(units.Scalar(5))
	require.NoError(t, err)
	require.InDelta(t, -5, diff.Base(), 1e-12)

	// It scales, converts and compares like any other dimensionless value — and every
	// zero it produces is a positive zero, the negation included.
	sameFloat64f(t, 0, v.Scale(3).Base(), "a zero scaled is a zero")
	sameFloat64f(t, 0, v.Neg().Base(), "the negation of a zero is a positive zero")
	require.True(t, v.Equal(units.Scalar(0), 0), "the zero Value equals a zero scalar")

	one, err := v.In(units.One)
	require.NoError(t, err)
	sameFloat64f(t, 0, one, "a zero converts to a zero")

	// The angle carve-out reaches it too: 0 + 90deg is 90deg, not +Inf.
	ang, err := v.Add(units.Degrees(90))
	require.NoError(t, err)
	require.Equal(t, units.Angle, ang.Kind())
	require.InDelta(t, 90, ang.Mag(), 1e-12)

	// It multiplies and divides without fabricating a NaN.
	p, err := v.Mul(units.Millimeters(3))
	require.NoError(t, err)
	require.Equal(t, units.Length, p.Kind())
	sameFloat64f(t, 0, p.Base(), "a zero times a length is a zero length")

	q, err := units.Millimeters(3).Div(units.Scalar(2))
	require.NoError(t, err)
	require.InDelta(t, 1.5, q.Base(), 1e-12)

	// …and dividing by it is a division by zero, not an infinity.
	_, err = units.Millimeters(3).Div(v)
	require.ErrorIs(t, err, units.ErrDivideByZero)
}

func TestDefineRejectsBadFactor(t *testing.T) {
	// A unit whose factor is zero, negative or non-finite cannot convert: every
	// magnitude expressed in it would come back an infinity or a NaN. That is a
	// programming error, like a duplicate symbol, so Define panics on it.
	for _, tc := range []struct {
		name   string
		symbol string
		factor float64
	}{
		{"zero", "zz-zero", 0},
		{"negative", "zz-neg", -1},
		{"+Inf", "zz-inf", math.Inf(1)},
		{"-Inf", "zz-neginf", math.Inf(-1)},
		{"NaN", "zz-nan", math.NaN()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Panics(t, func() { units.Define(tc.symbol, units.Length, tc.factor) })
			_, ok := units.Lookup(tc.symbol)
			require.False(t, ok, "a rejected unit must not be registered")
		})
	}

	// A positive, finite factor is fine, however small or large.
	u := units.Define("zz-ok", units.Length, 1e-9)
	require.InDelta(t, 1e-9, u.Factor(), 0)
}

func TestDefineRejectsOverflowedKind(t *testing.T) {
	// An overflowed kind is a programming error made visible: it has no base unit
	// and is carried only by the unregistered synthetic symbol "[overflow]", so it
	// can be neither resolved nor persisted. Registering one under an ordinary
	// symbol would launder it into a legitimate, resolvable, persistable unit — so
	// Define refuses it, as it refuses a duplicate symbol or an unusable factor.
	for _, tc := range []struct {
		name   string
		symbol string
		kind   units.Kind
	}{
		{"Pow past the exponent range", "zz-ovf-pow", units.Length.Pow(math.MaxInt64)},
		{"Pow the other way", "zz-ovf-negpow", units.Length.Pow(math.MinInt64)},
		{"Mul that saturates", "zz-ovf-mul", units.Length.Pow(120).Mul(units.Length.Pow(120))},
		{"Div that saturates", "zz-ovf-div", units.Length.Pow(-120).Div(units.Length.Pow(120))},
		{"sticky: composed back to zero exponents", "zz-ovf-sticky",
			units.Length.Pow(math.MaxInt64).Div(units.Length.Pow(math.MaxInt64))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.True(t, tc.kind.Overflowed(), "the premise: the kind has overflowed")

			require.Panics(t, func() { units.Define(tc.symbol, tc.kind, 1) },
				"Define refuses an overflowed kind")
			require.Panics(t, func() { units.Define(tc.symbol, tc.kind, 25.4) },
				"…whatever the factor")

			// Nothing may survive the rejection: not the symbol, not a base unit.
			_, ok := units.Lookup(tc.symbol)
			require.False(t, ok, "a rejected unit must not be registered")
			_, ok = units.BaseUnit(tc.kind)
			require.False(t, ok, "and an overflowed kind still has no base unit")
		})
	}

	// The kind that merely looks exotic is not the kind that overflowed: an L¹²⁰ is
	// an int8 exponent, and a unit of it registers as any other does.
	require.False(t, units.Length.Pow(120).Overflowed())
	u := units.Define("zz-L120b", units.Length.Pow(120), 1)
	got, ok := units.Lookup("zz-L120b")
	require.True(t, ok)
	require.Equal(t, u, got)
}

func TestRegistryConcurrent(t *testing.T) {
	// Define writes the registry that Lookup and BaseUnit read. An application may
	// register its units from one goroutine while another deserializes symbols, so
	// the registry is guarded; run under -race, this is the proof.
	const n = 64

	// Assertions belong to the test goroutine, so the readers report what they saw
	// rather than failing from a goroutine of their own.
	seen := make(chan units.Unit, 2*n)

	var wg sync.WaitGroup
	for i := range n {
		wg.Add(3)

		go func() {
			defer wg.Done()
			units.Define("zz-race-"+strconv.Itoa(i), units.Length, float64(i)+1)
		}()
		go func() {
			defer wg.Done()
			u, _ := units.Lookup("mm")
			units.Lookup("zz-race-" + strconv.Itoa(i)) // may or may not be defined yet
			seen <- u
		}()
		go func() {
			defer wg.Done()
			u, _ := units.BaseUnit(units.Area)
			seen <- u
		}()
	}
	wg.Wait()
	close(seen)

	for u := range seen {
		require.Contains(t, []units.Unit{units.Millimeter, units.SquareMillimeter}, u,
			"a concurrent reader never sees a torn or missing built-in")
	}

	for i := range n {
		u, ok := units.Lookup("zz-race-" + strconv.Itoa(i))
		require.True(t, ok, "every concurrently defined unit is registered")
		require.InDelta(t, float64(i)+1, u.Factor(), 0)
	}
}

func TestNamedKindsHaveBaseUnits(t *testing.T) {
	// Every named kind has a registered base unit, so no composition of named
	// kinds ever falls back to a synthetic unit.
	for _, k := range []units.Kind{
		units.Dimensionless, units.Length, units.Area, units.Volume, units.Angle,
		units.Mass, units.Density, units.MomentOfInertia, units.SecondMomentOfArea,
	} {
		t.Run(k.String(), func(t *testing.T) {
			u, ok := units.BaseUnit(k)
			require.True(t, ok, "a named kind must have a base unit")
			require.Equal(t, k, u.Kind())
			require.InDelta(t, 1, u.Factor(), 0, "a base unit has factor 1")

			got, ok := units.Lookup(u.Symbol())
			require.True(t, ok, "a base unit's symbol must resolve through Lookup")
			require.Equal(t, u, got)
		})
	}
}

func TestComposedNamedKinds(t *testing.T) {
	// mass x area is a moment of inertia, and it lands in a registered unit
	// whose symbol round-trips — not in a synthetic one named after the kind.
	moi, err := units.Kilograms(2).Mul(units.SquareMillimeters(3))
	require.NoError(t, err)
	require.Equal(t, units.MomentOfInertia, moi.Kind())
	require.Equal(t, units.KilogramSquareMillimeter, moi.Unit())
	require.Equal(t, "kg*mm^2", moi.Unit().Symbol())
	require.InDelta(t, 6, moi.Mag(), 1e-9)
	u, ok := units.Lookup("kg*mm^2")
	require.True(t, ok, "the moment-of-inertia base unit is registered")
	require.Equal(t, units.KilogramSquareMillimeter, u)

	// area x area is a second moment of area, likewise.
	smoa, err := units.SquareMillimeters(2).Mul(units.SquareMillimeters(3))
	require.NoError(t, err)
	require.Equal(t, units.SecondMomentOfArea, smoa.Kind())
	require.Equal(t, units.QuarticMillimeter, smoa.Unit())
	require.Equal(t, "6 mm^4", smoa.String())
	u, ok = units.Lookup("mm^4")
	require.True(t, ok, "the second-moment-of-area base unit is registered")
	require.Equal(t, units.QuarticMillimeter, u)

	// A system presents them in their base units rather than as bare numbers.
	m := units.Metric()
	require.Equal(t, units.KilogramSquareMillimeter, m.UnitFor(units.MomentOfInertia))
	require.Equal(t, units.QuarticMillimeter, m.UnitFor(units.SecondMomentOfArea))
}

func TestSyntheticUnitSymbol(t *testing.T) {
	// A value of an unnamed kind carries a synthetic unit whose symbol is ASCII,
	// bracketed, and unmistakably not a real unit — never the kind's prose name.
	for _, tc := range []struct {
		name string
		val  func() (units.Value, error)
		want string
	}{
		{
			name: "inverse length",
			val:  func() (units.Value, error) { return units.Scalar(1).Div(units.Millimeters(4)) },
			want: "[L^-1]",
		},
		{
			name: "inverse area",
			val:  func() (units.Value, error) { return units.Scalar(1).Div(units.SquareMillimeters(4)) },
			want: "[L^-2]",
		},
		{
			name: "areal density",
			val:  func() (units.Value, error) { return units.Kilograms(1).Div(units.SquareMillimeters(4)) },
			want: "[L^-2*M]",
		},
		{
			name: "length per angle",
			val:  func() (units.Value, error) { return units.Millimeters(1).Div(units.Radians(4)) },
			want: "[L*A^-1]",
		},
		{
			name: "mass angle",
			val:  func() (units.Value, error) { return units.Kilograms(1).Mul(units.Radians(4)) },
			want: "[M*A]",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, err := tc.val()
			require.NoError(t, err)
			require.Equal(t, tc.want, v.Unit().Symbol())
			for _, r := range v.Unit().Symbol() {
				require.Less(t, r, rune(utf8.RuneSelf), "a unit symbol is ASCII")
			}

			// The synthetic symbol is not a registry key, and the kind still
			// honestly reports that it has no base unit.
			_, ok := units.Lookup(v.Unit().Symbol())
			require.False(t, ok, "a synthetic unit is not registered")
			_, ok = units.BaseUnit(v.Kind())
			require.False(t, ok, "an unnamed kind has no base unit")

			// Kind.String stays display text; it is never the unit symbol.
			require.NotEqual(t, v.Kind().String(), v.Unit().Symbol())
			require.InDelta(t, 1, v.Unit().Factor(), 0, "a synthetic unit has factor 1")
		})
	}
}

// wideLength is an application-defined unit of an exotic-but-representable kind:
// L¹²⁰. Multiplying two of its values composes L²⁴⁰, which no int8 exponent
// holds — the only way to reach an overflowed kind from the Value API.
var wideLength = units.Define("L120", units.Length.Pow(120), 1)

func TestOverflowedKindHasNoUnit(t *testing.T) {
	// An overflowed kind is a programming error made visible. Nothing about it may
	// look like a real quantity: no name, no base unit, no resolvable symbol, and
	// no presentation unit of some other kind that a consumer could read as one.
	v, err := units.New(2, wideLength).Mul(units.New(3, wideLength))
	require.NoError(t, err, "the magnitudes are ordinary; it is the kind that overflowed")
	require.InDelta(t, 6, v.Mag(), 0)

	k := v.Kind()
	require.True(t, k.Overflowed(), "L²⁴⁰ does not fit an int8 exponent")
	require.Equal(t, "overflowed", k.String())
	require.NotEmpty(t, k.String())

	_, ok := units.BaseUnit(k)
	require.False(t, ok, "an overflowed kind has no base unit")

	require.Equal(t, "[overflow]", v.Unit().Symbol(), "and carries the reserved synthetic symbol")
	for _, r := range v.Unit().Symbol() {
		require.Less(t, r, rune(utf8.RuneSelf), "a unit symbol is ASCII")
	}
	_, ok = units.Lookup(v.Unit().Symbol())
	require.False(t, ok, "the synthetic symbol is not a registry key")
	require.Panics(t, func() { units.Define("[overflow]", units.Length, 1) },
		"the bracketed namespace is reserved, so nothing can hijack it")

	// Presentation never hands back a unit of some other kind — the rule that keeps
	// an overflowed quantity from being read as a length or a bare number.
	for _, sys := range []units.System{units.Metric(), units.SI(), units.Imperial(), {}} {
		u := sys.UnitFor(k)
		require.Equal(t, k, u.Kind(), "UnitFor answers in the kind it was asked about")
		require.True(t, u.Kind().Overflowed())
		require.Equal(t, "[overflow]", u.Symbol())
		require.Equal(t, k, sys.Display(v).Kind(), "Display preserves the kind")
	}

	// It never equals a named kind, so it can neither be added to one nor converted
	// into one.
	for _, named := range namedKinds() {
		require.NotEqual(t, named, k)
		u, ok := units.BaseUnit(named)
		require.True(t, ok)
		_, err := v.In(u)
		require.ErrorIs(t, err, units.ErrIncompatible, "an overflowed quantity is not a %s", named)
	}
	_, err = v.Add(units.Millimeters(1))
	require.ErrorIs(t, err, units.ErrIncompatible)
}

func TestReservedSymbolNamespace(t *testing.T) {
	// Bracketed symbols belong to the library: a consumer that could register one
	// could make a persisted synthetic symbol deserialize as a different kind.
	for _, symbol := range []string{"[L^-1]", "[", "[L^2*M]", "[anything]"} {
		t.Run(symbol, func(t *testing.T) {
			require.Panics(t, func() { units.Define(symbol, units.Length, 999) },
				"the bracketed namespace is reserved")
			_, ok := units.Lookup(symbol)
			require.False(t, ok, "a rejected symbol must not be registered")
		})
	}

	// The kind's prose name is not a unit symbol either, so nothing composed can
	// collide with one.
	_, ok := units.Lookup("moment of inertia")
	require.False(t, ok, "a kind name is never a registered symbol")
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

	// An unnamed kind has no registered base unit, but its presentation unit
	// still measures that kind: a default unit is never a different kind.
	curvature := units.Dimensionless.Div(units.Length)
	u := m.UnitFor(curvature)
	require.Equal(t, curvature, u.Kind(), "UnitFor preserves the kind of an unnamed kind")
	require.InDelta(t, 1, u.Factor(), 0, "the synthetic presentation unit has factor 1")
	require.Equal(t, "[L^-1]", u.Symbol())

	area, err := units.Meters(1).Mul(units.Meters(1))
	require.NoError(t, err)
	require.InDelta(t, 1e6, m.In(area), 1e-6, "1 m^2 displayed in mm^2")
	require.Equal(t, units.SquareMillimeter, m.Display(area).Unit())
}

func TestSystemNeverCoercesKind(t *testing.T) {
	// Routing a value through the system's default unit must not change what it
	// measures. If UnitFor handed back a unit of another kind, a length or a
	// curvature could be rebuilt as a dimensionless value — and then added to an
	// angle, because Add carves out angle + dimensionless. It would silently
	// become an angle, with a nil error the whole way.
	//
	// System has exported fields and a usable zero value, so every one of these
	// is an ordinary construction, not a contrivance.
	curvature := units.Dimensionless.Div(units.Length)
	kinds := []units.Kind{
		units.Dimensionless, units.Length, units.Angle, units.Area, units.Volume,
		units.Mass, units.Density, units.MomentOfInertia, units.SecondMomentOfArea,
		curvature,
		units.Mass.Div(units.Area),
		units.Length.Div(units.Angle),
		units.Length.Pow(5),
	}

	for _, sc := range []struct {
		name string
		sys  units.System
	}{
		{"metric", units.Metric()},
		{"si", units.SI()},
		{"imperial", units.Imperial()},
		{"zero value", units.System{}},
		{"angle unset", units.System{Length: units.Meter}},
		{"length unset", units.System{Angle: units.Degree}},
		{"length field holds an angle", units.System{Length: units.Degree, Angle: units.Degree}},
		{"angle field holds a length", units.System{Length: units.Meter, Angle: units.Meter}},
		{"both fields hold the wrong kind", units.System{Length: units.Kilogram, Angle: units.SquareMeter}},
	} {
		t.Run(sc.name, func(t *testing.T) {
			sys := sc.sys

			for _, k := range kinds {
				require.Equal(t, k, sys.UnitFor(k).Kind(),
					"a default unit always measures the kind asked for: %s", k)
				require.NotZero(t, sys.UnitFor(k).Factor(), "a default unit is usable: %s", k)
			}

			// The exploit chain, run in full: a 5 mm length round-tripped through
			// the system's presentation unit is still a length, and a length plus
			// an angle is still an error.
			length := units.Millimeters(5)
			round := units.New(length.Mag(), sys.UnitFor(length.Kind()))
			require.Equal(t, units.Length, round.Kind(), "a round trip through UnitFor keeps a length a length")

			_, err := round.Add(units.Degrees(90))
			require.ErrorIs(t, err, units.ErrIncompatible, "a length is not an angle")

			// And the same for an angle, which must not decay to a bare number
			// either.
			angle := units.Degrees(90)
			roundA := units.New(angle.Mag(), sys.UnitFor(angle.Kind()))
			require.Equal(t, units.Angle, roundA.Kind(), "a round trip through UnitFor keeps an angle an angle")

			_, err = roundA.Add(units.Millimeters(1))
			require.ErrorIs(t, err, units.ErrIncompatible, "an angle is not a length")

			// In and Display preserve the kind and the quantity, on every system.
			for _, v := range []units.Value{length, angle, units.Meters(2), units.SquareMeters(1), units.Kilograms(3)} {
				d := sys.Display(v)
				require.Equal(t, v.Kind(), d.Kind(), "Display preserves the kind: %s", v)
				require.True(t, d.Equal(v, math.Abs(v.Base())*1e-9+1e-9), "Display preserves the quantity: %s", v)

				want, err := v.In(sys.UnitFor(v.Kind()))
				require.NoError(t, err, "the default unit always measures v's kind")
				require.InDelta(t, want, sys.In(v), math.Abs(want)*1e-9+1e-9, "In agrees with the default unit: %s", v)
			}

			// FromBase wrappers land on the kind they name.
			require.Equal(t, units.Length, sys.LengthFromBase(25.4).Kind())
			require.InDelta(t, 25.4, sys.LengthFromBase(25.4).Base(), 1e-9)
			require.Equal(t, units.Angle, sys.AngleFromBase(math.Pi).Kind())
			require.InDelta(t, math.Pi, sys.AngleFromBase(math.Pi).Base(), 1e-9)
		})
	}

	// An unnamed kind is presented in its synthetic factor-1 unit, never in One.
	sys := units.Metric()
	c, err := units.Scalar(1).Div(units.Millimeters(4))
	require.NoError(t, err)
	require.Equal(t, curvature, c.Kind())

	round := units.New(c.Mag(), sys.UnitFor(c.Kind()))
	require.Equal(t, c.Kind(), round.Kind(), "a round trip through UnitFor keeps the kind")
	require.InDelta(t, c.Base(), round.Base(), 1e-12, "…and the quantity")

	_, err = round.Add(units.Degrees(90))
	require.ErrorIs(t, err, units.ErrIncompatible, "a curvature is not an angle")

	require.Equal(t, c.Kind(), sys.Display(c).Kind())
	require.InDelta(t, 0.25, sys.In(c), 1e-12)
}

func TestAddNotFinite(t *testing.T) {
	// A sum is finite or it is an error, for the same reason a product is: an
	// "+Inf m" carries a registered symbol, so it persists exactly like a real
	// quantity and nothing downstream can tell it apart.
	maxf := math.MaxFloat64

	for _, tc := range []struct {
		name string
		op   func() (units.Value, error)
	}{
		{"add overflows to +Inf", func() (units.Value, error) { return units.Meters(maxf).Add(units.Meters(maxf)) }},
		{"add overflows to -Inf", func() (units.Value, error) { return units.Meters(-maxf).Add(units.Meters(-maxf)) }},
		{"add, operands swapped", func() (units.Value, error) { return units.Meters(maxf).Add(units.Millimeters(maxf)) }},
		{"add, operands swapped the other way", func() (units.Value, error) {
			return units.Millimeters(maxf).Add(units.Meters(maxf))
		}},
		{"sub overflows to -Inf", func() (units.Value, error) { return units.Meters(-maxf).Sub(units.Meters(maxf)) }},
		{"sub overflows to +Inf", func() (units.Value, error) { return units.Meters(maxf).Sub(units.Meters(-maxf)) }},
		{"sub, operands swapped", func() (units.Value, error) { return units.Meters(-maxf).Sub(units.Millimeters(maxf)) }},
		{"an area sum", func() (units.Value, error) {
			return units.SquareMeters(maxf).Add(units.SquareMeters(maxf))
		}},
		{"the angle/scalar carve-out overflows", func() (units.Value, error) {
			return units.Radians(maxf).Add(units.Scalar(maxf))
		}},
		{"the carve-out, angle on the right", func() (units.Value, error) {
			return units.Scalar(maxf).Add(units.Radians(maxf))
		}},
		{"a NaN operand", func() (units.Value, error) { return units.Millimeters(math.NaN()).Add(units.Millimeters(2)) }},
		{"an infinite operand", func() (units.Value, error) {
			return units.Millimeters(math.Inf(1)).Sub(units.Millimeters(2))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, err := tc.op()
			require.ErrorIs(t, err, units.ErrNotFinite)
			require.Equal(t, units.Value{}, v, "no value escapes with the error")
		})
	}

	// The finite sums around them still add.
	s, err := units.Meters(1e308).Add(units.Meters(-1e308))
	require.NoError(t, err)
	require.Equal(t, units.Length, s.Kind())
	sameFloat64f(t, 0, s.Mag(), "the sum cancels to a positive zero")

	s, err = units.Meters(1).Add(units.Millimeters(500))
	require.NoError(t, err)
	require.InDelta(t, 1.5, s.Mag(), 1e-9)
}

func TestConversionNotFinite(t *testing.T) {
	// In and Convert report an error, so they too refuse to hand back an infinity.
	_, err := units.New(math.Inf(1), units.Meter).In(units.Millimeter)
	require.ErrorIs(t, err, units.ErrNotFinite)

	_, err = units.New(math.NaN(), units.Meter).Convert(units.Millimeter)
	require.ErrorIs(t, err, units.ErrNotFinite)

	// An overflow in the conversion itself, from a finite magnitude.
	_, err = units.New(math.MaxFloat64, units.Meter).In(units.Thou)
	require.ErrorIs(t, err, units.ErrNotFinite)

	// A cross-kind conversion is still ErrIncompatible, not ErrNotFinite.
	_, err = units.New(math.Inf(1), units.Meter).In(units.Degree)
	require.ErrorIs(t, err, units.ErrIncompatible)
}

// turn is an ordinary application-defined unit: one revolution, 2π radians. A
// magnitude in it that float64 holds easily has a base magnitude (in radians)
// that it does not.
var turn = units.Define("turn", units.Angle, 2*math.Pi)

func TestBaseMagnitudeIsNeverAnIntermediate(t *testing.T) {
	// Meters(1e307) is an ordinary length: a finite magnitude in a built-in unit.
	// Its base magnitude — 1e310 mm — is +Inf, because float64 stops at ~1.8e308.
	// It is the result of an operation that has to be representable, never an
	// intermediate, so no operation may route through Value.Base.
	huge := units.Meters(1e307)
	require.True(t, math.IsInf(huge.Base(), 1), "the premise: an ordinary value whose base magnitude overflows")

	t.Run("In: a value in the unit it already carries", func(t *testing.T) {
		m, err := huge.In(units.Meter)
		require.NoError(t, err, "a value is always expressible in its own unit")
		sameFloat64f(t, 1e307, m, "a value in its own unit is its own magnitude")
	})

	t.Run("In: a conversion whose result is representable", func(t *testing.T) {
		// 1e310 mm is 3.28e307 ft — comfortably in range, though the base is not.
		m, err := huge.In(units.Foot)
		require.NoError(t, err)
		require.InEpsilon(t, 1e307*(1000.0/304.8), m, 1e-12)
	})

	t.Run("Convert: the same, carried in the unit", func(t *testing.T) {
		c, err := huge.Convert(units.Meter)
		require.NoError(t, err)
		require.Equal(t, units.Meter, c.Unit())
		sameFloat64f(t, 1e307, c.Mag(), "Convert carries the same magnitude")
	})

	t.Run("Div: a value divided by itself is 1", func(t *testing.T) {
		q, err := huge.Div(huge)
		require.NoError(t, err)
		require.Equal(t, units.Dimensionless, q.Kind())
		sameFloat64f(t, 1.0, q.Mag(), "a value divided by itself is exactly 1")
	})

	t.Run("Mul: a huge length by a tiny one", func(t *testing.T) {
		// 1e310 mm × 1e-300 mm is 1e10 mm² — an unremarkable area.
		p, err := huge.Mul(units.Millimeters(1e-300))
		require.NoError(t, err)
		require.Equal(t, units.Area, p.Kind())
		require.Equal(t, units.SquareMillimeter, p.Unit())
		require.InEpsilon(t, 1e10, p.Mag(), 1e-12)
	})

	t.Run("Equal: a value equals itself", func(t *testing.T) {
		// Equal has no error channel, so an overflowed base magnitude would not
		// even be reported: |Inf − Inf| is NaN, and NaN <= tol is false.
		require.True(t, huge.Equal(huge, 1e-9))
		require.True(t, huge.Equal(units.Meters(1e307), 0), "…at a tolerance of zero")
		// …and across units, where both base magnitudes overflow: 1e307 m is
		// 3.28e307 ft, and the two agree to a tolerance that is minuscule beside
		// the 1e310 mm quantity itself.
		require.True(t, units.Meters(1e307).Equal(units.Feet(1e307*(1000.0/304.8)), 1e300))
		require.False(t, huge.Equal(units.Meters(-1e307), 1e9), "…while its negation is still not equal to it")
	})

	t.Run("Add: the angle/scalar carve-out", func(t *testing.T) {
		// 1e308 turns is 6.28e308 rad: +Inf. Adding a radian to it changes nothing,
		// and must not fail.
		rev := units.New(1e308, turn)
		require.True(t, math.IsInf(rev.Base(), 1), "the premise")

		s, err := rev.Add(units.Scalar(1))
		require.NoError(t, err)
		require.Equal(t, units.Angle, s.Kind(), "angle + scalar is an angle")
		require.Equal(t, turn, s.Unit(), "…carried in the angle's own unit")
		sameFloat64f(t, 1e308, s.Mag(), "a radian is far below the last ulp of 1e308 turns")

		s, err = units.Scalar(1).Add(rev)
		require.NoError(t, err)
		require.Equal(t, units.Angle, s.Kind(), "scalar + angle is an angle whichever side it appeared on")
		require.Equal(t, turn, s.Unit())
		sameFloat64f(t, 1e308, s.Mag(), "…and the same magnitude")

		// The same arm with an angle on both sides was already ratio-first.
		s, err = rev.Add(units.Radians(1))
		require.NoError(t, err)
		sameFloat64f(t, 1e308, s.Mag(), "…as with an angle on both sides")
	})

	t.Run("System.In and System.Display", func(t *testing.T) {
		// Metric presents a length in millimetres. 1e307 m is 1e310 mm, which no
		// float64 holds — so the number must not come back as 1e307, which the
		// caller would read as 1e307 mm: finite, wrong by a factor of 1000, silent.
		require.True(t, math.IsInf(units.Metric().In(huge), 1),
			"an unrepresentable magnitude is an infinity, never a finite number in the wrong unit")

		// The representable ones come back in the system's unit, as always.
		sameFloat64f(t, 1e307, units.SI().In(huge), "SI presents a length in metres")
		require.InEpsilon(t, 2000.0, units.Metric().In(units.Meters(2)), 1e-12)

		// Display cannot carry the quantity in millimetres, so it hands back the
		// value it was given rather than a relabelled magnitude.
		d := units.Metric().Display(huge)
		require.Equal(t, units.Length, d.Kind())
		require.Equal(t, units.Meter, d.Unit())
		sameFloat64f(t, 1e307, d.Mag(), "…with the magnitude it already had")

		require.Equal(t, units.Millimeter, units.Metric().Display(units.Meters(2)).Unit())
	})
}

// The oracle for the sweep below: exact rational arithmetic. A float64 magnitude
// and a float64 factor are both exact rationals, so their product is the true
// quantity a Value denotes — whether or not float64 can hold it.
var (
	relTol     = big.NewRat(1, 1e9)
	subnormalF = new(big.Rat).SetFloat64(1e-310)
)

// nearest returns the correctly rounded float64 rendering of the true result:
// the nearest float64 to it, or an infinity when no float64 is near enough —
// which is to say exactly when the true result overflows the range.
//
// It is what makes the overflow boundary decidable rather than a matter of
// opinion: there is no band around MaxFloat64 in which either answer will do.
// [big.Rat] holds the true result exactly, whatever its size, and Float64 rounds
// it to nearest-even once, the way an operation on the true result would have to.
//
// A zero result is +0 — a true zero, and a true value too small for the smallest
// subnormal alike. That is the contract the operations state: the sign of a zero is
// not a property of a quantity, so a Value never carries a negative one, and the
// oracle says the same. It matters because every assertion below compares against
// this number bit for bit ([sameFloat64f]).
func nearest(want *big.Rat) float64 { return positiveZero(mustFloat64(want)) }

func mustFloat64(want *big.Rat) float64 {
	f, _ := want.Float64()
	return f
}

// positiveZero renders a zero as +0, the way every operation in the package does:
// the sign of a zero is not a property of a quantity, so a Value never carries a
// −0. The oracles state the contract with it rather than IEEE's sign rules.
func positiveZero(x float64) float64 {
	if x == 0 {
		return 0
	}
	return x
}

// sameFloat64f asserts that got is want bit for bit, the sign of a zero included.
//
// An exactness claim is a claim about the float64 itself, and an IEEE == cannot
// make it: +0 == −0 is true, so require.Equal on two float64s is blind to a result
// that came back a negative zero — which is a different float64, prints as "-0",
// and reads as negative under math.Signbit. Every claim in this suite that a result
// is *the* correctly rounded float64, rather than one within a tolerance of it, is
// asserted with this.
func sameFloat64f(t *testing.T, want, got float64, format string, args ...any) {
	t.Helper()

	require.Equal(t, math.Float64bits(want), math.Float64bits(got),
		"%s: want %v (bits %#016x), got %v (bits %#016x)",
		fmt.Sprintf(format, args...), want, math.Float64bits(want), got, math.Float64bits(got))
}

// sameValuef asserts that got is want: the same unit, and the same magnitude bit for
// bit. require.Equal on two Values compares their magnitudes with ==, which — like
// every float comparison — cannot see a negative zero.
func sameValuef(t *testing.T, want, got units.Value, format string, args ...any) {
	t.Helper()

	label := fmt.Sprintf(format, args...)
	require.Equal(t, want.Unit(), got.Unit(), "%s: the unit", label)
	sameFloat64f(t, want.Mag(), got.Mag(), "%s: the magnitude", label)
}

// overflows reports whether the true result is past the last float64 — the last
// float64 itself, MaxFloat64, is not.
func overflows(want *big.Rat) bool { return math.IsInf(nearest(want), 0) }

// baseRat returns v's base magnitude exactly.
func baseRat(t *testing.T, v units.Value) *big.Rat {
	t.Helper()

	m := new(big.Rat).SetFloat64(v.Mag())
	require.NotNil(t, m, "the matrix holds finite magnitudes only")
	f := new(big.Rat).SetFloat64(v.Unit().Factor())
	require.NotNil(t, f, "a unit factor is finite")
	return new(big.Rat).Mul(m, f)
}

// ulpErr returns the distance from got to want, measured in ulps of want: the
// unit in the last place of the float64 nearest the true result. A correctly
// rounded result is at most 0.5 ulp out; anything above 1 ulp is a rounding the
// expression did not have to make.
func ulpErr(t *testing.T, got float64, want *big.Rat) float64 {
	t.Helper()

	w, _ := want.Float64()
	require.False(t, math.IsInf(w, 0), "the true result %s is representable", want.FloatString(3))
	if w == 0 {
		sameFloat64f(t, 0, got, "the true result is zero, so the result is a positive zero")
		return 0
	}
	return ulpsBetween(t, got, want, ulpAt(w))
}

// naiveUlpErr measures the plain expression the helpers replaced. It is the same
// distance ulpErr reports, except that the plain expression is entitled to be
// unusable at the extremes: an intermediate of its own that overflows leaves it
// an infinity or a NaN, which is infinitely far from the true result — and a true
// result that rounds to zero is measured against the last place of the subnormal
// range, since it has no last place of its own.
func naiveUlpErr(t *testing.T, naive float64, want *big.Rat) float64 {
	t.Helper()

	if math.IsInf(naive, 0) || math.IsNaN(naive) {
		return math.Inf(1)
	}
	if w, _ := want.Float64(); w == 0 {
		return ulpsBetween(t, naive, want, math.SmallestNonzeroFloat64)
	}
	return ulpErr(t, naive, want)
}

// ulpAt returns the size of the last place of x: x == mantissa × 2**e with the
// mantissa in [0.5, 1), so the last place of the 53-bit significand is 2**(e-53).
func ulpAt(x float64) float64 {
	_, e := math.Frexp(x)
	ulp := math.Ldexp(1, e-53)
	if ulp == 0 {
		return math.SmallestNonzeroFloat64 // a subnormal
	}
	return ulp
}

// ulpsBetween returns the distance from got to want, measured in units of ulp —
// the last place of whatever scale the caller judges the error at.
func ulpsBetween(t *testing.T, got float64, want *big.Rat, ulp float64) float64 {
	t.Helper()

	diff := new(big.Rat).Abs(new(big.Rat).Sub(ratOf(t, got), want))
	n, _ := new(big.Rat).Quo(diff, ratOf(t, ulp)).Float64()
	return n
}

func ratOf(t *testing.T, x float64) *big.Rat {
	t.Helper()

	r := new(big.Rat).SetFloat64(x)
	require.NotNil(t, r, "%v is finite", x)
	return r
}

// requireClose asserts that got is the float64 rendering of want, to a relative
// tolerance — with an absolute floor, because a true result down among the
// subnormals has no relative precision left to compare against.
//
// A zero it judges on the bits instead: a tolerance cannot see the difference
// between +0 and −0 (their difference is zero), and a Value never carries a −0. So
// every operation this helper judges — the whole extreme matrix — is swept for one.
func requireClose(t *testing.T, got float64, want *big.Rat) {
	t.Helper()

	g := new(big.Rat).SetFloat64(got)
	require.NotNil(t, g, "the true result %s is representable, so the result is finite: got %v",
		want.FloatString(3), got)
	require.False(t, math.Signbit(got) && got == 0,
		"a zero result is a positive zero: the true result is %s, got a negative zero", want.FloatString(3))

	diff := new(big.Rat).Abs(new(big.Rat).Sub(g, want))
	tol := new(big.Rat).Mul(new(big.Rat).Abs(want), relTol)
	tol.Add(tol, subnormalF)
	require.LessOrEqual(t, diff.Cmp(tol), 0, "want %s, got %v", want.FloatString(3), got)
}

// requireResult asserts the contract of an operation that can report an error:
// the true result whenever float64 can hold it, and ErrNotFinite exactly when it
// cannot. The boundary is decided by [nearest], not conceded to the rounding: a
// finite number handed back for a true result that overflows is the whole of the
// bug this guards, and it lives at MaxFloat64 itself.
func requireResult(t *testing.T, got float64, err error, want *big.Rat) {
	t.Helper()

	if overflows(want) {
		require.ErrorIs(t, err, units.ErrNotFinite,
			"the true result %s overflows float64, so it is not a finite %v", want.FloatString(3), got)
		return
	}
	require.NoError(t, err, "the true result %s is representable", want.FloatString(3))
	requireClose(t, got, want)
}

// requireFloat asserts the same contract for an operation with no error to
// report: the true result where float64 holds it, an infinity of the right sign
// where it does not — never a finite number that is not the quantity asked for.
func requireFloat(t *testing.T, got float64, want *big.Rat) {
	t.Helper()

	if overflows(want) {
		require.True(t, math.IsInf(got, want.Sign()),
			"the true result %s overflows float64, so it comes back as an infinity: got %v",
			want.FloatString(3), got)
		return
	}
	requireClose(t, got, want)
}

// extremes is the sweep matrix: every value in it is ordinary — a finite
// magnitude in a real unit, the kind of thing any caller can build — and several
// of them have a base magnitude that overflows float64 on its own.
func extremes() []units.Value {
	return []units.Value{
		{}, // the zero Value: 0 of One
		units.Scalar(1),
		units.Scalar(math.MaxFloat64),
		units.Millimeters(5e-324), // the smallest subnormal
		units.Millimeters(1e-300),
		units.Millimeters(math.MaxFloat64),
		units.Meters(1e307),
		units.Meters(-1e307),
		units.Meters(math.MaxFloat64),
		units.Thous(1e-300),
		units.Inches(-1e305),
		units.Degrees(90),
		units.Radians(1),
		units.New(1e308, turn),
		units.New(-1e308, turn),
		units.Kilograms(1e300),
		units.SquareMeters(1e300),
		units.CubicMillimeters(5e-324),
		units.Grams(1e-322),                  // a base magnitude that underflows to zero…
		units.KilogramsPerCubicMeter(1e-320), // …and one that underflows harder
	}
}

// unitsOfKind lists the units the sweep converts through, per kind.
func unitsOfKind(k units.Kind) []units.Unit {
	switch k {
	case units.Length:
		return []units.Unit{units.Millimeter, units.Centimeter, units.Meter, units.Inch, units.Foot, units.Thou}
	case units.Angle:
		return []units.Unit{units.Radian, units.Degree, turn}
	case units.Mass:
		return []units.Unit{units.Kilogram, units.Gram, units.Pound}
	case units.Area:
		return []units.Unit{units.SquareMillimeter, units.SquareMeter, units.SquareInch}
	case units.Volume:
		return []units.Unit{units.CubicMillimeter, units.CubicCentimeter, units.Liter}
	case units.Density:
		return []units.Unit{units.KilogramPerCubicMillimeter, units.KilogramPerCubicMeter, units.GramPerCubicCentimeter}
	case units.Dimensionless:
		return []units.Unit{units.One}
	}
	return nil
}

// TestExtremeMatrix sweeps every exported operation over the matrix, against an
// exact-rational oracle. It is the class-level guard: an operation that formed a
// base magnitude as an intermediate — a magnitude times its unit's factor, which
// overflows for an ordinary operand such as Meters(1e307) — would fail here,
// whichever operation it was.
func TestExtremeMatrix(t *testing.T) {
	t.Run("In and Convert", func(t *testing.T) {
		for _, v := range extremes() {
			// The identity conversion is exact: a value is expressible in the unit
			// it already carries, whatever its base magnitude.
			m, err := v.In(v.Unit())
			require.NoError(t, err, "%s in its own unit", v)
			sameFloat64f(t, v.Mag(), m, "%s in its own unit is its own magnitude", v)

			for _, u := range unitsOfKind(v.Kind()) {
				want := new(big.Rat).Quo(baseRat(t, v), ratOf(t, u.Factor()))

				m, err := v.In(u)
				requireResult(t, m, err, want)

				c, cerr := v.Convert(u)
				if err != nil {
					require.ErrorIs(t, cerr, units.ErrNotFinite, "Convert fails exactly where In does: %s in %s", v, u)
					require.Equal(t, units.Value{}, c, "no value escapes with the error")
					continue
				}
				require.NoError(t, cerr)
				require.Equal(t, u, c.Unit(), "Convert carries the target unit")
				sameFloat64f(t, m, c.Mag(), "Convert agrees with In: %s in %s", v, u)
			}
		}
	})

	t.Run("Add and Sub", func(t *testing.T) {
		for _, a := range extremes() {
			for _, b := range extremes() {
				if !sameKindOrCarveOut(a.Kind(), b.Kind()) {
					_, err := a.Add(b)
					require.ErrorIs(t, err, units.ErrIncompatible, "%s + %s", a, b)
					_, err = a.Sub(b)
					require.ErrorIs(t, err, units.ErrIncompatible, "%s - %s", a, b)
					continue
				}

				u := combineUnit(a, b)
				for _, o := range addSubOps() {
					want := combineRat(t, a, b, o.sign)

					r, err := o.do(a, b)
					requireResult(t, r.Mag(), err, want)
					if err != nil {
						require.Equal(t, units.Value{}, r, "no value escapes with the error")
						continue
					}
					require.Equal(t, u, r.Unit(), "%s %s %s", a, o.op, b)
					require.Equal(t, u.Kind(), r.Kind())
				}
			}
		}
	})

	t.Run("Mul", func(t *testing.T) {
		for _, a := range extremes() {
			for _, b := range extremes() {
				want := new(big.Rat).Mul(baseRat(t, a), baseRat(t, b))

				p, err := a.Mul(b)
				requireResult(t, p.Mag(), err, want)
				if err != nil {
					require.Equal(t, units.Value{}, p, "no value escapes with the error")
					continue
				}
				require.Equal(t, a.Kind().Mul(b.Kind()), p.Kind(), "%s x %s", a, b)
				require.InDelta(t, 1, p.Unit().Factor(), 0, "a product is carried in a factor-1 unit")
			}
		}
	})

	t.Run("Div", func(t *testing.T) {
		for _, a := range extremes() {
			for _, b := range extremes() {
				q, err := a.Div(b)

				// Only a zero divisor is a division by zero. A divisor that is
				// nonzero in its own unit divides — even where its base magnitude
				// underflows to zero — so the oracle below runs on it.
				if b.Mag() == 0 {
					require.ErrorIs(t, err, units.ErrDivideByZero, "%s / %s", a, b)
					continue
				}

				want := new(big.Rat).Quo(baseRat(t, a), baseRat(t, b))
				requireResult(t, q.Mag(), err, want)
				if err != nil {
					require.Equal(t, units.Value{}, q, "no value escapes with the error")
					continue
				}
				require.Equal(t, a.Kind().Div(b.Kind()), q.Kind(), "%s / %s", a, b)
				require.InDelta(t, 1, q.Unit().Factor(), 0, "a quotient is carried in a factor-1 unit")
			}
		}
	})

	t.Run("Equal is reflexive", func(t *testing.T) {
		for _, v := range extremes() {
			for _, tol := range []float64{0, 1e-9, 1e300, math.MaxFloat64} {
				require.True(t, v.Equal(v, tol), "%s equals itself at tol %v", v, tol)
				require.True(t, units.New(v.Mag(), v.Unit()).Equal(v, tol),
					"…and so does a value rebuilt from its own magnitude and unit")
			}

			// …and it is symmetric, and never equal across kinds.
			for _, o := range extremes() {
				require.Equal(t, v.Equal(o, 1e-9), o.Equal(v, 1e-9), "Equal is symmetric: %s, %s", v, o)
				if v.Kind() != o.Kind() {
					require.False(t, v.Equal(o, math.MaxFloat64), "%s is not a %s", v.Kind(), o.Kind())
				}
			}
		}
	})

	t.Run("Scale and Neg", func(t *testing.T) {
		for _, v := range extremes() {
			sameFloat64f(t, positiveZero(-v.Mag()), v.Neg().Mag(), "%s negates", v)
			require.Equal(t, v.Unit(), v.Neg().Unit(), "…in its own unit")
			require.True(t, v.Neg().Neg().Equal(v, 0), "…and negating twice is the identity")

			sameFloat64f(t, v.Mag(), v.Scale(1).Mag(), "scaling by 1 is the identity: %s", v)
			require.True(t, v.Scale(1).Equal(v, 0))
			require.Equal(t, v.Kind(), v.Scale(2).Kind(), "scaling keeps the kind")
		}
	})

	t.Run("System", func(t *testing.T) {
		for _, sys := range []units.System{units.Metric(), units.SI(), units.Imperial(), {}} {
			for _, v := range extremes() {
				u := sys.UnitFor(v.Kind())
				want := new(big.Rat).Quo(baseRat(t, v), ratOf(t, u.Factor()))

				// In always answers in u — an unrepresentable magnitude as the
				// infinity it is, never as a finite number in some other unit.
				requireFloat(t, sys.In(v), want)

				d := sys.Display(v)
				require.Equal(t, v.Kind(), d.Kind(), "Display preserves the kind: %s", v)
				if _, err := v.In(u); err != nil {
					require.Equal(t, v.Unit(), d.Unit(), "a value it cannot carry in u comes back unchanged")
					sameFloat64f(t, v.Mag(), d.Mag(), "…with its own magnitude: %s", v)
					continue
				}
				require.Equal(t, u, d.Unit(), "Display carries the system's unit: %s", v)
				requireFloat(t, d.Mag(), want)
			}

			// The FromBase wrappers land on the kind they name, and on the
			// magnitude the system's unit gives that base.
			for _, base := range []float64{0, 1, math.Pi, -1e300, 5e-324, 1e307, math.MaxFloat64} {
				l := sys.LengthFromBase(base)
				require.Equal(t, units.Length, l.Kind())
				requireFloat(t, l.Mag(), new(big.Rat).Quo(ratOf(t, base), ratOf(t, sys.UnitFor(units.Length).Factor())))

				a := sys.AngleFromBase(base)
				require.Equal(t, units.Angle, a.Kind())
				requireFloat(t, a.Mag(), new(big.Rat).Quo(ratOf(t, base), ratOf(t, sys.UnitFor(units.Angle).Factor())))
			}
		}
	})
}

func TestConversionIsExact(t *testing.T) {
	// The conversions a caller reads off the screen are exact, not merely close.
	// A relative tolerance of 1e-9 is some 1e7 ulps: it cannot tell an exact 1 in
	// from a 0.9999999999999999, which is why these are bit-for-bit assertions.
	in, err := units.Millimeters(25.4).In(units.Inch)
	require.NoError(t, err)
	sameFloat64f(t, 1.0, in, "25.4 mm is exactly 1 in")

	d, err := units.GramsPerCubicCentimeter(1000).In(units.KilogramPerCubicMeter)
	require.NoError(t, err)
	sameFloat64f(t, 1e6, d, "1000 g/cm^3 is exactly 1e6 kg/m^3")

	t.Run("a value in the unit it already carries", func(t *testing.T) {
		mags := append(everydayMags(),
			0, negativeZero(), // a zero comes back a positive zero, whichever zero went in
			5e-324, 1e-322, 1e-300, 1e307, -1e307, math.MaxFloat64)
		for _, u := range builtinUnits() {
			for _, m := range mags {
				v := units.New(m, u)

				got, err := v.In(u)
				require.NoError(t, err, "a value is always expressible in its own unit: %s", v)
				sameFloat64f(t, v.Mag(), got, "%s in its own unit is its own magnitude", v)

				c, err := v.Convert(u)
				require.NoError(t, err)
				sameValuef(t, v, c, "…and Convert hands back the value itself")
			}
		}
	})

	t.Run("a value divided by itself is exactly 1", func(t *testing.T) {
		mags := append(everydayMags(), 5e-324, 1e-322, 1e-300, 1e307, -1e307, math.MaxFloat64)
		for _, u := range builtinUnits() {
			for _, m := range mags {
				v := units.New(m, u)

				q, err := v.Div(v)
				require.NoError(t, err, "a value divides by itself, however large or small its base magnitude: %s", v)
				require.Equal(t, units.Scalar(1), q, "%s divided by itself is 1", v)
			}
		}
	})
}

// equalTols are the tolerances the symmetry sweep is run at: exact equality, a
// tolerance below the last place of an everyday base magnitude, and tolerances a
// caller would actually pass. They are absolute, in the kind's base unit, and
// fixed here — a tolerance computed from the code under test (a.Base(), say)
// would grow with whatever that code returned and could swallow the very
// disagreement it is supposed to catch.
func equalTols() []float64 { return []float64{0, 1e-12, 1e-9, 1e-6, 1e-3} }

// sameQuantityPairs are pairs that denote one quantity written two ways: the
// specific reproducers of the asymmetry, alongside the exact ones.
func sameQuantityPairs() [][2]units.Value {
	return [][2]units.Value{
		{units.Millimeters(25.4), units.Inches(1)},
		{units.GramsPerCubicCentimeter(1000), units.KilogramsPerCubicMeter(1e6)},
		{units.KilogramsPerCubicMillimeter(1), units.KilogramsPerCubicMeter(1e9)},
		{units.KilogramsPerCubicMillimeter(1), units.GramsPerCubicCentimeter(1e6)},
		{units.Inches(1e-6), units.Thous(0.001)}, // both are exactly 2.54e-5 mm
		{units.Meters(1), units.Millimeters(1000)},
		{units.Feet(1), units.Inches(12)},
		{units.SquareMeters(1), units.SquareCentimeters(1e4)},
		{units.Liters(1), units.CubicCentimeters(1000)},
		{units.Kilograms(1), units.Grams(1000)},
		{units.Degrees(180), units.Radians(math.Pi)},
	}
}

func TestEqualIsSymmetric(t *testing.T) {
	// An equality predicate whose answer depends on the order of its operands is
	// broken whichever answer it gives. The assertion here is symmetry itself —
	// that the two readings agree — not that either says true: at tol 0 a
	// conversion that rounds may leave two renderings of one quantity an ulp
	// apart, and the predicate is entitled to say so, but it must say the same
	// thing whichever operand it is handed first.
	require.True(t, units.Millimeters(25.4).Equal(units.Inches(1), 0), "25.4 mm is one inch")
	require.True(t, units.Inches(1).Equal(units.Millimeters(25.4), 0), "…in either order")

	requireSymmetric := func(t *testing.T, a, b units.Value) {
		t.Helper()

		for _, tol := range equalTols() {
			require.Equal(t, a.Equal(b, tol), b.Equal(a, tol),
				"Equal is symmetric: %s, %s at tol %v", a, b, tol)
		}
	}

	t.Run("one quantity written two ways", func(t *testing.T) {
		// The sharp case, and the one the sweep below never used to construct: the
		// magnitudes differ precisely so that the two values denote the same
		// quantity. An asymmetric predicate rescales the operand it is handed
		// second, so it is exactly here that the two readings can round apart.
		for _, p := range sameQuantityPairs() {
			requireSymmetric(t, p[0], p[1])
		}
	})

	t.Run("every built-in pair of a kind", func(t *testing.T) {
		for _, ua := range builtinUnits() {
			for _, ub := range builtinUnits() {
				for _, m := range everydayMags() {
					// Two magnitudes that are the same number, and so — across units —
					// almost never the same quantity.
					requireSymmetric(t, units.New(m, ua), units.New(m, ub))

					if ua.Kind() != ub.Kind() {
						continue
					}

					// And the same quantity in both units: c is a's own quantity carried
					// in ub. Both readings must agree at every tolerance, tol 0 included.
					a := units.New(m, ua)
					c, err := a.Convert(ub)
					require.NoError(t, err)
					requireSymmetric(t, a, c)

					// A conversion that rounds moves the quantity by an ulp or so of its
					// own magnitude; one base unit is far above that for every everyday
					// value here, and both readings must find them equal there.
					require.True(t, a.Equal(c, 1), "%s is %s to within a base unit", a, c)
					require.True(t, c.Equal(a, 1), "…and %s is %s, whichever side it is read from", c, a)
				}
			}
		}
	})

	t.Run("different kinds are never equal", func(t *testing.T) {
		for _, ua := range builtinUnits() {
			for _, ub := range builtinUnits() {
				if ua.Kind() == ub.Kind() {
					continue
				}
				a, b := units.New(1, ua), units.New(1, ub)
				for _, tol := range append(equalTols(), 1e9) {
					require.False(t, a.Equal(b, tol), "%s is not %s at any tolerance", a, b)
					require.False(t, b.Equal(a, tol), "…nor the other way round")
				}
			}
		}
	})
}

func TestArithmeticIsNoWorseThanNaive(t *testing.T) {
	// The Frexp/Ldexp helpers exist to keep an intermediate in range, not to buy
	// that range with accuracy. Against an exact-rational oracle, In, Mul and Div
	// must land no further from the true result than the plain expression each
	// replaced — the one assertion a relative-tolerance oracle cannot make, since
	// a regression of an ulp is invisible at 1e-9.
	//
	// Add and Sub are held to it too, and to more: they are decided on the true sum,
	// so they are correctly rounded — TestAddSubIsCorrectlyRounded asserts that
	// outright — and no plain expression can beat a correctly rounded result.
	t.Run("In", func(t *testing.T) {
		for _, from := range builtinUnits() {
			for _, to := range builtinUnits() {
				if from.Kind() != to.Kind() {
					continue
				}

				for _, m := range everydayMags() {
					v := units.New(m, from)

					got, err := v.In(to)
					require.NoError(t, err, "%s in %s", v, to)

					want := new(big.Rat).Quo(
						new(big.Rat).Mul(ratOf(t, m), ratOf(t, from.Factor())),
						ratOf(t, to.Factor()))
					naive := m * from.Factor() / to.Factor()

					require.LessOrEqual(t, ulpErr(t, got, want), ulpErr(t, naive, want),
						"%v %s in %s: %v is further from the true result than the plain %v", m, from, to, got, naive)
				}
			}
		}
	})

	t.Run("Mul", func(t *testing.T) {
		for _, ua := range builtinUnits() {
			for _, ub := range builtinUnits() {
				for _, ma := range everydayMags() {
					for _, mb := range everydayMags() {
						a, b := units.New(ma, ua), units.New(mb, ub)

						p, err := a.Mul(b)
						require.NoError(t, err, "%s x %s", a, b)

						want := new(big.Rat).Mul(
							new(big.Rat).Mul(ratOf(t, ma), ratOf(t, ua.Factor())),
							new(big.Rat).Mul(ratOf(t, mb), ratOf(t, ub.Factor())))
						naive := (ma * ua.Factor()) * (mb * ub.Factor())

						require.LessOrEqual(t, ulpErr(t, p.Mag(), want), ulpErr(t, naive, want),
							"%s x %s: %v is further from the true result than the plain %v", a, b, p.Mag(), naive)
					}
				}
			}
		}
	})

	t.Run("Div", func(t *testing.T) {
		for _, ua := range builtinUnits() {
			for _, ub := range builtinUnits() {
				for _, ma := range everydayMags() {
					for _, mb := range everydayMags() {
						a, b := units.New(ma, ua), units.New(mb, ub)

						q, err := a.Div(b)
						require.NoError(t, err, "%s / %s", a, b)

						want := new(big.Rat).Quo(
							new(big.Rat).Mul(ratOf(t, ma), ratOf(t, ua.Factor())),
							new(big.Rat).Mul(ratOf(t, mb), ratOf(t, ub.Factor())))
						naive := (ma * ua.Factor()) / (mb * ub.Factor())

						require.LessOrEqual(t, ulpErr(t, q.Mag(), want), ulpErr(t, naive, want),
							"%s / %s: %v is further from the true result than the plain %v", a, b, q.Mag(), naive)
					}
				}
			}
		}
	})

	t.Run("Add and Sub", func(t *testing.T) {
		for _, ua := range builtinUnits() {
			for _, ub := range builtinUnits() {
				if !sameKindOrCarveOut(ua.Kind(), ub.Kind()) {
					continue
				}

				for _, ma := range everydayMags() {
					for _, mb := range everydayMags() {
						for _, sb := range signs() {
							a, b := units.New(ma, ua), units.New(sb*mb, ub)

							for _, o := range addSubOps() {
								sum, err := o.do(a, b)
								require.NoError(t, err, "%s %s %s", a, o.op, b)

								want := combineRat(t, a, b, o.sign)
								u := combineUnit(a, b)
								naive := (a.Mag()*ua.Factor() + o.sign*b.Mag()*ub.Factor()) / u.Factor()

								require.LessOrEqual(t, ulpErr(t, sum.Mag(), want), naiveUlpErr(t, naive, want),
									"%s %s %s: %v is further from the true result than the plain %v",
									a, o.op, b, sum.Mag(), naive)
							}
						}
					}
				}
			}
		}
	})
}

// hugeLength and tinyLength are application-defined units whose factors sit at
// the ends of the float64 range. They are what turns an everyday magnitude into
// a base magnitude that overflows, or one that lands among the subnormals, so
// the accuracy sweep below can reach both without leaving the ordinary API.
var (
	hugeLength = units.Define("Lhuge", units.Length, 1e300)
	tinyLength = units.Define("Ltiny", units.Length, 1e-300)
)

// extremeMags are the magnitudes accuracy has to survive, not merely the ones a
// caller writes: the smallest subnormal and its neighbours, the boundary between
// the subnormal and normal ranges, and the top of the float64 range.
func extremeMags() []float64 {
	return []float64{
		5e-324, 1e-323, 3e-322, 1e-310, // subnormals
		2.2250738585072014e-308, // the smallest normal
		1e-300, 0.001, 1, 1.25, -2.5, 25.4, math.Pi, 1e300, 1e307, 1e308, math.MaxFloat64,
	}
}

// extremeUnits are the units the accuracy sweep runs over: a representative unit
// of every kind, plus the two application-defined factors of 1e±300.
func extremeUnits() []units.Unit {
	return []units.Unit{
		units.One,
		units.Millimeter, units.Centimeter, units.Meter, units.Inch, units.Thou,
		units.Gram, units.Kilogram,
		units.Degree, units.Radian, turn,
		units.SquareMeter, units.Liter, units.GramPerCubicCentimeter,
		hugeLength, tinyLength,
	}
}

// extremeUlpBound is the accuracy Mul, Div, In and Convert are held to at the
// extremes: the result lands within two ulps of the true one, subnormals
// included. It is a bound on top of the sweep's real assertion — that no result
// is further out than the plain expression it replaced — and it is what pins the
// subnormal range, where the plain expression is often unusable and so is a low
// bar to clear.
//
// Two ulps rather than a half is the honest bound: (a × af) × (b × bf) rounds
// three times whoever computes it — the two base magnitudes and the product — so
// neither the helpers nor the plain expression is correctly rounded in general.
// What the helpers guarantee is that they round where the plain expression
// rounds, and once — never a fourth time on the way back down into the
// subnormals.
const extremeUlpBound = 2

// requireNoWorseThanNaivef asserts the whole accuracy contract of a helper at one
// point: the result is no further from the true value than the plain expression
// (which, at the extremes, is often an infinity or a zero — so the bar is only
// low where the plain expression is unusable), and it is within the ulp bound.
func requireNoWorseThanNaivef(t *testing.T, got, naive float64, want *big.Rat, format string, args ...any) {
	t.Helper()

	label := fmt.Sprintf(format, args...)
	ours := ulpErr(t, got, want)
	require.LessOrEqual(t, ours, naiveUlpErr(t, naive, want),
		"%s: %v is further from the true result %s than the plain %v", label, got, want.FloatString(3), naive)
	require.LessOrEqual(t, ours, float64(extremeUlpBound),
		"%s: %v is more than %d ulp from the true result %s", label, got, extremeUlpBound, want.FloatString(3))
}

func TestArithmeticAtTheExtremesIsNoWorseThanNaive(t *testing.T) {
	// The sweep above judges accuracy where nothing can be excused as the float64
	// range running out. This one judges it exactly where that excuse is available
	// and must not be taken: at the top of the range, and down among the
	// subnormals, where a result that is rounded a second time on its way into the
	// range is worse than the plain expression that rounds once.
	t.Run("the reproducers", func(t *testing.T) {
		// 1.25 ÷ 1e307 cm is 1.25e-308: a subnormal, and exactly what the plain
		// expression gives, because 1.25e307 cm is 1e308 mm — still in range.
		q, err := units.Scalar(1.25).Div(units.Centimeters(1e307))
		require.NoError(t, err)
		sameFloat64f(t, 1.25e-308, q.Mag(), "a subnormal quotient is rounded once, not twice")
		sameFloat64f(t, (1.25*1)/(1e307*10), q.Mag(), "…which is what the plain expression gives")

		// 5e-324 m × -2.5 g is -3 × 2⁻¹⁰⁷⁴: three ulps of the subnormal range, not
		// the two a second rounding would give.
		p, err := units.Meters(5e-324).Mul(units.Grams(-2.5))
		require.NoError(t, err)
		sameFloat64f(t, -1.5e-323, p.Mag(), "a subnormal product is rounded once, not twice")
		sameFloat64f(t, (5e-324*1000)*(-2.5*0.001), p.Mag(), "…which is what the plain expression gives")
	})

	t.Run("Mul", func(t *testing.T) {
		for _, ua := range extremeUnits() {
			for _, ub := range extremeUnits() {
				for _, ma := range extremeMags() {
					for _, mb := range extremeMags() {
						a, b := units.New(ma, ua), units.New(mb, ub)
						want := new(big.Rat).Mul(
							new(big.Rat).Mul(ratOf(t, ma), ratOf(t, ua.Factor())),
							new(big.Rat).Mul(ratOf(t, mb), ratOf(t, ub.Factor())))

						p, err := a.Mul(b)
						if overflows(want) {
							require.ErrorIs(t, err, units.ErrNotFinite,
								"%s x %s: the true product %s overflows float64", a, b, want.FloatString(3))
							continue
						}
						require.NoError(t, err, "the true product %s is representable", want.FloatString(3))

						naive := (ma * ua.Factor()) * (mb * ub.Factor())
						requireNoWorseThanNaivef(t, p.Mag(), naive, want, "%s x %s", a, b)
					}
				}
			}
		}
	})

	t.Run("Div", func(t *testing.T) {
		for _, ua := range extremeUnits() {
			for _, ub := range extremeUnits() {
				for _, ma := range extremeMags() {
					for _, mb := range extremeMags() {
						a, b := units.New(ma, ua), units.New(mb, ub)
						want := new(big.Rat).Quo(
							new(big.Rat).Mul(ratOf(t, ma), ratOf(t, ua.Factor())),
							new(big.Rat).Mul(ratOf(t, mb), ratOf(t, ub.Factor())))

						q, err := a.Div(b)
						if overflows(want) {
							require.ErrorIs(t, err, units.ErrNotFinite,
								"%s / %s: the true quotient %s overflows float64", a, b, want.FloatString(3))
							continue
						}
						require.NoError(t, err, "the true quotient %s is representable", want.FloatString(3))

						naive := (ma * ua.Factor()) / (mb * ub.Factor())
						requireNoWorseThanNaivef(t, q.Mag(), naive, want, "%s / %s", a, b)
					}
				}
			}
		}
	})

	t.Run("In and Convert", func(t *testing.T) {
		for _, from := range extremeUnits() {
			for _, to := range extremeUnits() {
				if from.Kind() != to.Kind() {
					continue
				}

				for _, m := range extremeMags() {
					v := units.New(m, from)
					want := new(big.Rat).Quo(
						new(big.Rat).Mul(ratOf(t, m), ratOf(t, from.Factor())),
						ratOf(t, to.Factor()))

					got, err := v.In(to)
					if overflows(want) {
						require.ErrorIs(t, err, units.ErrNotFinite,
							"%s in %s: the true magnitude %s overflows float64", v, to, want.FloatString(3))
						_, cerr := v.Convert(to)
						require.ErrorIs(t, cerr, units.ErrNotFinite, "Convert fails exactly where In does")
						continue
					}
					require.NoError(t, err, "the true magnitude %s is representable", want.FloatString(3))

					naive := m * from.Factor() / to.Factor()
					requireNoWorseThanNaivef(t, got, naive, want, "%s in %s", v, to)

					c, err := v.Convert(to)
					require.NoError(t, err)
					sameFloat64f(t, got, c.Mag(), "Convert agrees with In: %s in %s", v, to)
				}
			}
		}
	})

	t.Run("Add and Sub", func(t *testing.T) {
		for _, ua := range extremeUnits() {
			for _, ub := range extremeUnits() {
				if !sameKindOrCarveOut(ua.Kind(), ub.Kind()) {
					continue
				}

				for _, ma := range extremeMags() {
					for _, mb := range extremeMags() {
						for _, sb := range signs() {
							a, b := units.New(ma, ua), units.New(sb*mb, ub)

							for _, o := range addSubOps() {
								want := combineRat(t, a, b, o.sign)
								u := combineUnit(a, b)

								sum, err := o.do(a, b)
								if overflows(want) {
									require.ErrorIs(t, err, units.ErrNotFinite,
										"%s %s %s: the true result %s overflows float64", a, o.op, b, want.FloatString(3))
									continue
								}
								require.NoError(t, err, "the true result %s is representable", want.FloatString(3))

								naive := (a.Mag()*ua.Factor() + o.sign*b.Mag()*ub.Factor()) / u.Factor()
								requireNoWorseThanNaivef(t, sum.Mag(), naive, want, "%s %s %s", a, o.op, b)
							}
						}
					}
				}
			}
		}
	})
}

// boundaryMags are the magnitudes the overflow boundary is swept at: the last
// float64 and the two below it, the top of the range, the smallest normal and the
// smallest subnormal, and — the sharp ones — the factors either side of 1 that
// decide, when multiplied into a magnitude already at the top of the range, whether
// the true result stays representable or crosses the last float64. Every one of them
// is swept at both signs.
func boundaryMags() []float64 {
	last := math.MaxFloat64
	return []float64{
		last,
		math.Nextafter(last, 0),
		math.Nextafter(math.Nextafter(last, 0), 0),
		8.98846567431158e307, // 2¹⁰²³, where the last binade begins
		1e308,
		1.0000000000000002, // 1 + 2⁻⁵²: enough to carry MaxFloat64 over the end
		1.0000000000000004, // 1 + 2⁻⁵¹
		0.9999999999999999, // 1 − 2⁻⁵³: enough to bring an infinity back
		1, 2, 0.5,
		1e300, 1e-300, // the Defined factors, met head on
		2.2250738585072014e-308, // the smallest normal
		1e-323, 5e-324,          // and the subnormals: the other end must not regress
	}
}

// boundaryUnits pair an ordinary unit set with Defined factors of 1e±300, which are
// what carry an everyday magnitude to the ends of the range and back.
func boundaryUnits() []units.Unit {
	return []units.Unit{units.One, units.Millimeter, units.Meter, units.Gram, hugeLength, tinyLength}
}

// atTheEnds reports whether the true result lies in the last binade of float64 or
// below the smallest normal: the two regions where a single rounding decides between
// the last float64 and an infinity, or between one subnormal and the next, and so
// where the arithmetic must be correctly rounded rather than merely close.
func atTheEnds(w float64) bool {
	a := math.Abs(w)
	return a >= 8.98846567431158e307 || a < 2*2.2250738585072014e-308
}

// requireExactBoundaryf asserts the whole finiteness contract at one point: the true
// result is past the last float64 and the operation says so, or it is representable
// and the operation hands it back — correctly rounded at the ends of the range, and
// no further out than the plain expression anywhere else. There is no third case,
// and in particular no band around MaxFloat64 in which a finite answer to an
// infinite question will do.
func requireExactBoundaryf(t *testing.T, got, naive float64, err error, want *big.Rat, format string, args ...any) {
	t.Helper()

	label := fmt.Sprintf(format, args...)
	if overflows(want) {
		require.ErrorIs(t, err, units.ErrNotFinite,
			"%s: the true result %s is past the last float64, so it is not the finite %v",
			label, want.FloatString(3), got)
		return
	}
	require.NoError(t, err, "%s: the true result %s is representable", label, want.FloatString(3))

	if w := nearest(want); atTheEnds(w) {
		sameFloat64f(t, w, got, "%s: the true result %s rounds to %v", label, want.FloatString(3), w)
		return
	}
	requireNoWorseThanNaivef(t, got, naive, want, "%s", label)
}

// TestOverflowBoundaryIsDecided sweeps the top of the float64 range, where the
// difference between the last float64 and an infinity is one rounding. An exact
// rational decides each case, so every point in the sweep has one right answer:
// ErrNotFinite when the true result is past the last float64, and the correctly
// rounded value when it is not.
func TestOverflowBoundaryIsDecided(t *testing.T) {
	t.Run("the reproducer", func(t *testing.T) {
		// MaxFloat64 × (1e-300 × 1e300): the two factors multiply to a shade over 1,
		// which is all it takes to carry the last float64 past the end of the range.
		// The product is an infinity, and no finite magnitude may be handed back for it.
		huge := units.Define("boundary_huge_len", units.Length, 1e300)
		v, err := units.Scalar(1e-300).Mul(units.New(math.MaxFloat64, huge))
		require.ErrorIs(t, err, units.ErrNotFinite, "the true product is past the last float64")
		require.Equal(t, units.Value{}, v, "no value escapes with the error")

		// …and the mirror on the other side: a quotient whose true value is the last
		// float64 must come back, not be refused as an overflow because a mantissa
		// rounded up over the boundary on the way.
		q, err := units.Grams(math.Nextafter(math.MaxFloat64, 0)).Div(units.Grams(0.9999999999999999))
		require.NoError(t, err, "the true quotient is representable, so it is not an overflow")
		sameFloat64f(t, math.MaxFloat64, q.Mag(), "…and it is the last float64")
	})

	signs := []float64{1, -1}

	t.Run("Mul", func(t *testing.T) {
		for _, ua := range boundaryUnits() {
			for _, ub := range boundaryUnits() {
				for _, ma := range boundaryMags() {
					for _, mb := range boundaryMags() {
						for _, sa := range signs {
							for _, sb := range signs {
								a := units.New(sa*ma, ua)
								b := units.New(sb*mb, ub)
								want := new(big.Rat).Mul(baseRat(t, a), baseRat(t, b))
								naive := (a.Mag() * ua.Factor()) * (b.Mag() * ub.Factor())

								p, err := a.Mul(b)
								requireExactBoundaryf(t, p.Mag(), naive, err, want, "%s x %s", a, b)
							}
						}
					}
				}
			}
		}
	})

	t.Run("Div", func(t *testing.T) {
		for _, ua := range boundaryUnits() {
			for _, ub := range boundaryUnits() {
				for _, ma := range boundaryMags() {
					for _, mb := range boundaryMags() {
						for _, sa := range signs {
							for _, sb := range signs {
								a := units.New(sa*ma, ua)
								b := units.New(sb*mb, ub)
								want := new(big.Rat).Quo(baseRat(t, a), baseRat(t, b))
								naive := (a.Mag() * ua.Factor()) / (b.Mag() * ub.Factor())

								q, err := a.Div(b)
								requireExactBoundaryf(t, q.Mag(), naive, err, want, "%s / %s", a, b)
							}
						}
					}
				}
			}
		}
	})

	t.Run("In and Convert", func(t *testing.T) {
		for _, from := range boundaryUnits() {
			for _, to := range boundaryUnits() {
				if from.Kind() != to.Kind() {
					continue
				}

				for _, m := range boundaryMags() {
					for _, s := range signs {
						v := units.New(s*m, from)
						want := new(big.Rat).Quo(baseRat(t, v), ratOf(t, to.Factor()))
						naive := v.Mag() * from.Factor() / to.Factor()

						got, err := v.In(to)
						requireExactBoundaryf(t, got, naive, err, want, "%s in %s", v, to)

						c, cerr := v.Convert(to)
						if err != nil {
							require.ErrorIs(t, cerr, units.ErrNotFinite, "Convert fails exactly where In does")
							continue
						}
						require.NoError(t, cerr)
						sameFloat64f(t, got, c.Mag(), "Convert agrees with In")
					}
				}
			}
		}
	})
}

// signs is the pair of signs every operand in the sweeps below is taken at: a
// cancellation, and an overflow, are questions about a sum of two terms of
// opposite sign, and they must be asked from both sides.
func signs() []float64 { return []float64{1, -1} }

// addSubOps is Add and Sub, so a sweep states each assertion once and runs it at
// both operations. sign is what the oracle applies to the right-hand operand.
func addSubOps() []struct {
	op   string
	sign float64
	do   func(units.Value, units.Value) (units.Value, error)
} {
	return []struct {
		op   string
		sign float64
		do   func(units.Value, units.Value) (units.Value, error)
	}{
		{"+", 1, units.Value.Add},
		{"-", -1, units.Value.Sub},
	}
}

// sameKindOrCarveOut reports whether Add and Sub accept the pair: the same kind,
// or the one cross-kind pair they take — an angle and a dimensionless value, in
// either order.
func sameKindOrCarveOut(a, b units.Kind) bool {
	return a == b ||
		(a == units.Angle && b == units.Dimensionless) ||
		(a == units.Dimensionless && b == units.Angle)
}

// combineUnit returns the unit a + b is carried in: a's, except in the
// angle/dimensionless carve-out entered from the dimensionless side, where it is
// b's — so the sum is an angle whichever operand the angle was.
func combineUnit(a, b units.Value) units.Unit {
	if a.Kind() == units.Dimensionless && b.Kind() == units.Angle {
		return b.Unit()
	}
	return a.Unit()
}

// combineRat returns the true value of a + sign×b, in the unit the sum is carried
// in: the oracle for every Add and Sub assertion in the suite.
func combineRat(t *testing.T, a, b units.Value, sign float64) *big.Rat {
	t.Helper()

	rb := new(big.Rat).Mul(baseRat(t, b), ratOf(t, sign))
	return new(big.Rat).Quo(new(big.Rat).Add(baseRat(t, a), rb), ratOf(t, combineUnit(a, b).Factor()))
}

func TestAddSubIsCorrectlyRounded(t *testing.T) {
	// Add and Sub are decided on the true sum, so there is no ulp bound to state
	// here and no scale to state one at: the result is the float64 nearest the
	// exact value of a + b, bit for bit, or ErrNotFinite when no float64 is near
	// enough. A rescale of an operand into the result's unit would round before the
	// addition, and the addition would then round what it was handed — two roundings
	// where the sum authorises one.
	for _, ua := range builtinUnits() {
		for _, ub := range builtinUnits() {
			if ua.Kind() != ub.Kind() {
				continue
			}

			for _, ma := range everydayMags() {
				for _, mb := range everydayMags() {
					for _, sa := range signs() {
						for _, sb := range signs() {
							a, b := units.New(sa*ma, ua), units.New(sb*mb, ub)

							for _, o := range addSubOps() {
								want := combineRat(t, a, b, o.sign)

								sum, err := o.do(a, b)
								require.NoError(t, err, "%s %s %s", a, o.op, b)
								require.Equal(t, ua, sum.Unit(), "the sum is carried in the left operand's unit")
								sameFloat64f(t, nearest(want), sum.Mag(),
									"%s %s %s: the true result is %s", a, o.op, b, want.FloatString(30))
							}
						}
					}
				}
			}
		}
	}
}

// cancellingMags are the magnitudes the cancellation sweep is run at: the ends of
// the range, the subnormals, and everyday numbers. Each is paired with the operand
// that very nearly annihilates it, so the true sum keeps only the bits the two
// operands do not share.
func cancellingMags() []float64 {
	return []float64{
		math.MaxFloat64,
		math.Nextafter(math.MaxFloat64, 0),
		1e308, 1e300, 1e150, 25.4, 1, math.Pi,
		1e-300,
		2.2250738585072014e-308, // the smallest normal
		5e-324,                  // the smallest subnormal
	}
}

// annihilating returns the magnitude in unit u whose quantity most nearly negates
// v's: the float64 nearest to −(v's base magnitude) ÷ u's factor. Adding it to v
// leaves only what the two cannot cancel — which is the whole of the true sum, and
// the whole of what an intermediate rounding would destroy.
//
// It is computed in exact rationals, not in float64: the very magnitudes that make
// this sharp are the ones whose base magnitude float64 cannot hold. The second
// return is false where no float64 negates v in u — where the quantity is past the
// end of u's range — and there is nothing to sweep.
func annihilating(t *testing.T, v units.Value, u units.Unit) (float64, bool) {
	t.Helper()

	m := nearest(new(big.Rat).Quo(new(big.Rat).Neg(baseRat(t, v)), ratOf(t, u.Factor())))
	if math.IsInf(m, 0) || m == 0 {
		return 0, false
	}
	return m, true
}

func TestAddSubKeepsCancellation(t *testing.T) {
	// A sum is not a rescale of each operand followed by an addition. Each rescale
	// can be correctly rounded on its own and the sum still be wrong: the bits the
	// addition would have cancelled against are already gone when it runs. What is
	// asserted here is the true sum — every point decided by an exact rational, at
	// both ends of the range, in the subnormals, and across the cancellation.
	t.Run("the reproducer", func(t *testing.T) {
		// MaxFloat64 in a unit of factor 1e-300, against the magnitude in a unit of
		// factor 1e300 that all but negates it: two operands whose factors are 600
		// decades apart, whose true sum float64 holds without difficulty, and which
		// rescale into two 53-bit numbers that annihilate.
		a := units.New(-math.MaxFloat64, tinyLength)
		b := units.New(1.7976931348623157e-292, hugeLength)

		// In a's unit the sum is an ordinary number near the top of the range…
		sum, err := a.Add(b)
		require.NoError(t, err, "the true sum is representable, so it is not an overflow")
		sameFloat64f(t, 6.531456099116113e+291, sum.Mag(), "and it is not zero")

		// …and in b's — the same quantity, so the same cancellation — a subnormal.
		sum, err = b.Add(a)
		require.NoError(t, err, "the true sum is representable in either operand's unit")
		sameFloat64f(t, 6.53145609911611e-309, sum.Mag(), "a subnormal sum is the true one, rounded once")

		// Sub is the same sum with the right-hand operand negated, and says the same.
		sum, err = a.Sub(b.Neg())
		require.NoError(t, err)
		sameFloat64f(t, 6.531456099116113e+291, sum.Mag(), "Sub is the same sum")

		sum, err = b.Sub(a.Neg())
		require.NoError(t, err)
		sameFloat64f(t, 6.53145609911611e-309, sum.Mag(), "…in either operand's unit")

		// Both signs: negating both operands negates the sum, and nothing else.
		sum, err = a.Neg().Add(b.Neg())
		require.NoError(t, err)
		sameFloat64f(t, -6.531456099116113e+291, sum.Mag(), "negating both operands negates the sum")

		sum, err = b.Neg().Sub(a)
		require.NoError(t, err)
		sameFloat64f(t, -6.53145609911611e-309, sum.Mag(), "…and nothing else")

		// And where the two do not cancel but reinforce, the true sum is past the
		// last float64 in a's unit — which is an overflow, and is reported as one.
		_, err = a.Sub(b)
		require.ErrorIs(t, err, units.ErrNotFinite, "the true difference is past the last float64")
	})

	t.Run("the sweep", func(t *testing.T) {
		// Every same-kind pair of units — the Defined factors of 1e±300 included, so
		// the two operands' factors can be 600 decades apart — against the operand
		// that most nearly annihilates the first. Then the neighbours of that
		// operand, which cancel almost as much and leave a different handful of bits.
		for _, ua := range extremeUnits() {
			for _, ub := range extremeUnits() {
				if !sameKindOrCarveOut(ua.Kind(), ub.Kind()) {
					continue
				}

				for _, ma := range cancellingMags() {
					for _, sa := range signs() {
						a := units.New(sa*ma, ua)

						mb, ok := annihilating(t, a, ub)
						if !ok {
							continue
						}

						for _, mb := range []float64{
							mb,
							math.Nextafter(mb, math.Inf(1)),
							math.Nextafter(mb, math.Inf(-1)),
							mb * 0.5, // a half of it: half the significand survives
						} {
							if math.IsInf(mb, 0) {
								continue // the neighbour of the last float64 is not one
							}

							for _, o := range addSubOps() {
								// b is the annihilating operand for the operation at hand: Sub
								// negates its right-hand operand, so it is negated here too.
								b := units.New(mb/o.sign, ub)
								want := combineRat(t, a, b, o.sign)

								sum, err := o.do(a, b)
								if overflows(want) {
									require.ErrorIs(t, err, units.ErrNotFinite,
										"%s %s %s: the true result %s is past the last float64",
										a, o.op, b, want.FloatString(3))
									require.Equal(t, units.Value{}, sum, "no value escapes with the error")
									continue
								}
								require.NoError(t, err, "%s %s %s: the true result %s is representable",
									a, o.op, b, want.FloatString(3))
								require.Equal(t, combineUnit(a, b), sum.Unit())
								sameFloat64f(t, nearest(want), sum.Mag(),
									"%s %s %s: the true result is %s", a, o.op, b, want.FloatString(30))
							}
						}
					}
				}
			}
		}
	})
}

// equalOracle decides a.Equal(b, tol) from the definition, in exact rationals:
// the two are equal exactly when the true difference of their base magnitudes is
// within tol. It is computed from the operands' own magnitudes and factors, never
// from an expression in the code under test, so it sees a difference that no
// float64 arithmetic on the way to the answer could have kept — one of 1e-300 mm,
// say, which a rescale into a unit of factor 1e300 underflows to zero.
func equalOracle(t *testing.T, a, b units.Value, tol float64) bool {
	t.Helper()

	if a.Kind() != b.Kind() {
		return false
	}
	d := new(big.Rat).Sub(baseRat(t, a), baseRat(t, b))
	return d.Abs(d).Cmp(ratOf(t, tol)) <= 0
}

// equalSweepUnits are the units of a kind the difference sweep runs over: the
// built-ins, plus — for a length — the application-defined factors of 1e±300, so
// the two operands' factors can be 600 decades apart and a rescale between them
// has the whole range to lose a difference in.
func equalSweepUnits(k units.Kind) []units.Unit {
	u := unitsOfKind(k)
	if k == units.Length {
		u = append(u, hugeLength, tinyLength)
	}
	return u
}

// equalSweepMags are the magnitudes the difference sweep runs over: zero — handed in
// at either sign, both of which build a Value of +0 — the subnormals, the everyday
// numbers, and the top of the range.
func equalSweepMags() []float64 {
	return []float64{
		0, negativeZero(),
		5e-324, 1e-320, 1e-300,
		1, -2.5, 25.4, math.Pi, 1000,
		1e300, 1e307, math.MaxFloat64,
	}
}

// equalSweepTols are the tolerances it is judged at: exact equality, tolerances
// down among the subnormals — where a difference a rescale erases still lives —
// and the ones a caller would actually pass. They are absolute, in the kind's base
// unit, and fixed here: a tolerance derived from the code under test could swallow
// the very disagreement it is meant to catch.
func equalSweepTols() []float64 {
	return []float64{0, 5e-324, 1e-320, 1e-300, 1e-12, 1e-9, 1, 1e300}
}

// equalSweepKinds are the kinds swept: every kind with more than one unit, so
// there is a pair of factors to lose a difference between.
func equalSweepKinds() []units.Kind {
	return []units.Kind{
		units.Length, units.Angle, units.Mass, units.Area, units.Volume, units.Density, units.Dimensionless,
	}
}

// nearestIn returns the magnitude in u that comes nearest v's quantity: v's true
// base magnitude divided by u's factor, rounded once. It is where a difference is
// finest — v and New(nearestIn(v, u), u) are one rounding apart, so their true
// difference is nonzero and minuscule, exactly the difference a rescale into the
// coarser of the two units underflows to nothing — and its mirror, a true
// difference of zero that a rescale would inflate, is the same construction
// wherever that rounding happens to be exact.
//
// It is computed in exact rationals, not by the conversion under test.
func nearestIn(t *testing.T, v units.Value, u units.Unit) (float64, bool) {
	t.Helper()

	m := nearest(new(big.Rat).Quo(baseRat(t, v), ratOf(t, u.Factor())))
	if math.IsInf(m, 0) {
		return 0, false // v has no rendering in u; there is no pair to judge
	}
	return m, true
}

func TestEqualDecidesOnTheTrueDifference(t *testing.T) {
	// Equal answers on the true difference of the two quantities, never on a
	// rounding of it. Rescaling both operands into a common unit and subtracting
	// there is a composition, and the rounding in between is one the comparison never
	// authorised: it can erase a difference entirely, and then report two quantities
	// that genuinely differ as equal at a tolerance of zero.

	t.Run("a difference a rescale would erase", func(t *testing.T) {
		// 1e-300 mm against zero of a unit whose factor is 1e300. The true difference
		// is 1e-300 mm — an ordinary number, which float64 holds without difficulty —
		// but 1e-300 mm rescaled into that unit underflows to 0, and a predicate that
		// subtracts after the rescale has nothing left to compare.
		tiny := units.Millimeters(1e-300)
		for _, z := range []units.Value{
			units.New(negativeZero(), hugeLength), // a −0 handed in, which New canonicalises to +0…
			units.New(0, hugeLength),              // …and an ordinary zero
		} {
			require.False(t, tiny.Equal(z, 0), "%s is not %s: they differ by 1e-300 mm", tiny, z)
			require.False(t, z.Equal(tiny, 0), "…nor the other way round")
			require.True(t, tiny.Equal(z, 1e-300), "…and they are equal at a tolerance that admits the difference")
			require.True(t, z.Equal(tiny, 1e-300), "…in either order")
		}

		// The same erasure with ordinary magnitudes on both sides and no zero in
		// sight: 1e-300 of the 1e300 unit is all but exactly 1 mm — the two factors
		// are rounded float64s, so the quantities differ — and 1 mm rescaled into the
		// 1e300 unit rounds to precisely that magnitude, leaving a difference of zero.
		a, b := units.Millimeters(1), units.New(1e-300, hugeLength)
		require.NotEqual(t, 0, new(big.Rat).Sub(baseRat(t, a), baseRat(t, b)).Sign(),
			"the premise: the two quantities genuinely differ")
		require.False(t, a.Equal(b, 0), "%s is not exactly %s", a, b)
		require.False(t, b.Equal(a, 0), "…nor the other way round")
		require.True(t, a.Equal(b, 1e-9), "…while a tolerance a caller would pass finds them equal")
		require.True(t, b.Equal(a, 1e-9), "…in either order")

		// And its mirror at the bottom of the range: the smallest subnormal in the
		// 1e-300 unit is 5e-624 mm, a quantity no float64 holds. It is not zero, and
		// it is not the same quantity as the smallest subnormal millimetre.
		sub := units.New(5e-324, tinyLength)
		require.False(t, sub.Equal(units.Millimeters(0), 0), "%s is not zero", sub)
		require.False(t, units.Millimeters(0).Equal(sub, 0), "…nor the other way round")
		require.False(t, sub.Equal(units.Millimeters(5e-324), 0), "…and it is not 5e-324 mm either")
		require.True(t, sub.Equal(units.Millimeters(0), 5e-324), "…though a tolerance of one subnormal admits it")
	})

	t.Run("against the exact difference", func(t *testing.T) {
		// The sweep: every same-kind pair of units, the Defined factors of 1e±300
		// included, with the second operand chosen so that the two quantities are as
		// near as the units allow — one rounding apart, or exactly the same quantity.
		// That is where a rescale destroys the difference, and where the answer at a
		// tolerance of zero is decided. Every point is judged against the exact
		// rational difference.
		leaks := 0
		check := func(a, b units.Value, tol float64) {
			want := equalOracle(t, a, b, tol)
			got, swapped := a.Equal(b, tol), b.Equal(a, tol)
			if got == want && swapped == want {
				return
			}
			leaks++
			if leaks <= 10 {
				t.Errorf("Equal(%s, %s, %v) = %v, %v; the true difference says %v",
					a, b, tol, got, swapped, want)
			}
		}

		for _, k := range equalSweepKinds() {
			for _, ua := range equalSweepUnits(k) {
				for _, ub := range equalSweepUnits(k) {
					for _, ma := range equalSweepMags() {
						a := units.New(ma, ua)

						mbs := []float64{ma, 0, negativeZero()}
						if mb, ok := nearestIn(t, a, ub); ok {
							mbs = append(mbs,
								mb,
								math.Nextafter(mb, math.Inf(1)),
								math.Nextafter(mb, math.Inf(-1)),
								mb*0.5,
							)
						}

						for _, mb := range mbs {
							if math.IsInf(mb, 0) {
								continue // the neighbour of the last float64 is not one
							}
							b := units.New(mb, ub)
							for _, tol := range equalSweepTols() {
								check(a, b, tol)
							}
						}
					}
				}
			}
		}
		require.Zero(t, leaks, "%d pairs where Equal disagreed with the true difference", leaks)
	})

	t.Run("symmetry", func(t *testing.T) {
		// Every pair, every magnitude, every tolerance — zero included. An equality
		// predicate whose answer depends on which operand is the receiver is broken
		// whichever answer it gives.
		for _, k := range equalSweepKinds() {
			for _, ua := range equalSweepUnits(k) {
				for _, ub := range equalSweepUnits(k) {
					for _, ma := range equalSweepMags() {
						for _, mb := range equalSweepMags() {
							a, b := units.New(ma, ua), units.New(mb, ub)
							for _, tol := range equalSweepTols() {
								require.Equal(t, a.Equal(b, tol), b.Equal(a, tol),
									"Equal is symmetric: %s, %s at tol %v", a, b, tol)
							}
						}
					}
				}
			}
		}
	})

	t.Run("reflexivity", func(t *testing.T) {
		// A value equals itself at every tolerance, whatever its magnitude — including
		// Meters(1e307), whose base magnitude, 1e310 mm, is +Inf.
		vs := append(extremes(), units.Meters(1e307), units.New(math.MaxFloat64, hugeLength),
			units.New(5e-324, tinyLength), units.Grams(1e-322))
		for _, k := range equalSweepKinds() {
			for _, u := range equalSweepUnits(k) {
				for _, m := range equalSweepMags() {
					vs = append(vs, units.New(m, u))
				}
			}
		}

		for _, v := range vs {
			for _, tol := range []float64{0, 5e-324, 1e-9, 1e300, math.MaxFloat64} {
				require.True(t, v.Equal(v, tol), "%s equals itself at tol %v", v, tol)
				require.True(t, units.New(v.Mag(), v.Unit()).Equal(v, tol),
					"…and so does a value rebuilt from its own magnitude and unit")
			}
		}
	})

	t.Run("a tolerance of zero across units", func(t *testing.T) {
		// A unit's factor is itself a rounded float64, so two values written in
		// different units generally do not denote the same real number. Degree's factor
		// is a rounded pi/180: 180 of them and math.Pi radians are different quantities,
		// and at a tolerance of zero Equal says so. It is what a caller asked for — the
		// same real number — and the tolerance is what a cross-unit comparison is for.
		require.False(t, units.Degrees(180).Equal(units.Radians(math.Pi), 0),
			"180 deg and pi rad are not the same real number")
		require.False(t, units.Radians(math.Pi).Equal(units.Degrees(180), 0), "…in either order")
		require.True(t, units.Degrees(180).Equal(units.Radians(math.Pi), 1e-15),
			"…and a tolerance in base units is what compares them")

		// A gram's factor is a rounded 0.001, so 1000 of them are not exactly the
		// kilogram either — the same fact, in a unit nobody thinks of as inexact.
		require.False(t, units.Kilograms(1).Equal(units.Grams(1000), 0),
			"1000 g is not exactly 1 kg: the gram's factor is a rounded 0.001")
		require.False(t, units.Grams(1000).Equal(units.Kilograms(1), 0), "…in either order")
		require.True(t, units.Kilograms(1).Equal(units.Grams(1000), 1e-15),
			"…and a tolerance in base units is what compares them")

		// Where the two renderings do coincide exactly — a factor float64 holds, such
		// as the 1000 in a metre or the 25.4 an inch is defined as — a tolerance of
		// zero finds them equal, whatever the two factors are.
		for _, p := range [][2]units.Value{
			{units.Millimeters(25.4), units.Inches(1)},
			{units.Meters(1), units.Millimeters(1000)},
			{units.Liters(1), units.CubicCentimeters(1000)},
			{units.Millimeters(1e300), units.New(1, hugeLength)},
			{units.Millimeters(1e-300), units.New(1, tinyLength)},
		} {
			require.True(t, p[0].Equal(p[1], 0), "%s is exactly %s", p[0], p[1])
			require.True(t, p[1].Equal(p[0], 0), "…in either order")
		}
	})
}

func TestEqualNonFiniteMagnitudes(t *testing.T) {
	// New, FromBase, Scale and Neg have no error to report, so a Value can be built
	// whose magnitude is an infinity or a NaN. It is not a quantity: it has no true
	// difference from one, so no tolerance — not even an infinite one — may admit it
	// beside one. The one pair that is equal is a value and the same signed infinity,
	// which keeps Equal reflexive for a value built from an infinity on purpose.

	t.Run("the reproducer", func(t *testing.T) {
		// An infinite length is not one millimetre, at any tolerance a caller can pass.
		inf, one := units.New(math.Inf(1), units.Millimeter), units.Millimeters(1)
		require.False(t, inf.Equal(one, math.Inf(1)), "an infinite length is not 1 mm")
		require.False(t, one.Equal(inf, math.Inf(1)), "…nor the other way round")
		for _, tol := range nonFiniteEqualTols() {
			require.False(t, inf.Equal(one, tol), "…at tol %v either", tol)
			require.False(t, one.Equal(inf, tol), "…in either order")
		}

		// +Inf is not −Inf, and a NaN is nothing at all — not even itself.
		neg := units.New(math.Inf(-1), units.Millimeter)
		nan := units.New(math.NaN(), units.Millimeter)
		require.False(t, inf.Equal(neg, math.Inf(1)), "+Inf mm is not −Inf mm")
		require.False(t, nan.Equal(nan, math.Inf(1)), "a NaN magnitude equals nothing, itself included")

		// …while the same signed infinity is the same non-quantity, so Equal stays
		// reflexive for a value someone constructed with an infinity.
		require.True(t, inf.Equal(inf, 0), "+Inf mm equals itself")
		require.True(t, inf.Equal(units.New(math.Inf(1), units.Millimeter), 1e-9), "…however it was built")
		require.True(t, neg.Equal(neg, math.Inf(1)), "and so does −Inf mm")
	})

	t.Run("a different kind is never equal", func(t *testing.T) {
		for _, m := range nonFiniteMags() {
			a, b := units.New(m, units.Millimeter), units.New(m, units.Radian)
			for _, tol := range nonFiniteEqualTols() {
				require.False(t, a.Equal(b, tol), "%s is not %s at tol %v", a, b, tol)
				require.False(t, b.Equal(a, tol), "…nor the other way round")
			}
		}
	})

	t.Run("against the specified truth table", func(t *testing.T) {
		// The whole matrix: every magnitude — the infinities, the NaN and the finite
		// ones — in every pair of length units, at every tolerance, in both operand
		// orders. The expectation comes from the rules, computed from the operands'
		// own magnitudes and factors, never from an expression in the code under test.
		leaks := 0
		mags := append(nonFiniteMags(), equalSweepMags()...)
		us := equalSweepUnits(units.Length)

		for _, ua := range us {
			for _, ub := range us {
				for _, ma := range mags {
					for _, mb := range mags {
						a, b := units.New(ma, ua), units.New(mb, ub)
						for _, tol := range nonFiniteEqualTols() {
							want := nonFiniteEqualOracle(t, a, b, tol)
							got, swapped := a.Equal(b, tol), b.Equal(a, tol)
							if got == want && swapped == want {
								continue
							}
							leaks++
							if leaks <= 10 {
								t.Errorf("Equal(%s, %s, %v) = %v, %v; the rules say %v",
									a, b, tol, got, swapped, want)
							}
						}
					}
				}
			}
		}
		require.Zero(t, leaks, "%d pairs where Equal disagreed with the rules", leaks)
	})

	t.Run("symmetry and reflexivity across the matrix", func(t *testing.T) {
		mags := append(nonFiniteMags(), equalSweepMags()...)
		us := equalSweepUnits(units.Length)

		for _, ua := range us {
			for _, ub := range us {
				for _, ma := range mags {
					a := units.New(ma, ua)
					for _, tol := range nonFiniteEqualTols() {
						// Reflexivity: every finite value equals itself at every tol >= 0,
						// and so does a value carrying a signed infinity. A NaN equals
						// nothing.
						if tol >= 0 {
							switch {
							case math.IsNaN(ma):
								require.False(t, a.Equal(a, tol), "%s equals nothing, itself included", a)
							default:
								require.True(t, a.Equal(a, tol), "%s equals itself at tol %v", a, tol)
							}
						}

						for _, mb := range mags {
							b := units.New(mb, ub)
							require.Equal(t, a.Equal(b, tol), b.Equal(a, tol),
								"Equal is symmetric: %s, %s at tol %v", a, b, tol)
						}
					}
				}
			}
		}
	})
}

// nonFiniteMags are the magnitudes that are not quantities: a Value can carry one,
// because New and FromBase have no error to report, and Equal must never find one
// within a tolerance of an ordinary quantity.
func nonFiniteMags() []float64 {
	return []float64{math.Inf(1), math.Inf(-1), math.NaN()}
}

// nonFiniteEqualTols are the tolerances the non-finite matrix is judged at: the
// ones a caller passes, and the ones that are not bounds on a real difference at
// all — a negative tolerance, a NaN, and the infinities.
func nonFiniteEqualTols() []float64 {
	return []float64{
		math.Inf(-1), math.NaN(), -1, 0, 5e-324, 1e-9, 1, 1e300, math.MaxFloat64, math.Inf(1),
	}
}

// nonFiniteEqualOracle is the specified answer, read off the rules rather than off
// the code under test: a NaN magnitude is equal to nothing, a lone infinity is
// equal to nothing, two infinities are equal exactly when they carry the same sign
// (a unit's factor is positive, so the magnitude's sign is the quantity's), a
// tolerance that is negative or a NaN admits nothing, an infinite tolerance admits
// every pair of finite quantities of the kind — and two finite magnitudes are
// decided on the true difference, in exact rationals.
func nonFiniteEqualOracle(t *testing.T, a, b units.Value, tol float64) bool {
	t.Helper()

	if a.Kind() != b.Kind() {
		return false
	}

	ma, mb := a.Mag(), b.Mag()
	if math.IsNaN(ma) || math.IsNaN(mb) {
		return false
	}
	if math.IsInf(ma, 0) || math.IsInf(mb, 0) {
		if math.IsInf(ma, 0) && math.IsInf(mb, 0) {
			return math.Signbit(ma) == math.Signbit(mb) && !math.IsNaN(tol) && tol >= 0
		}
		return false
	}

	if math.IsNaN(tol) || tol < 0 {
		return false
	}
	if math.IsInf(tol, 1) {
		return true
	}
	return equalOracle(t, a, b, tol)
}

// negativeZero is the float64 whose sign bit is set and whose value is zero: the
// one number a Value may not carry, and the one an IEEE == cannot tell from +0.
func negativeZero() float64 { return math.Copysign(0, -1) }

// requirePositiveZerof asserts that got is a zero, and the positive one: a Value
// never carries a −0, so every zero the package hands back has the bits of +0.
func requirePositiveZerof(t *testing.T, got float64, format string, args ...any) {
	t.Helper()

	sameFloat64f(t, 0, got, format, args...)
}

// zeroPathUnits are pairs of same-kind units chosen to force each of [sum]'s two
// paths. The fast path is taken where the two operands are already carried in the
// result's unit — the same factor on both sides, where the addition is a plain IEEE
// one and (−0) + (−0) is −0 — and the exact-rational path wherever a factor differs,
// where a rational carries no signed zero at all. Which path runs is an
// implementation detail, and the two must not disagree about the result.
func zeroPathUnits() []struct {
	name     string
	fast     bool
	ua, ub   units.Unit
	ma, mb   float64 // operands that cancel to zero: ma of ua is exactly mb of ub
	nonzeros bool    // whether those operands are nonzero
} {
	return []struct {
		name     string
		fast     bool
		ua, ub   units.Unit
		ma, mb   float64
		nonzeros bool
	}{
		{"the same unit: the fast path", true, units.Millimeter, units.Millimeter, 2.5, 2.5, true},
		{"the same factor: the fast path", true, units.One, units.One, 1, 1, true},
		{"different factors: the exact path", false, units.Millimeter, units.Centimeter, 10, 1, true},
		{"factors 600 decades apart: the exact path", false, tinyLength, hugeLength, 1e300, 1e-300, true},
		{"an angle in two units: the exact path", false, units.Degree, units.Radian, 0, 0, false},
	}
}

func TestZeroIsAlwaysPositive(t *testing.T) {
	// A zero result is +0, always. The sign of a zero is not a property of a
	// quantity — there is no −0 mm — it is a record of how a float64 expression
	// arrived at zero: which term underflowed, which operand was negated, and which
	// of two internal paths the arithmetic took. Add and Sub are correctly rounded
	// from the true sum, and the true sum has one value; a result whose sign bit
	// depends on whether the operands happened to share a unit is a path made
	// observable, and no caller can predict it. So it is canonicalised, in every
	// operation and on construction, and asserted here on the bits: +0 == −0 is true
	// in IEEE, so no comparison by value can see this.

	t.Run("the reproducer", func(t *testing.T) {
		// The same subtraction, once with both operands in the unit the result is
		// carried in — where sum adds them as float64s — and once across units, where
		// it redoes the arithmetic in exact rationals. Both are zero, and both zeros
		// are the same float64.
		fast, err := units.Scalar(negativeZero()).Sub(units.Scalar(0))
		require.NoError(t, err)
		requirePositiveZerof(t, fast.Mag(), "(−0) − 0 on the fast path")

		exact, err := units.Millimeters(negativeZero()).Sub(units.Centimeters(0))
		require.NoError(t, err)
		requirePositiveZerof(t, exact.Mag(), "(−0) − 0 on the exact path")

		sameFloat64f(t, fast.Mag(), exact.Mag(), "the two paths agree on the sign of a zero")

		// (−0) + (−0) is −0 in IEEE. It is not a quantity of −0 mm.
		s, err := units.Scalar(negativeZero()).Add(units.Scalar(negativeZero()))
		require.NoError(t, err)
		requirePositiveZerof(t, s.Mag(), "(−0) + (−0)")

		// And the negation of a zero, which would otherwise print as "-0" and read as
		// negative under math.Signbit.
		n := units.Scalar(0).Neg()
		requirePositiveZerof(t, n.Mag(), "−(0)")
		require.False(t, math.Signbit(n.Mag()), "a zero quantity is not negative")
		require.Equal(t, "0", n.String(), "…and it does not print as one")
		require.Equal(t, "0 mm", units.Millimeters(0).Neg().String())
	})

	t.Run("construction", func(t *testing.T) {
		// A −0 handed to a constructor is a +0 in the Value: no Value anywhere carries
		// a negative zero, so Mag never returns one and no operation has to defend
		// against one arriving from the outside.
		for _, u := range builtinUnits() {
			requirePositiveZerof(t, units.New(negativeZero(), u).Mag(), "New(−0, %s)", u)
			requirePositiveZerof(t, units.FromBase(negativeZero(), u).Mag(), "FromBase(−0, %s)", u)
			requirePositiveZerof(t, units.New(negativeZero(), u).Base(), "New(−0, %s).Base()", u)
		}
		for name, v := range map[string]units.Value{
			"Millimeters": units.Millimeters(negativeZero()),
			"Meters":      units.Meters(negativeZero()),
			"Inches":      units.Inches(negativeZero()),
			"Degrees":     units.Degrees(negativeZero()),
			"Radians":     units.Radians(negativeZero()),
			"Scalar":      units.Scalar(negativeZero()),
			"Kilograms":   units.Kilograms(negativeZero()),
			"Liters":      units.Liters(negativeZero()),
		} {
			requirePositiveZerof(t, v.Mag(), "%s(−0)", name)
		}

		// FromBase divides by the unit's factor, so a negative quantity too small for
		// the unit underflows there — to a zero, which is a positive one.
		requirePositiveZerof(t, units.FromBase(-5e-324, hugeLength).Mag(),
			"a negative quantity that underflows into a coarse unit")
	})

	t.Run("Add and Sub, on both paths", func(t *testing.T) {
		// Every way a sum can be zero — two zeros, either of them negative, and two
		// nonzero operands that annihilate — at every unit pair, so the fast path and
		// the exact path are both taken with each.
		for _, p := range zeroPathUnits() {
			t.Run(p.name, func(t *testing.T) {
				require.Equal(t, p.fast, p.ua.Factor() == p.ub.Factor(),
					"the premise: the operands share the result's unit exactly when the fast path runs")

				zeros := []float64{0, negativeZero()}
				for _, ma := range zeros {
					for _, mb := range zeros {
						a, b := units.New(ma, p.ua), units.New(mb, p.ub)

						s, err := a.Add(b)
						require.NoError(t, err)
						requirePositiveZerof(t, s.Mag(), "%s + %s", a, b)

						d, err := a.Sub(b)
						require.NoError(t, err)
						requirePositiveZerof(t, d.Mag(), "%s − %s", a, b)
					}
				}

				if !p.nonzeros {
					return
				}

				// The cancellation: two nonzero quantities that are exactly each other,
				// so the true difference is zero — the zero the exact path computes as a
				// rational and the fast path as an IEEE subtraction.
				a, b := units.New(p.ma, p.ua), units.New(p.mb, p.ub)
				d, err := a.Sub(b)
				require.NoError(t, err, "%s − %s", a, b)
				requirePositiveZerof(t, d.Mag(), "%s − %s cancels", a, b)

				s, err := a.Add(b.Neg())
				require.NoError(t, err)
				requirePositiveZerof(t, s.Mag(), "%s + (−%s) cancels", a, b)
			})
		}
	})

	t.Run("the two paths agree", func(t *testing.T) {
		// The same quantity, written in a unit that forces the fast path and in one
		// that forces the exact path. Path selection is not something a caller can see:
		// the two results are the same float64, bit for bit.
		for _, ma := range []float64{0, negativeZero()} {
			for _, mb := range []float64{0, negativeZero()} {
				for _, o := range addSubOps() {
					fast, err := o.do(units.Millimeters(ma), units.Millimeters(mb))
					require.NoError(t, err)

					exact, err := o.do(units.Millimeters(ma), units.Centimeters(mb))
					require.NoError(t, err)

					sameFloat64f(t, fast.Mag(), exact.Mag(),
						"%v %s %v: the fast path and the exact path give the same zero", ma, o.op, mb)
				}
			}
		}
	})

	t.Run("the angle/dimensionless carve-out", func(t *testing.T) {
		// The one arm that crosses kinds, entered from either side: the sum is an angle
		// whichever operand the angle was, and its zero is a positive zero.
		for _, ang := range []units.Unit{units.Degree, units.Radian} {
			for _, ma := range []float64{0, negativeZero()} {
				for _, mb := range []float64{0, negativeZero()} {
					for _, o := range addSubOps() {
						s, err := o.do(units.New(ma, ang), units.Scalar(mb))
						require.NoError(t, err)
						require.Equal(t, units.Angle, s.Kind())
						requirePositiveZerof(t, s.Mag(), "%v %s %s: angle %s scalar", ma, o.op, ang, o.op)

						s, err = o.do(units.Scalar(ma), units.New(mb, ang))
						require.NoError(t, err)
						require.Equal(t, units.Angle, s.Kind(), "the sum is an angle whichever side it appeared on")
						requirePositiveZerof(t, s.Mag(), "%v %s %s: scalar %s angle", ma, o.op, ang, o.op)
					}
				}
			}
		}
	})

	t.Run("Neg and Scale", func(t *testing.T) {
		// Neg must not manufacture a −0 that then leaks into a comparison or a String.
		for _, u := range builtinUnits() {
			z := units.New(0, u)
			requirePositiveZerof(t, z.Neg().Mag(), "−(0 %s)", u)
			requirePositiveZerof(t, z.Neg().Neg().Mag(), "0 %s negated twice", u)
			requirePositiveZerof(t, z.Scale(-1).Mag(), "0 %s scaled by −1", u)
			requirePositiveZerof(t, z.Scale(0).Mag(), "0 %s scaled by 0", u)
			requirePositiveZerof(t, units.New(2, u).Scale(negativeZero()).Mag(), "2 %s scaled by −0", u)
			requirePositiveZerof(t, units.New(-2, u).Scale(0).Mag(), "−2 %s scaled by 0", u)
			require.NotContains(t, z.Neg().String(), "-0", "a zero does not print as a negative one")

			// A product that underflows to nothing is a zero like any other.
			requirePositiveZerof(t, units.New(-5e-324, u).Scale(5e-324).Mag(), "a product that underflows")
		}
	})

	t.Run("Mul, Div, In and Convert", func(t *testing.T) {
		// Every operation that can produce a zero produces the same one. Mul and Div
		// carry their result in a base unit, In and Convert in the unit asked for.
		zero, minus := units.Millimeters(0), units.Millimeters(-3)

		p, err := zero.Mul(minus)
		require.NoError(t, err)
		requirePositiveZerof(t, p.Mag(), "0 mm × −3 mm")

		p, err = minus.Mul(zero)
		require.NoError(t, err)
		requirePositiveZerof(t, p.Mag(), "−3 mm × 0 mm")

		q, err := zero.Div(minus)
		require.NoError(t, err)
		requirePositiveZerof(t, q.Mag(), "0 mm ÷ −3 mm")

		// …including at the ends of the range, where the rational path runs: a true
		// product too small for the smallest subnormal is a zero, not a signed one.
		p, err = units.New(-1e-300, tinyLength).Mul(units.New(1e-300, tinyLength))
		require.NoError(t, err)
		requirePositiveZerof(t, p.Mag(), "a product that underflows to nothing")

		q, err = units.New(-1e-300, tinyLength).Div(units.New(1e300, hugeLength))
		require.NoError(t, err)
		requirePositiveZerof(t, q.Mag(), "a quotient that underflows to nothing")

		for _, u := range unitsOfKind(units.Length) {
			m, err := zero.In(u)
			require.NoError(t, err)
			requirePositiveZerof(t, m, "0 mm in %s", u)

			c, err := zero.Convert(u)
			require.NoError(t, err)
			requirePositiveZerof(t, c.Mag(), "0 mm converted to %s", u)
			requirePositiveZerof(t, c.Base(), "…and its base magnitude")
		}

		// A negative quantity that no float64 of the target unit can hold converts to a
		// zero — a positive one, like every other zero the package hands back.
		m, err := units.Millimeters(-5e-324).In(hugeLength)
		require.NoError(t, err)
		requirePositiveZerof(t, m, "a negative length that underflows into a coarse unit")

		requirePositiveZerof(t, units.Metric().In(units.New(-5e-324, tinyLength)),
			"System.In: the same conversion, with no error to report")
		requirePositiveZerof(t, units.Metric().Display(units.Millimeters(negativeZero())).Mag(),
			"System.Display")
	})

	t.Run("the sweep", func(t *testing.T) {
		// Every operation, over every unit of every kind, at both zeros and at the
		// operands that cancel: not one negative zero anywhere in the API.
		for _, k := range equalSweepKinds() {
			for _, ua := range equalSweepUnits(k) {
				for _, ub := range equalSweepUnits(k) {
					for _, ma := range []float64{0, negativeZero()} {
						a, b := units.New(ma, ua), units.New(negativeZero(), ub)

						for _, o := range addSubOps() {
							s, err := o.do(a, b)
							require.NoError(t, err, "%s %s %s", a, o.op, b)
							requirePositiveZerof(t, s.Mag(), "%s %s %s", a, o.op, b)
							requirePositiveZerof(t, s.Base(), "%s %s %s, in base units", a, o.op, b)
						}

						p, err := a.Mul(units.Scalar(-1))
						require.NoError(t, err)
						requirePositiveZerof(t, p.Mag(), "%s × −1", a)

						q, err := a.Div(units.Scalar(-1))
						require.NoError(t, err)
						requirePositiveZerof(t, q.Mag(), "%s ÷ −1", a)

						requirePositiveZerof(t, a.Neg().Mag(), "−%s", a)
						requirePositiveZerof(t, a.Scale(-1).Mag(), "%s scaled by −1", a)

						m, err := a.In(ub)
						require.NoError(t, err)
						requirePositiveZerof(t, m, "%s in %s", a, ub)
					}
				}
			}
		}
	})
}
