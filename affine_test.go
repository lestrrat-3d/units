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

func TestAffineToRatio(t *testing.T) {
	t.Parallel()

	// ToRatio is the crossing between the two types, and the reason an affine
	// quantity needs no arithmetic of its own: it converts to one that has it.
	for _, tc := range []struct {
		name string
		from units.AffineValue
		to   units.Unit
		want float64
	}{
		{name: "water freezes in kelvin", from: units.DegreesCelsius(0), to: units.Kelvin, want: 273.15},
		{name: "water boils in kelvin", from: units.DegreesCelsius(100), to: units.Kelvin, want: 373.15},
		{name: "a nozzle at 210 degC", from: units.DegreesCelsius(210), to: units.Kelvin, want: 483.15},
		{name: "fahrenheit to kelvin", from: units.DegreesFahrenheit(32), to: units.Kelvin, want: 273.15},
		{name: "fahrenheit to rankine", from: units.DegreesFahrenheit(0), to: units.Rankine, want: 459.67},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := tc.from.ToRatio(tc.to)
			require.NoError(t, err)
			require.InDelta(t, tc.want, got.Mag(), tempTol)
			require.Equal(t, tc.to, got.Unit())
		})
	}
}

func TestAffineConvert(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		from units.AffineValue
		to   units.AffineUnit
		want float64
	}{
		{name: "water freezes in fahrenheit", from: units.DegreesCelsius(0), to: units.Fahrenheit, want: 32},
		{name: "water boils in fahrenheit", from: units.DegreesCelsius(100), to: units.Fahrenheit, want: 212},
		{name: "fahrenheit back to celsius", from: units.DegreesFahrenheit(32), to: units.Celsius, want: 0},
		{name: "the scales cross at -40", from: units.DegreesFahrenheit(-40), to: units.Celsius, want: -40},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := tc.from.Convert(tc.to)
			require.NoError(t, err)
			require.InDelta(t, tc.want, got.Mag(), tempTol)
		})
	}
}

func TestValueToAffine(t *testing.T) {
	t.Parallel()

	// And back the other way, so the two types are not a one-way door.
	c, err := units.Kelvins(293.15).ToAffine(units.Celsius)
	require.NoError(t, err)
	require.InDelta(t, 20.0, c.Mag(), tempTol)

	f, err := units.Kelvins(273.15).ToAffine(units.Fahrenheit)
	require.NoError(t, err)
	require.InDelta(t, 32.0, f.Mag(), tempTol)
}

func TestTemperatureRoundTripsThroughEveryScale(t *testing.T) {
	t.Parallel()

	// Whatever a temperature is carried in, converting away and back is the same
	// quantity. The property now has to hold across the type boundary as well as
	// within it.
	for _, from := range []units.AffineUnit{units.Celsius, units.Fahrenheit} {
		for _, to := range []units.AffineUnit{units.Celsius, units.Fahrenheit} {
			v := units.NewAffine(210, from)
			round, err := v.Convert(to)
			require.NoError(t, err)
			back, err := round.Convert(from)
			require.NoError(t, err)
			require.True(t, v.Equal(back, tempTol), "%s -> %s -> %s gave %s", v, to, from, back)
		}
		for _, to := range []units.Unit{units.Kelvin, units.Rankine} {
			v := units.NewAffine(210, from)
			round, err := v.ToRatio(to)
			require.NoError(t, err)
			back, err := round.ToAffine(from)
			require.NoError(t, err)
			require.True(t, v.Equal(back, tempTol), "%s -> %s -> %s gave %s", v, to, from, back)
		}
	}
}

func TestAffineKindsDoNotCoerce(t *testing.T) {
	t.Parallel()

	_, err := units.DegreesCelsius(20).ToRatio(units.Millimeter)
	require.ErrorIs(t, err, units.ErrIncompatible)

	_, err = units.DegreesCelsius(20).In(units.Second)
	require.ErrorIs(t, err, units.ErrIncompatible)

	_, err = units.Millimeters(20).ToAffine(units.Celsius)
	require.ErrorIs(t, err, units.ErrIncompatible)
}

func TestZeroAffineUnitIsNotAQuantity(t *testing.T) {
	t.Parallel()

	// The zero Value reads as 0 of One, but there is no natural affine unit to
	// fall back on, so the zero AffineValue reports rather than invents.
	var zero units.AffineValue
	require.False(t, zero.Unit().Valid())

	_, err := zero.ToRatio(units.Kelvin)
	require.ErrorIs(t, err, units.ErrNotAffine)

	_, err = zero.MarshalText()
	require.ErrorIs(t, err, units.ErrNotAffine)

	require.True(t, math.IsNaN(zero.Base()))
	require.False(t, zero.Equal(zero, math.Inf(1)))

	var zeroUnit units.AffineUnit
	_, err = units.Kelvins(1).ToAffine(zeroUnit)
	require.ErrorIs(t, err, units.ErrNotAffine)
}

func TestAffineEquality(t *testing.T) {
	t.Parallel()

	require.True(t, units.DegreesCelsius(-40).Equal(units.DegreesFahrenheit(-40), tempTol))
	require.False(t, units.DegreesCelsius(20).Equal(units.DegreesCelsius(21), 0))
	require.True(t, units.DegreesCelsius(20).Equal(units.DegreesCelsius(20), 0))

	// Across the two types: 0 degC really is 273.15 K.
	require.True(t, units.DegreesCelsius(0).EqualValue(units.Kelvins(273.15), tempTol))
	require.False(t, units.DegreesCelsius(0).EqualValue(units.Kelvins(0), tempTol))
}

func TestAffineTextRoundTrip(t *testing.T) {
	t.Parallel()

	type profile struct {
		Nozzle units.AffineValue `json:"nozzle"`
	}

	data, err := json.Marshal(profile{Nozzle: units.DegreesCelsius(210)})
	require.NoError(t, err)
	require.JSONEq(t, `{"nozzle":"210 degC"}`, string(data))

	var got profile
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, units.Celsius, got.Nozzle.Unit())
	require.Equal(t, 210.0, got.Nozzle.Mag())
}

func TestTextFormsDoNotCrossTypes(t *testing.T) {
	t.Parallel()

	// A ratio symbol is not an affine unit and vice versa, so neither type can be
	// made to hold the other's quantity by way of a document.
	var a units.AffineValue
	require.ErrorIs(t, a.UnmarshalText([]byte("273.15 K")), units.ErrUnknownUnit)

	var v units.Value
	require.ErrorIs(t, v.UnmarshalText([]byte("210 degC")), units.ErrUnknownUnit)

	// And the registries agree on which symbol is which.
	_, ok := units.Lookup("degC")
	require.False(t, ok)
	_, ok = units.LookupAffine("K")
	require.False(t, ok)
	_, ok = units.LookupAffine("degC")
	require.True(t, ok)
}

func TestDefineAffineRejects(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		units.DefineAffine("probe-inf-offset", units.Temperature, 1, math.Inf(1))
	})
	require.Panics(t, func() {
		units.DefineAffine("probe-nan-offset", units.Temperature, 1, math.NaN())
	})
	// A zero offset is a ratio unit; there is no unit that is both.
	require.Panics(t, func() {
		units.DefineAffine("probe-zero-offset", units.Temperature, 2, 0)
	})
	// The symbol namespace is shared with the ratio units, in both directions.
	require.Panics(t, func() {
		units.DefineAffine("mm", units.Temperature, 1, 10)
	})
	require.Panics(t, func() {
		units.Define("degC", units.Temperature, 1)
	})

	u := units.DefineAffine("probe-ok", units.Temperature, 2, 7)
	require.True(t, u.Valid())
	require.Equal(t, 7.0, u.Offset())
	require.Equal(t, units.Temperature, u.Kind())
}
