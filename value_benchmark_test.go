package units_test

import (
	"math"
	"sync/atomic"
	"testing"

	"github.com/lestrrat-3d/units"
)

var (
	benchmarkBoolSink   bool
	errBenchmarkSink    error
	benchmarkFloatSink  float64
	benchmarkStringSink string
	benchmarkUnitSink   units.Unit
	benchmarkValueSink  units.Value
	parallelFloatSink   atomic.Uint64
	parallelResultSink  atomic.Bool
)

func BenchmarkValueAdd(b *testing.B) {
	cases := []struct {
		name        string
		left, right units.Value
	}{
		{"SameUnit", units.Millimeters(12.5), units.Millimeters(7.25)},
		{"CrossUnit", units.Meters(1.25), units.Millimeters(725)},
		{"CrossUnitCancellation", units.Meters(-1), units.Millimeters(1000)},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			left, right := tc.left, tc.right
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				result, err := left.Add(right)
				benchmarkValueSink = result
				errBenchmarkSink = err
			}
		})
	}
}

func BenchmarkValueSub(b *testing.B) {
	cases := []struct {
		name        string
		left, right units.Value
	}{
		{"SameUnit", units.Millimeters(12.5), units.Millimeters(7.25)},
		{"CrossUnit", units.Meters(1.25), units.Millimeters(725)},
		{"CrossUnitCancellation", units.Meters(1), units.Millimeters(1000)},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			left, right := tc.left, tc.right
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				result, err := left.Sub(right)
				benchmarkValueSink = result
				errBenchmarkSink = err
			}
		})
	}
}

func BenchmarkValueEqual(b *testing.B) {
	cases := []struct {
		name        string
		left, right units.Value
		tolerance   float64
	}{
		{"SameUnitEqual", units.Meters(1), units.Meters(1), 0},
		{"SameUnitTolerance", units.Meters(1), units.Meters(1.0000001), 1e-4},
		{"CrossUnitEqual", units.Meters(1), units.Millimeters(1000), 0},
		{"CrossUnitMismatch", units.Meters(1), units.Millimeters(999), 0.1},
		{"CrossUnitCancellation", units.Meters(1), units.Millimeters(1000), 1e-12},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			left, right := tc.left, tc.right
			tolerance := tc.tolerance
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				benchmarkBoolSink = left.Equal(right, tolerance)
			}
		})
	}
}

func BenchmarkValueIn(b *testing.B) {
	cases := []struct {
		name  string
		value units.Value
		unit  units.Unit
	}{
		{"Ordinary", units.Meters(1.25), units.Millimeter},
		{"Boundary", units.Meters(1e307), units.Foot},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			value, unit := tc.value, tc.unit
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				result, err := value.In(unit)
				benchmarkFloatSink = result
				errBenchmarkSink = err
			}
		})
	}
}

func BenchmarkValueMul(b *testing.B) {
	cases := []struct {
		name        string
		left, right units.Value
	}{
		{"Ordinary", units.Millimeters(2), units.Millimeters(3)},
		{"Boundary", units.Meters(1e297), units.Scalar(1e7)},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			left, right := tc.left, tc.right
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				result, err := left.Mul(right)
				benchmarkValueSink = result
				errBenchmarkSink = err
			}
		})
	}
}

func BenchmarkValueDiv(b *testing.B) {
	cases := []struct {
		name        string
		left, right units.Value
	}{
		{"Ordinary", units.Millimeters(6), units.Millimeters(3)},
		{"Boundary", units.Meters(1e307), units.Scalar(100)},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			left, right := tc.left, tc.right
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				result, err := left.Div(right)
				benchmarkValueSink = result
				errBenchmarkSink = err
			}
		})
	}
}

func BenchmarkValueString(b *testing.B) {
	cases := []struct {
		name  string
		value units.Value
	}{
		{"UnitBearing", units.Meters(12.5)},
		{"Dimensionless", units.Scalar(12.5)},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			value := tc.value
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				benchmarkStringSink = value.String()
			}
		})
	}
}

func BenchmarkBaseUnit(b *testing.B) {
	unnamed := units.Length.Div(units.Mass)
	cases := []struct {
		name string
		kind units.Kind
	}{
		{"Named", units.Length},
		{"Unnamed", unnamed},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			kind := tc.kind
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				unit, ok := units.BaseUnit(kind)
				benchmarkUnitSink = unit
				benchmarkBoolSink = ok
			}
		})
	}
}

func BenchmarkValueAddParallel(b *testing.B) {
	left, right := units.Meters(1.25), units.Millimeters(725)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			result, err := left.Add(right)
			parallelFloatSink.Store(math.Float64bits(result.Mag()))
			parallelResultSink.Store(err != nil)
		}
	})
}

func BenchmarkValueEqualParallel(b *testing.B) {
	left, right := units.Meters(1), units.Millimeters(1000)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			parallelResultSink.Store(left.Equal(right, 0))
		}
	})
}
