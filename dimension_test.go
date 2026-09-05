package units_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lestrrat-3d/units"
)

func TestTimeDimensionComposes(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		op   func() (units.Value, error)
		want units.Kind
	}{
		{
			name: "length over time is a velocity",
			op:   func() (units.Value, error) { return units.Millimeters(60).Div(units.Seconds(1)) },
			want: units.Velocity,
		},
		{
			name: "velocity over time is an acceleration",
			op: func() (units.Value, error) {
				return units.MillimetersPerSecond(500).Div(units.Seconds(1))
			},
			want: units.Acceleration,
		},
		{
			name: "velocity times time is a length",
			op: func() (units.Value, error) {
				return units.MillimetersPerSecond(60).Mul(units.Seconds(2))
			},
			want: units.Length,
		},
		{
			name: "acceleration times time is a velocity",
			op: func() (units.Value, error) {
				return units.MillimetersPerSecondSquared(100).Mul(units.Seconds(3))
			},
			want: units.Velocity,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := tc.op()
			require.NoError(t, err)
			require.Equal(t, tc.want, got.Kind())
		})
	}
}

func TestTimeAndVelocityConversion(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		from units.Value
		to   units.Unit
		want float64
	}{
		{name: "a minute is sixty seconds", from: units.Minutes(1), to: units.Second, want: 60},
		{name: "an hour is sixty minutes", from: units.Hours(1), to: units.Minute, want: 60},
		{name: "a millisecond is a thousandth", from: units.Milliseconds(1), to: units.Second, want: 0.001},
		{
			name: "a gcode feedrate is mm per minute",
			from: units.MillimetersPerSecond(60),
			to:   units.MillimeterPerMinute,
			want: 3600,
		},
		{name: "a metre per second is a thousand mm/s", from: units.MetersPerSecond(1), to: units.MillimeterPerSecond, want: 1000},
		{
			name: "acceleration in metres",
			from: units.MetersPerSecondSquared(1),
			to:   units.MillimeterPerSecondSquared,
			want: 1000,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := tc.from.In(tc.to)
			require.NoError(t, err)
			require.InEpsilon(t, tc.want, got, 1e-12)
		})
	}
}

func TestNewDimensionsDoNotCoerce(t *testing.T) {
	t.Parallel()

	// The whole point of a dimension is that it never silently becomes another.
	_, err := units.Seconds(1).In(units.Millimeter)
	require.ErrorIs(t, err, units.ErrIncompatible)

	_, err = units.MillimetersPerSecond(1).In(units.Millimeter)
	require.ErrorIs(t, err, units.ErrIncompatible)

	_, err = units.Kelvins(1).In(units.Second)
	require.ErrorIs(t, err, units.ErrIncompatible)

	_, err = units.Kelvins(1).Add(units.Seconds(1))
	require.ErrorIs(t, err, units.ErrIncompatible)
}

func TestKindNamesAndSymbolsAreStable(t *testing.T) {
	t.Parallel()

	require.Equal(t, "time", units.Time.String())
	require.Equal(t, "velocity", units.Velocity.String())
	require.Equal(t, "acceleration", units.Acceleration.String())
	require.Equal(t, "temperature", units.Temperature.String())

	// Time and temperature were added after length, mass and angle, and they sit
	// after them in the dimension order. A kind that predates them therefore still
	// prints exactly as it did, so no document written before they existed now
	// resolves to a different kind.
	inverseLength := units.Dimensionless.Div(units.Length)
	require.Equal(t, "L⁻¹", inverseLength.String())

	_, err := units.Scalar(1).Div(units.Millimeters(2))
	require.NoError(t, err)
}

func TestNewKindsHaveBaseUnits(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		kind units.Kind
		want units.Unit
	}{
		{kind: units.Time, want: units.Second},
		{kind: units.Velocity, want: units.MillimeterPerSecond},
		{kind: units.Acceleration, want: units.MillimeterPerSecondSquared},
		{kind: units.Temperature, want: units.Kelvin},
	} {
		t.Run(tc.kind.String(), func(t *testing.T) {
			t.Parallel()

			got, ok := units.BaseUnit(tc.kind)
			require.True(t, ok)
			require.Equal(t, tc.want, got)
			require.Equal(t, 1.0, got.Factor())
			require.False(t, got.Affine())
		})
	}
}
