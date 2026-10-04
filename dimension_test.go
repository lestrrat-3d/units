package units_test

import (
	"encoding/json"
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

func TestRigidDynamicsDimensionsComposeAndPersist(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		value units.Value
		kind  units.Kind
		base  units.Unit
		text  string
	}{
		{"angular velocity", units.RadiansPerSecond(2), units.AngularVelocity, units.RadianPerSecond, `"2 rad/s"`},
		{"impulse", units.KilogramMillimetersPerSecond(3), units.Impulse,
			units.KilogramMillimeterPerSecond, `"3 kg*mm/s"`},
		{"force", units.KilogramMillimetersPerSecondSquared(4), units.Force,
			units.KilogramMillimeterPerSecondSquared, `"4 kg*mm/s^2"`},
		{"torque", units.KilogramSquareMillimetersPerSecondSquared(5), units.Torque,
			units.KilogramSquareMillimeterPerSecondSquared, `"5 kg*mm^2/s^2"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.kind, tc.value.Kind())
			base, ok := units.BaseUnit(tc.kind)
			require.True(t, ok)
			require.Equal(t, tc.base, base)
			data, err := json.Marshal(tc.value)
			require.NoError(t, err)
			require.JSONEq(t, tc.text, string(data))
			var decoded units.Value
			require.NoError(t, json.Unmarshal(data, &decoded))
			require.Equal(t, tc.value, decoded)
		})
	}

	angular, err := units.Radians(6).Div(units.Seconds(2))
	require.NoError(t, err)
	require.Equal(t, units.AngularVelocity, angular.Kind())

	impulse, err := units.Kilograms(2).Mul(units.MillimetersPerSecond(3))
	require.NoError(t, err)
	require.Equal(t, units.Impulse, impulse.Kind())

	force, err := impulse.Div(units.Seconds(2))
	require.NoError(t, err)
	require.Equal(t, units.Force, force.Kind())

	torque, err := force.Mul(units.Millimeters(5))
	require.NoError(t, err)
	require.Equal(t, units.Torque, torque.Kind())

	_, err = units.RadiansPerSecond(1).In(units.MillimeterPerSecond)
	require.ErrorIs(t, err, units.ErrIncompatible)
}

func TestRigidDynamicsUnitConversions(t *testing.T) {
	t.Parallel()

	degrees, err := units.RadiansPerSecond(1).In(units.DegreePerSecond)
	require.NoError(t, err)
	require.InDelta(t, 180/3.141592653589793, degrees, 1e-12)

	force, err := units.Newtons(1).In(units.KilogramMillimeterPerSecondSquared)
	require.NoError(t, err)
	require.Equal(t, 1000.0, force)

	torque, err := units.NewtonMillimeters(1).In(units.KilogramSquareMillimeterPerSecondSquared)
	require.NoError(t, err)
	require.Equal(t, 1000.0, torque)
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
		})
	}
}
