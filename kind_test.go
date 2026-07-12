package units_test

import (
	"math"
	"testing"

	"github.com/lestrrat-3d/units"
	"github.com/stretchr/testify/require"
)

func TestKindComposition(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  units.Kind
		want units.Kind
	}{
		{"length x length = area", units.Length.Mul(units.Length), units.Area},
		{"area x length = volume", units.Area.Mul(units.Length), units.Volume},
		{"volume / area = length", units.Volume.Div(units.Area), units.Length},
		{"mass / volume = density", units.Mass.Div(units.Volume), units.Density},
		{"mass x area = moment of inertia", units.Mass.Mul(units.Area), units.MomentOfInertia},
		{"area x area = second moment of area", units.Area.Mul(units.Area), units.SecondMomentOfArea},
		{"length^3 = volume", units.Length.Pow(3), units.Volume},
		{"length^4 = second moment of area", units.Length.Pow(4), units.SecondMomentOfArea},
		{"length^0 = dimensionless", units.Length.Pow(0), units.Dimensionless},
		{"length^1 = length", units.Length.Pow(1), units.Length},
		{"density^0 = dimensionless", units.Density.Pow(0), units.Dimensionless},
		{"k x dimensionless = k", units.Density.Mul(units.Dimensionless), units.Density},
		{"dimensionless x k = k", units.Dimensionless.Mul(units.Volume), units.Volume},
		{"k / k = dimensionless", units.MomentOfInertia.Div(units.MomentOfInertia), units.Dimensionless},
		{"k / dimensionless = k", units.Angle.Div(units.Dimensionless), units.Angle},
		{"volume / length^-1 = second moment of area", units.Volume.Div(units.Dimensionless.Div(units.Length)), units.SecondMomentOfArea},
		{"zero value is dimensionless", units.Kind{}, units.Dimensionless},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.got)
		})
	}
}

func TestKindDistinct(t *testing.T) {
	// An angle is dimensionless in physics; here it never unifies with a bare
	// number, or Scalar(2) would silently pass as 2 radians.
	require.NotEqual(t, units.Dimensionless, units.Angle, "angle must not unify with dimensionless")
	require.NotEqual(t, units.Angle, units.Angle.Mul(units.Angle), "an angle is not a squared angle")
	require.NotEqual(t, units.Area, units.Volume)
	require.NotEqual(t, units.Density, units.MomentOfInertia)
}

func TestKindString(t *testing.T) {
	for _, tc := range []struct {
		want string
		kind units.Kind
	}{
		{"dimensionless", units.Dimensionless},
		{"length", units.Length},
		{"area", units.Area},
		{"volume", units.Volume},
		{"angle", units.Angle},
		{"mass", units.Mass},
		{"density", units.Density},
		{"moment of inertia", units.MomentOfInertia},
		{"second moment of area", units.SecondMomentOfArea},
		{"L⁻¹", units.Dimensionless.Div(units.Length)},                    // curvature
		{"L⁵", units.Length.Pow(5)},                                       // unnamed, no base unit
		{"L⁻²·M", units.Mass.Div(units.Area)},                             // areal density
		{"L·A⁻¹", units.Length.Div(units.Angle)},                          // unnamed
		{"L³·M·A²", units.Volume.Mul(units.Mass).Mul(units.Angle.Pow(2))}, // unnamed
	} {
		t.Run(tc.want, func(t *testing.T) {
			require.Equal(t, tc.want, tc.kind.String())
		})
	}
}

func TestKindExponentBoundary(t *testing.T) {
	// Exponents are int8: composition saturates at the endpoints rather than
	// wrapping into a plausible-looking wrong kind.
	hi := units.Length.Pow(127)
	require.Equal(t, units.Length.Pow(127), hi.Mul(units.Length), "saturated at +127")
	require.Equal(t, "L¹²⁷", hi.String())

	lo := units.Length.Pow(-128)
	require.Equal(t, units.Length.Pow(-128), lo.Div(units.Length), "saturated at -128")
	require.Equal(t, "L⁻¹²⁸", lo.String())

	require.NotEqual(t, hi, lo)
}

func TestKindPowOverflow(t *testing.T) {
	// The multiplier is an int, far wider than the int8 an exponent is stored in.
	// Scaling must saturate to the endpoint the sign points at: an exponent that
	// wrapped could land back on a named, plausible-looking kind.
	for _, tc := range []struct {
		name string
		got  units.Kind
		want units.Kind
	}{
		{"area^MinInt64", units.Area.Pow(math.MinInt64), units.Length.Pow(-128)},
		{"area^MaxInt64", units.Area.Pow(math.MaxInt64), units.Length.Pow(127)},
		{"area^(1<<62)", units.Area.Pow(1 << 62), units.Length.Pow(127)},
		{"area^-(1<<62)", units.Area.Pow(-(1 << 62)), units.Length.Pow(-128)},
		{"length^MinInt64", units.Length.Pow(math.MinInt64), units.Length.Pow(-128)},
		{"length^MaxInt64", units.Length.Pow(math.MaxInt64), units.Length.Pow(127)},
		{"volume^MaxInt32", units.Volume.Pow(math.MaxInt32), units.Length.Pow(127)},
		{"angle^MinInt64", units.Angle.Pow(math.MinInt64), units.Angle.Pow(-128)},
		{"density^MaxInt64", units.Density.Pow(math.MaxInt64), units.Length.Pow(-128).Mul(units.Mass.Pow(127))},
		// A dimensionless kind has no exponent to scale, so it stays itself.
		{"dimensionless^MinInt64", units.Dimensionless.Pow(math.MinInt64), units.Dimensionless},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.got)
		})
	}
}

func TestKindPowNeverFabricatesANamedKind(t *testing.T) {
	// The point of saturating: no out-of-range power may quietly produce one of
	// the named kinds, which would read as a perfectly plausible result.
	named := []units.Kind{
		units.Dimensionless, units.Length, units.Area, units.Volume, units.Angle,
		units.Mass, units.Density, units.MomentOfInertia, units.SecondMomentOfArea,
	}
	powers := []int{
		math.MinInt64, math.MinInt64 + 1, math.MaxInt64, math.MaxInt64 - 1,
		1 << 62, -(1 << 62), math.MaxInt32, math.MinInt32, 1 << 40, -(1 << 40),
		128, -129, 1000, -1000,
	}
	for _, k := range named {
		if k == units.Dimensionless {
			continue // every power of a dimensionless kind is dimensionless
		}
		for _, n := range powers {
			got := k.Pow(n)
			for _, want := range named {
				require.NotEqual(t, want, got,
					"%s.Pow(%d) must not fabricate the named kind %s", k, n, want)
			}
		}
	}
}

func TestKindNoBaseUnit(t *testing.T) {
	// An unnamed kind is a first-class kind: it composes, it prints, and it
	// honestly reports that no base unit is registered for it.
	curvature := units.Dimensionless.Div(units.Length)

	_, ok := units.BaseUnit(curvature)
	require.False(t, ok, "an unnamed kind has no base unit")
	require.Equal(t, "L⁻¹", curvature.String())
	require.Equal(t, units.Dimensionless, curvature.Mul(units.Length), "round trip")

	u, ok := units.BaseUnit(units.Area)
	require.True(t, ok, "area has a base unit")
	require.Equal(t, units.SquareMillimeter, u)
}
