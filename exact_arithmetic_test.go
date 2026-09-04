package units_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/lestrrat-3d/units"
	"github.com/stretchr/testify/require"
)

var (
	exactArithmeticValue units.Value
	exactArithmeticEqual bool
)

func TestExactArithmeticRandomizedOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(0x5eed2))
	lengthUnits := []units.Unit{
		units.Millimeter,
		units.Centimeter,
		units.Meter,
		units.Inch,
		hugeLength,
		tinyLength,
	}

	for i := range 512 {
		a := units.New(randomNormalFloat(rng), lengthUnits[rng.Intn(len(lengthUnits))])
		b := units.New(randomNormalFloat(rng), lengthUnits[rng.Intn(len(lengthUnits))])

		for _, operation := range []struct {
			name string
			sign float64
			do   func(units.Value, units.Value) (units.Value, error)
		}{
			{"add", 1, units.Value.Add},
			{"sub", -1, units.Value.Sub},
		} {
			want := combineRat(t, a, b, operation.sign)
			got, err := operation.do(a, b)
			if overflows(want) {
				require.ErrorIs(t, err, units.ErrNotFinite, "%d: %s", i, operation.name)
				continue
			}
			require.NoError(t, err, "%d: %s", i, operation.name)
			sameFloat64f(t, nearest(want), got.Mag(), "%d: %s", i, operation.name)
		}

		for _, tolerance := range []float64{0, 5e-324, 1e-12, 1e-9, 1e300} {
			want := equalOracle(t, a, b, tolerance)
			require.Equal(t, want, a.Equal(b, tolerance), "%d: Equal", i)
			require.Equal(t, want, b.Equal(a, tolerance), "%d: Equal symmetry", i)
		}
	}
}

func TestExactArithmeticBoundaryOracle(t *testing.T) {
	cases := [][2]units.Value{
		{units.New(math.MaxFloat64, tinyLength), units.New(-math.MaxFloat64, tinyLength)},
		{units.New(math.MaxFloat64, hugeLength), units.New(math.MaxFloat64, tinyLength)},
		{units.New(5e-324, tinyLength), units.Millimeters(0)},
		{units.New(5e-324, hugeLength), units.New(-5e-324, hugeLength)},
		{units.New(math.MaxFloat64, units.Meter), units.New(-math.MaxFloat64, units.Inch)},
	}

	for i, pair := range cases {
		for _, operation := range []struct {
			name string
			sign float64
			do   func(units.Value, units.Value) (units.Value, error)
		}{
			{"add", 1, units.Value.Add},
			{"sub", -1, units.Value.Sub},
		} {
			want := combineRat(t, pair[0], pair[1], operation.sign)
			got, err := operation.do(pair[0], pair[1])
			if overflows(want) {
				require.ErrorIs(t, err, units.ErrNotFinite, "%d: %s", i, operation.name)
				continue
			}
			require.NoError(t, err, "%d: %s", i, operation.name)
			sameFloat64f(t, nearest(want), got.Mag(), "%d: %s", i, operation.name)
		}
	}
}

func TestExactArithmeticAllocations(t *testing.T) {
	a, b := units.Millimeters(25.4), units.Inches(1)
	allocs := testing.AllocsPerRun(1000, func() {
		exactArithmeticValue, _ = a.Add(b)
	})
	require.Zero(t, allocs, "cross-unit Add")

	allocs = testing.AllocsPerRun(1000, func() {
		exactArithmeticValue, _ = a.Sub(b)
	})
	require.Zero(t, allocs, "cross-unit Sub")

	allocs = testing.AllocsPerRun(1000, func() {
		exactArithmeticEqual = a.Equal(b, 1e-9)
	})
	require.Zero(t, allocs, "cross-unit Equal")
}

func randomNormalFloat(rng *rand.Rand) float64 {
	bitsValue := rng.Uint64() &^ (uint64(0x7ff) << 52)
	bitsValue |= uint64(rng.Intn(0x7fe)+1) << 52
	return math.Float64frombits(bitsValue)
}
