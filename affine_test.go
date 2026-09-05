package units_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lestrrat-3d/units"
)

// tempTol is a tolerance in kelvin, the base unit of a temperature. The affine
// factors are rounded float64s (Fahrenheit's is 5/9), so a conversion round trip
// lands within a few ulps of the true value rather than on it.
const tempTol = 1e-9

func TestTemperatureConversion(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		from units.Value
		to   units.Unit
		want float64
	}{
		{name: "water freezes in kelvin", from: units.DegreesCelsius(0), to: units.Kelvin, want: 273.15},
		{name: "water boils in kelvin", from: units.DegreesCelsius(100), to: units.Kelvin, want: 373.15},
		{name: "absolute zero in celsius", from: units.Kelvins(0), to: units.Celsius, want: -273.15},
		{name: "water freezes in fahrenheit", from: units.DegreesCelsius(0), to: units.Fahrenheit, want: 32},
		{name: "water boils in fahrenheit", from: units.DegreesCelsius(100), to: units.Fahrenheit, want: 212},
		{name: "fahrenheit back to celsius", from: units.DegreesFahrenheit(32), to: units.Celsius, want: 0},
		{name: "the scales cross at -40", from: units.DegreesFahrenheit(-40), to: units.Celsius, want: -40},
		{name: "rankine shares the kelvin zero", from: units.Kelvins(0), to: units.Rankine, want: 0},
		{name: "a nozzle at 210 degC", from: units.DegreesCelsius(210), to: units.Kelvin, want: 483.15},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := tc.from.In(tc.to)
			require.NoError(t, err)
			require.InDelta(t, tc.want, got, tempTol)
		})
	}
}

func TestTemperatureRoundTripsThroughEveryUnit(t *testing.T) {
	t.Parallel()

	// Whatever a temperature is carried in, converting away and back is the same
	// quantity. This is the property the offset arithmetic has to keep.
	scales := []units.Unit{units.Kelvin, units.Celsius, units.Fahrenheit, units.Rankine}
	for _, from := range scales {
		for _, to := range scales {
			v := units.New(210, from)
			round, err := v.Convert(to)
			require.NoError(t, err)
			back, err := round.Convert(from)
			require.NoError(t, err)
			require.True(t, v.Equal(back, tempTol),
				"%s -> %s -> %s gave %s", v, to, from, back)
		}
	}
}

func TestAffineUnitHasNoArithmetic(t *testing.T) {
	t.Parallel()

	c := units.DegreesCelsius(20)
	k := units.Kelvins(5)

	for _, tc := range []struct {
		name string
		op   func() (units.Value, error)
	}{
		{name: "add affine to affine", op: func() (units.Value, error) { return c.Add(c) }},
		{name: "add ratio to affine", op: func() (units.Value, error) { return c.Add(k) }},
		{name: "add affine to ratio", op: func() (units.Value, error) { return k.Add(c) }},
		{name: "sub", op: func() (units.Value, error) { return c.Sub(c) }},
		{name: "mul", op: func() (units.Value, error) { return c.Mul(units.Scalar(2)) }},
		{name: "mul from the other side", op: func() (units.Value, error) { return units.Scalar(2).Mul(c) }},
		{name: "div", op: func() (units.Value, error) { return c.Div(units.Scalar(2)) }},
		{name: "div by an affine", op: func() (units.Value, error) { return k.Div(c) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := tc.op()
			require.ErrorIs(t, err, units.ErrAffineUnit)
		})
	}
}

func TestKelvinKeepsItsArithmetic(t *testing.T) {
	t.Parallel()

	// The refusal is a property of the unit, not of the kind: converting to the
	// base unit buys the arithmetic back.
	k, err := units.DegreesCelsius(20).Convert(units.Kelvin)
	require.NoError(t, err)

	doubled, err := k.Mul(units.Scalar(2))
	require.NoError(t, err)
	require.InDelta(t, 586.3, doubled.Base(), tempTol)

	sum, err := k.Add(units.Kelvins(10))
	require.NoError(t, err)
	require.InDelta(t, 303.15, sum.Base(), tempTol)
}

func TestAffineUnitReporting(t *testing.T) {
	t.Parallel()

	require.True(t, units.Celsius.Affine())
	require.True(t, units.Fahrenheit.Affine())
	require.False(t, units.Kelvin.Affine())
	require.False(t, units.Rankine.Affine())
	require.False(t, units.Millimeter.Affine())

	require.Equal(t, 273.15, units.Celsius.Offset())
	require.Equal(t, 0.0, units.Kelvin.Offset())

	// The zero Unit reads as One, which shifts nothing.
	var zero units.Unit
	require.False(t, zero.Affine())
	require.Equal(t, 0.0, zero.Offset())
}

func TestAffineBaseAndFromBase(t *testing.T) {
	t.Parallel()

	// Base scales then shifts; FromBase undoes it in the other order.
	require.InDelta(t, 273.15, units.DegreesCelsius(0).Base(), tempTol)
	require.InDelta(t, 0.0, units.FromBase(273.15, units.Celsius).Mag(), tempTol)
	require.InDelta(t, 210.0, units.FromBase(483.15, units.Celsius).Mag(), tempTol)
}

func TestAffineEquality(t *testing.T) {
	t.Parallel()

	// Equal magnitudes in units that share a factor but not a zero are not the
	// same quantity: 0 degC and 0 K are 273.15 K apart.
	require.False(t, units.DegreesCelsius(0).Equal(units.Kelvins(0), 0))
	require.True(t, units.DegreesCelsius(0).Equal(units.Kelvins(273.15), tempTol))
	require.True(t, units.DegreesCelsius(-40).Equal(units.DegreesFahrenheit(-40), tempTol))
	require.False(t, units.DegreesCelsius(20).Equal(units.DegreesCelsius(21), 0))

	// Reflexive, and symmetric across units.
	c := units.DegreesCelsius(20)
	require.True(t, c.Equal(c, 0))
	k, err := c.Convert(units.Kelvin)
	require.NoError(t, err)
	require.True(t, k.Equal(c, tempTol))
	require.True(t, c.Equal(k, tempTol))
}

func TestAffineTextRoundTrip(t *testing.T) {
	t.Parallel()

	type profile struct {
		Nozzle units.Value `json:"nozzle"`
	}

	data, err := json.Marshal(profile{Nozzle: units.DegreesCelsius(210)})
	require.NoError(t, err)
	require.JSONEq(t, `{"nozzle":"210 degC"}`, string(data))

	var got profile
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, units.Celsius, got.Nozzle.Unit())
	require.Equal(t, 210.0, got.Nozzle.Mag())
}

func TestDefineAffineRejectsNonFiniteOffset(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		units.DefineAffine("probe-inf-offset", units.Temperature, 1, math.Inf(1))
	})
	require.Panics(t, func() {
		units.DefineAffine("probe-nan-offset", units.Temperature, 1, math.NaN())
	})

	// A zero offset is an ordinary ratio unit, which is what Define builds.
	u := units.DefineAffine("probe-zero-offset", units.Temperature, 2, 0)
	require.False(t, u.Affine())
}
