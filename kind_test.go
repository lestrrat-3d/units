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

// namedKinds are the nine kinds the package names and registers a base unit for.
// A saturated kind must never compare equal to one of them.
func namedKinds() []units.Kind {
	return []units.Kind{
		units.Dimensionless, units.Length, units.Area, units.Volume, units.Angle,
		units.Mass, units.Density, units.MomentOfInertia, units.SecondMomentOfArea,
	}
}

// saturatedHigh and saturatedLow are the endpoint kinds a length composed past
// the int8 range lands on. They are built by clamping — a Mul or a Div that runs
// off the end — rather than by an out-of-range Pow, so the Pow table below is
// compared against something it does not itself produce.
func saturatedHigh() units.Kind { return units.Length.Pow(127).Mul(units.Length) }
func saturatedLow() units.Kind  { return units.Length.Pow(-128).Div(units.Length) }

func TestKindExponentBoundary(t *testing.T) {
	// Exponents are int8: composition saturates at the endpoints rather than
	// wrapping into a plausible-looking wrong kind, and a kind that saturated says
	// so — its exponent is a clamped stand-in, not the number it stands for.
	hi := units.Length.Pow(127)
	require.False(t, hi.Overflowed(), "L¹²⁷ is exactly representable")
	require.Equal(t, "L¹²⁷", hi.String())

	hs := hi.Mul(units.Length)
	require.True(t, hs.Overflowed(), "L¹²⁸ does not fit: saturated at +127")
	require.NotEqual(t, hi, hs, "a saturated kind is not the endpoint it clamped to")
	require.Equal(t, "overflowed", hs.String())

	lo := units.Length.Pow(-128)
	require.False(t, lo.Overflowed(), "L⁻¹²⁸ is exactly representable")
	require.Equal(t, "L⁻¹²⁸", lo.String())

	ls := lo.Div(units.Length)
	require.True(t, ls.Overflowed(), "L⁻¹²⁹ does not fit: saturated at -128")
	require.NotEqual(t, lo, ls)

	require.NotEqual(t, hi, lo)
	require.NotEqual(t, hs, ls, "the two endpoints stay distinct once saturated")
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
		{"area^MinInt64", units.Area.Pow(math.MinInt64), saturatedLow()},
		{"area^MaxInt64", units.Area.Pow(math.MaxInt64), saturatedHigh()},
		{"area^(1<<62)", units.Area.Pow(1 << 62), saturatedHigh()},
		{"area^-(1<<62)", units.Area.Pow(-(1 << 62)), saturatedLow()},
		{"length^MinInt64", units.Length.Pow(math.MinInt64), saturatedLow()},
		{"length^MaxInt64", units.Length.Pow(math.MaxInt64), saturatedHigh()},
		{"volume^MaxInt32", units.Volume.Pow(math.MaxInt32), saturatedHigh()},
		{"angle^MinInt64", units.Angle.Pow(math.MinInt64), units.Angle.Pow(-128).Div(units.Angle)},
		{"density^MaxInt64", units.Density.Pow(math.MaxInt64), saturatedLow().Mul(units.Mass.Pow(127))},
		// The exponents that still fit are not overflow, however wide the multiplier
		// it took to reach them.
		{"length^127", units.Length.Pow(127), units.Length.Pow(127)},
		{"length^-128", units.Length.Pow(-128), units.Length.Pow(-128)},
		{"length^-128 via -(-128)", units.Length.Pow(-1).Pow(128), units.Length.Pow(-128)},
		// A dimensionless kind has no exponent to scale, so it stays itself.
		{"dimensionless^MinInt64", units.Dimensionless.Pow(math.MinInt64), units.Dimensionless},
		{"dimensionless^MaxInt64", units.Dimensionless.Pow(math.MaxInt64), units.Dimensionless},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.got)
			require.Equal(t, tc.want.Overflowed(), tc.got.Overflowed())
		})
	}
}

func TestKindPowNeverFabricatesANamedKind(t *testing.T) {
	// The point of saturating: no out-of-range power may quietly produce one of
	// the named kinds, which would read as a perfectly plausible result.
	powers := []int{
		math.MinInt64, math.MinInt64 + 1, math.MaxInt64, math.MaxInt64 - 1,
		1 << 62, -(1 << 62), math.MaxInt32, math.MinInt32, 1 << 40, -(1 << 40),
		128, -129, 1000, -1000,
	}
	for _, k := range namedKinds() {
		if k == units.Dimensionless {
			continue // every power of a dimensionless kind is dimensionless
		}
		for _, n := range powers {
			got := k.Pow(n)
			require.True(t, got.Overflowed(), "%s.Pow(%d) does not fit an int8 exponent", k, n)
			for _, want := range namedKinds() {
				require.NotEqual(t, want, got,
					"%s.Pow(%d) must not fabricate the named kind %s", k, n, want)
			}
		}
	}
}

// saturate returns every way of driving k out of the int8 exponent range. The
// exponents that come back are clamped stand-ins for numbers astronomically
// larger, so the kind is a lie about its dimension — and must never again pass
// for one that is not.
func saturate(k units.Kind) map[string]units.Kind {
	return map[string]units.Kind{
		"Pow(MaxInt64)": k.Pow(math.MaxInt64),
		"Pow(MinInt64)": k.Pow(math.MinInt64),
		"Pow(1<<62)":    k.Pow(1 << 62),
	}
}

// ordinaryKinds are the kinds a saturated one is composed with below: the named
// ones, plus the unnamed kinds a caller reaches by ordinary arithmetic.
func ordinaryKinds() []units.Kind {
	return append(namedKinds(),
		units.Dimensionless.Div(units.Length),         // curvature, L⁻¹
		units.Mass.Div(units.Area),                    // areal density, L⁻²·M
		units.Length.Div(units.Angle),                 // L·A⁻¹
		units.Length.Pow(127),                         // the exponent endpoint itself
		units.Length.Pow(-128),                        // …and the other one
		units.Volume.Mul(units.Mass).Mul(units.Angle), // L³·M·A
		units.Angle.Pow(2),                            // A²
	)
}

func TestKindOverflowIsSticky(t *testing.T) {
	// The reproducer: a saturated L¹²⁷ divided by an L¹²⁶ is not a length. Its
	// true exponent is astronomical; a Kind that came back Length would hand a
	// consumer an overflowed quantity dressed as an ordinary one.
	maxInt := int(^uint(0) >> 1)
	overflowed := units.Length.Pow(maxInt)
	require.True(t, overflowed.Overflowed())

	back := overflowed.Div(units.Length.Pow(126))
	require.True(t, back.Overflowed(), "overflow does not divide away")
	require.NotEqual(t, units.Length, back, "a saturated exponent must not walk back into a named kind")

	// And no composition of a saturated kind with an ordinary one — in either
	// direction, or by any power — ever lands on a named kind or loses the flag.
	for _, k := range namedKinds() {
		if k == units.Dimensionless {
			continue // it has no exponent to saturate
		}
		for how, sat := range saturate(k) {
			require.True(t, sat.Overflowed(), "%s.%s saturates", k, how)
			require.Equal(t, "overflowed", sat.String())

			for _, o := range ordinaryKinds() {
				for name, got := range map[string]units.Kind{
					"sat.Mul(o)":        sat.Mul(o),
					"o.Mul(sat)":        o.Mul(sat),
					"sat.Div(o)":        sat.Div(o),
					"o.Div(sat)":        o.Div(sat),
					"sat.Pow(0)":        sat.Pow(0),
					"sat.Pow(1)":        sat.Pow(1),
					"sat.Pow(-1)":       sat.Pow(-1),
					"sat.Pow(3)":        sat.Pow(3),
					"sat.Div(sat)":      sat.Div(sat),
					"sat.Mul(sat)":      sat.Mul(sat),
					"sat.Div(o).Mul(o)": sat.Div(o).Mul(o),
				} {
					require.True(t, got.Overflowed(),
						"%s.%s: %s must stay overflowed", k, how, name)
					require.Equal(t, "overflowed", got.String())
					for _, want := range namedKinds() {
						require.NotEqual(t, want, got,
							"%s.%s: %s must not fabricate the named kind %s", k, how, name, want)
					}
				}
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
