package units_test

import (
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

func TestLookupRoundTrip(t *testing.T) {
	for _, u := range []units.Unit{
		units.One,
		units.Millimeter, units.Centimeter, units.Meter, units.Inch, units.Foot, units.Thou,
		units.SquareMillimeter, units.SquareCentimeter, units.SquareMeter, units.SquareInch,
		units.CubicMillimeter, units.CubicCentimeter, units.CubicMeter, units.CubicInch, units.Liter,
		units.Kilogram, units.Gram, units.Pound,
		units.KilogramPerCubicMillimeter, units.KilogramPerCubicMeter, units.GramPerCubicCentimeter,
		units.KilogramSquareMillimeter, units.QuarticMillimeter,
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

	// A divisor that is nonzero in its own unit but underflows to zero in the
	// base unit is still a division by zero.
	_, err = units.Millimeters(1).Div(units.KilogramsPerCubicMeter(1e-320))
	require.ErrorIs(t, err, units.ErrDivideByZero, "a divisor that underflows to a zero base")

	// A subnormal divisor does not underflow, but blows the quotient up to +Inf.
	// The contract is a finite quotient or an error, never an infinity — and the
	// error says what actually happened, which is an overflow, not a zero divisor.
	_, err = units.Millimeters(1).Div(units.Kilograms(5e-324))
	require.ErrorIs(t, err, units.ErrNotFinite, "a divisor that overflows the quotient")
	require.NotErrorIs(t, err, units.ErrDivideByZero, "the divisor is not zero")

	_, err = units.Millimeters(-1).Div(units.Kilograms(5e-324))
	require.ErrorIs(t, err, units.ErrNotFinite, "…and in the negative direction")

	// The finite quotients around it still divide.
	q, err := units.Millimeters(1).Div(units.Kilograms(2))
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
	require.InDelta(t, 0, v.Mag(), 0)
	require.InDelta(t, 0, v.Base(), 0)
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

	// It scales, converts and compares like any other dimensionless value.
	require.InDelta(t, 0, v.Scale(3).Base(), 0)
	require.InDelta(t, 0, v.Neg().Base(), 0)
	require.True(t, v.Equal(units.Scalar(0), 0), "the zero Value equals a zero scalar")

	one, err := v.In(units.One)
	require.NoError(t, err)
	require.InDelta(t, 0, one, 0)

	// The angle carve-out reaches it too: 0 + 90deg is 90deg, not +Inf.
	ang, err := v.Add(units.Degrees(90))
	require.NoError(t, err)
	require.Equal(t, units.Angle, ang.Kind())
	require.InDelta(t, 90, ang.Mag(), 1e-12)

	// It multiplies and divides without fabricating a NaN.
	p, err := v.Mul(units.Millimeters(3))
	require.NoError(t, err)
	require.Equal(t, units.Length, p.Kind())
	require.InDelta(t, 0, p.Base(), 0)

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
	require.InDelta(t, 0, s.Mag(), 0)

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
		require.Equal(t, 1e307, m)
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
		require.Equal(t, 1e307, c.Mag())
	})

	t.Run("Div: a value divided by itself is 1", func(t *testing.T) {
		q, err := huge.Div(huge)
		require.NoError(t, err)
		require.Equal(t, units.Dimensionless, q.Kind())
		require.Equal(t, 1.0, q.Mag())
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
		require.Equal(t, 1e308, s.Mag(), "a radian is far below the last ulp of 1e308 turns")

		s, err = units.Scalar(1).Add(rev)
		require.NoError(t, err)
		require.Equal(t, units.Angle, s.Kind(), "scalar + angle is an angle whichever side it appeared on")
		require.Equal(t, turn, s.Unit())
		require.Equal(t, 1e308, s.Mag())

		// The same arm with an angle on both sides was already ratio-first.
		s, err = rev.Add(units.Radians(1))
		require.NoError(t, err)
		require.Equal(t, 1e308, s.Mag())
	})

	t.Run("System.In and System.Display", func(t *testing.T) {
		// Metric presents a length in millimetres. 1e307 m is 1e310 mm, which no
		// float64 holds — so the number must not come back as 1e307, which the
		// caller would read as 1e307 mm: finite, wrong by a factor of 1000, silent.
		require.True(t, math.IsInf(units.Metric().In(huge), 1),
			"an unrepresentable magnitude is an infinity, never a finite number in the wrong unit")

		// The representable ones come back in the system's unit, as always.
		require.Equal(t, 1e307, units.SI().In(huge), "SI presents a length in metres")
		require.InEpsilon(t, 2000.0, units.Metric().In(units.Meters(2)), 1e-12)

		// Display cannot carry the quantity in millimetres, so it hands back the
		// value it was given rather than a relabelled magnitude.
		d := units.Metric().Display(huge)
		require.Equal(t, units.Length, d.Kind())
		require.Equal(t, units.Meter, d.Unit())
		require.Equal(t, 1e307, d.Mag())

		require.Equal(t, units.Millimeter, units.Metric().Display(units.Meters(2)).Unit())
	})
}

// The oracle for the sweep below: exact rational arithmetic. A float64 magnitude
// and a float64 factor are both exact rationals, so their product is the true
// quantity a Value denotes — whether or not float64 can hold it.
var (
	maxFloat   = new(big.Rat).SetFloat64(math.MaxFloat64)
	halfMax    = new(big.Rat).Quo(maxFloat, big.NewRat(2, 1))
	twiceMax   = new(big.Rat).Mul(maxFloat, big.NewRat(2, 1))
	relTol     = big.NewRat(1, 1e9)
	subnormalF = new(big.Rat).SetFloat64(1e-310)
)

// baseRat returns v's base magnitude exactly.
func baseRat(t *testing.T, v units.Value) *big.Rat {
	t.Helper()

	m := new(big.Rat).SetFloat64(v.Mag())
	require.NotNil(t, m, "the matrix holds finite magnitudes only")
	f := new(big.Rat).SetFloat64(v.Unit().Factor())
	require.NotNil(t, f, "a unit factor is finite")
	return new(big.Rat).Mul(m, f)
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
func requireClose(t *testing.T, got float64, want *big.Rat) {
	t.Helper()

	g := new(big.Rat).SetFloat64(got)
	require.NotNil(t, g, "the true result %s is representable, so the result is finite: got %v",
		want.FloatString(3), got)

	diff := new(big.Rat).Abs(new(big.Rat).Sub(g, want))
	tol := new(big.Rat).Mul(new(big.Rat).Abs(want), relTol)
	tol.Add(tol, subnormalF)
	require.LessOrEqual(t, diff.Cmp(tol), 0, "want %s, got %v", want.FloatString(3), got)
}

// requireResult asserts the contract of an operation that can report an error:
// the true result whenever float64 can hold it, and ErrNotFinite only when it
// genuinely cannot. Within a factor of two of MaxFloat64 the rounding decides,
// so either answer is honest there.
func requireResult(t *testing.T, got float64, err error, want *big.Rat) {
	t.Helper()

	abs := new(big.Rat).Abs(want)
	switch {
	case abs.Cmp(halfMax) <= 0:
		require.NoError(t, err, "the true result %s is representable", want.FloatString(3))
		requireClose(t, got, want)
	case abs.Cmp(twiceMax) >= 0:
		require.ErrorIs(t, err, units.ErrNotFinite, "the true result %s overflows float64", want.FloatString(3))
	case err != nil:
		require.ErrorIs(t, err, units.ErrNotFinite)
	default:
		requireClose(t, got, want)
	}
}

// requireFloat asserts the same contract for an operation with no error to
// report: the true result where float64 holds it, an infinity of the right sign
// where it does not — never a finite number that is not the quantity asked for.
func requireFloat(t *testing.T, got float64, want *big.Rat) {
	t.Helper()

	abs := new(big.Rat).Abs(want)
	switch {
	case abs.Cmp(halfMax) <= 0:
		requireClose(t, got, want)
	case abs.Cmp(twiceMax) >= 0:
		require.True(t, math.IsInf(got, want.Sign()),
			"the true result %s overflows float64, so it comes back as an infinity: got %v",
			want.FloatString(3), got)
	case !math.IsInf(got, 0):
		requireClose(t, got, want)
	}
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
			require.Equal(t, v.Mag(), m, "%s in its own unit is its own magnitude", v)

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
				require.Equal(t, m, c.Mag(), "Convert agrees with In: %s in %s", v, u)
			}
		}
	})

	t.Run("Add and Sub", func(t *testing.T) {
		for _, a := range extremes() {
			for _, b := range extremes() {
				sameKind := a.Kind() == b.Kind()
				carveOut := (a.Kind() == units.Angle && b.Kind() == units.Dimensionless) ||
					(a.Kind() == units.Dimensionless && b.Kind() == units.Angle)

				if !sameKind && !carveOut {
					_, err := a.Add(b)
					require.ErrorIs(t, err, units.ErrIncompatible, "%s + %s", a, b)
					_, err = a.Sub(b)
					require.ErrorIs(t, err, units.ErrIncompatible, "%s - %s", a, b)
					continue
				}

				// The sum is carried in a's unit — except in the carve-out entered
				// from the dimensionless side, where it is carried in b's.
				u := a.Unit()
				if a.Kind() == units.Dimensionless && b.Kind() == units.Angle {
					u = b.Unit()
				}
				f := ratOf(t, u.Factor())

				for _, op := range []struct {
					name string
					sign int
					do   func(units.Value, units.Value) (units.Value, error)
				}{
					{"add", 1, units.Value.Add},
					{"sub", -1, units.Value.Sub},
				} {
					ob := baseRat(t, b)
					if op.sign < 0 {
						ob = new(big.Rat).Neg(ob)
					}
					want := new(big.Rat).Quo(new(big.Rat).Add(baseRat(t, a), ob), f)

					r, err := op.do(a, b)
					requireResult(t, r.Mag(), err, want)
					if err != nil {
						require.Equal(t, units.Value{}, r, "no value escapes with the error")
						continue
					}
					require.Equal(t, u, r.Unit(), "%s: %s %s %s", op.name, a, op.name, b)
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

				// A divisor whose base magnitude is zero — genuinely, or by
				// underflow — is a division by zero, not an infinity.
				if b.Base() == 0 {
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
			require.Equal(t, -v.Mag(), v.Neg().Mag(), "%s negates", v)
			require.Equal(t, v.Unit(), v.Neg().Unit(), "…in its own unit")
			require.True(t, v.Neg().Neg().Equal(v, 0), "…and negating twice is the identity")

			require.Equal(t, v.Mag(), v.Scale(1).Mag(), "scaling by 1 is the identity")
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
					require.Equal(t, v.Mag(), d.Mag())
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
