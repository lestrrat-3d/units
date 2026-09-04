package units_test

import (
	"math"
	"testing"

	"github.com/lestrrat-3d/units"
	"github.com/stretchr/testify/require"
)

var valueStringUnit = units.Define("string-test-unit", units.Length, 1)

func TestValueStringOutput(t *testing.T) {
	tests := []struct {
		name  string
		value units.Value
		want  string
	}{
		{name: "dimensionless", value: units.Scalar(1.5), want: "1.5"},
		{name: "dimensionless infinity", value: units.Scalar(math.Inf(1)), want: "+Inf"},
		{name: "dimensionless nan", value: units.Scalar(math.NaN()), want: "NaN"},
		{name: "unit bearing", value: units.Millimeters(100), want: "100 mm"},
		{name: "custom symbol", value: units.New(2, valueStringUnit), want: "2 string-test-unit"},
		{name: "positive infinity", value: units.New(math.Inf(1), units.Millimeter), want: "+Inf mm"},
		{name: "negative infinity", value: units.New(math.Inf(-1), units.Millimeter), want: "-Inf mm"},
		{name: "nan", value: units.New(math.NaN(), units.Millimeter), want: "NaN mm"},
		{name: "dimensionless canonical zero", value: units.Scalar(math.Copysign(0, -1)), want: "0"},
		{name: "canonical zero", value: units.New(math.Copysign(0, -1), units.Millimeter), want: "0 mm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.value.String())
		})
	}
}

var valueStringSink string

func BenchmarkLegacyValueString(b *testing.B) {
	tests := []struct {
		name  string
		value units.Value
	}{
		{name: "dimensionless", value: units.Scalar(1.5)},
		{name: "unit bearing", value: units.Millimeters(100)},
		{name: "infinity", value: units.New(math.Inf(1), units.Millimeter)},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				valueStringSink = tt.value.String()
			}
		})
	}
}
