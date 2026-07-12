package units_test

import (
	"bytes"
	"encoding/json"
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/lestrrat-3d/units"
	"github.com/stretchr/testify/require"
)

// The application units the text-form sweep registers. They are package-level so the
// registry holds them however the tests are selected, and their symbols are their own.
var (
	fathom = units.Define("fathom", units.Length, 1828.8)
	carat  = units.Define("ct", units.Mass, 0.0002)
	grad   = units.Define("grad", units.Angle, math.Pi/200)
)

// textMags is the magnitude sweep the round trip is asserted over: the numbers a
// caller writes, and the ones a float64 barely holds. Both zeros are in it — a −0
// canonicalises to +0 on construction, and the text form must say so — as are the
// subnormals, the smallest normal, MaxFloat64 and a magnitude whose shortest rendering
// takes all 17 significant digits, at both signs.
func textMags() []float64 {
	mags := []float64{
		0,
		math.Copysign(0, -1),
		1,
		0.1,
		0.30000000000000004, // 0.1 + 0.2: 17 significant digits
		2.5,
		25.4,
		math.Pi,
		7850,
		1e6,
		1e20,  // 'g' writes this in full…
		1e21,  // …and this in exponent form
		1e307, // an ordinary magnitude whose base magnitude overflows
		math.MaxFloat64,
		math.Nextafter(math.MaxFloat64, 0),
		1.7976931348623155e+308,
		2.2250738585072014e-308, // the smallest normal
		math.Nextafter(2.2250738585072014e-308, 0),
		1e-310,                          // a subnormal
		5e-324,                          // the smallest subnormal
		math.SmallestNonzeroFloat64 * 3, // and one with a couple of bits in it
		1e-300,
		1.2345678901234567e-15,
		math.Nextafter(1, 2), // 1 + one ulp: the whole significand
	}

	// Every one of them at both signs: a text form that lost the sign, or that wrote a
	// negative magnitude it could not read back, would be as wrong as one that lost the
	// unit.
	signed := make([]float64, 0, 2*len(mags))
	for _, m := range mags {
		signed = append(signed, m, -m)
	}
	return signed
}

// requireTextRoundTrip asserts that v survives its own text form: the same unit, and
// the same magnitude bit for bit. It is the whole point of the form — "close" is a
// regression in a library where 25.4 mm is exactly 1 in — so it is asserted on
// math.Float64bits (sameValuef), never with ==, which cannot see a negative zero.
func requireTextRoundTrip(t *testing.T, v units.Value) []byte {
	t.Helper()

	text, err := v.MarshalText()
	require.NoError(t, err, "%s has a text form", v)

	var got units.Value
	require.NoError(t, got.UnmarshalText(text), "reading back %q", text)
	sameValuef(t, v, got, "%q round-trips", text)
	return text
}

func TestValueTextForm(t *testing.T) {
	// The form is the magnitude, one space, and the unit's registered symbol — and for
	// a dimensionless value, whose unit has no symbol, the bare magnitude.
	for _, tc := range []struct {
		val  units.Value
		want string
	}{
		{units.Millimeters(10), "10 mm"},
		{units.Millimeters(-2.5), "-2.5 mm"},
		{units.Inches(1), "1 in"},
		{units.SquareMillimeters(2.5), "2.5 mm^2"},
		{units.CubicInches(3), "3 in^3"},
		{units.KilogramsPerCubicMeter(7850), "7850 kg/m^3"},
		{units.KilogramSquareMillimeters(1e-7), "1e-07 kg*mm^2"},
		{units.Degrees(90), "90 deg"},
		{units.Radians(math.Pi), "3.141592653589793 rad"},
		{units.Thous(1), "1 thou"},
		{units.Liters(0.5), "0.5 L"},
		{units.Scalar(1.5), "1.5"},
		{units.Scalar(0), "0"},
		{units.Millimeters(0), "0 mm"},
		{units.Millimeters(math.Copysign(0, -1)), "0 mm"}, // a Value carries no −0
		{units.Millimeters(math.MaxFloat64), "1.7976931348623157e+308 mm"},
		{units.Millimeters(5e-324), "5e-324 mm"},
		{units.Value{}, "0"}, // the zero Value: 0 of One
	} {
		t.Run(tc.want, func(t *testing.T) {
			text, err := tc.val.MarshalText()
			require.NoError(t, err)
			require.Equal(t, tc.want, string(text))

			// Whatever it wrote, it reads back — the same quantity, bit for bit.
			requireTextRoundTrip(t, tc.val)
		})
	}
}

func TestValueTextRoundTrip(t *testing.T) {
	// Every built-in unit, and a few an application defined, over the whole magnitude
	// sweep: the everyday numbers, the subnormals, MaxFloat64 and both zeros.
	for _, u := range append(builtinUnits(), fathom, carat, grad, hugeLength, tinyLength, turn) {
		t.Run(u.String(), func(t *testing.T) {
			for _, m := range textMags() {
				v := units.New(m, u)
				text := requireTextRoundTrip(t, v)

				// The magnitude the text carries is the value's own — no rescale, no
				// base magnitude (which overflows for an ordinary Meters(1e307)), and
				// so nothing for a conversion to round.
				require.Equal(t, strconv.FormatFloat(v.Mag(), 'g', -1, 64),
					strings.TrimSuffix(strings.TrimSuffix(string(text), u.Symbol()), " "),
					"%q carries the magnitude as written", text)
			}
		})
	}
}

func TestValueTextRoundTripOverRandomBits(t *testing.T) {
	// The curated sweep names the magnitudes a reader expects; this one names none.
	// Random bit patterns land wherever float64s live — every binade, the subnormals,
	// the full 17-digit significands — so the claim that strconv's shortest 'g' form
	// reads back as the same float64 is tested rather than trusted, and no band of the
	// range is left out because nobody thought to write it down.
	r := rand.New(rand.NewPCG(0x5eed, 0xf10a75))
	sweep := []units.Unit{units.One, units.Millimeter, units.Inch, units.SquareMeter, units.Gram, units.Degree, tinyLength, hugeLength}

	for i := range 200_000 {
		m := math.Float64frombits(r.Uint64())
		if math.IsInf(m, 0) || math.IsNaN(m) {
			continue // not a quantity; MarshalText refuses it, and TestValueTextRefusesNonFinite asserts that
		}

		v := units.New(m, sweep[i%len(sweep)])
		text, err := v.MarshalText()
		require.NoError(t, err)

		var got units.Value
		require.NoError(t, got.UnmarshalText(text))
		require.Equal(t, v.Unit(), got.Unit(), "%q keeps the unit", text)
		require.Equal(t, math.Float64bits(v.Mag()), math.Float64bits(got.Mag()),
			"%q must read back bit for bit: want %v (bits %#016x), got %v (bits %#016x)",
			text, v.Mag(), math.Float64bits(v.Mag()), got.Mag(), math.Float64bits(got.Mag()))
	}
}

func TestValueTextSymbolResolves(t *testing.T) {
	// No symbol the library emits may be one it cannot read: the text form's symbol is
	// a registry key, and Lookup hands back the very unit that wrote it.
	for _, u := range append(builtinUnits(), fathom, carat, grad) {
		t.Run(u.String(), func(t *testing.T) {
			text, err := units.New(3, u).MarshalText()
			require.NoError(t, err)

			symbol := ""
			if _, after, found := bytes.Cut(text, []byte(" ")); found {
				symbol = string(after)
			}
			require.Equal(t, u.Symbol(), symbol, "the text carries the unit's own symbol")

			got, ok := units.Lookup(symbol)
			require.True(t, ok, "the symbol %q must resolve", symbol)
			require.Equal(t, u, got)
			require.Equal(t, u.Kind(), got.Kind(), "and to the same kind")
		})
	}
}

func TestValueTextDimensionless(t *testing.T) {
	// One's symbol is empty, so a dimensionless value is the bare number. It is not a
	// text with a missing symbol: nothing else parses as one, and a text with a symbol
	// never parses as dimensionless.
	for _, tc := range []struct {
		text string
		mag  float64
	}{
		{"0", 0},
		{"-0", 0}, // read back as +0: a Value carries no negative zero
		{"1", 1},
		{"-1.5", -1.5},
		{"1e-07", 1e-7},
		{"5e-324", 5e-324},
		{"1.7976931348623157e+308", math.MaxFloat64},
	} {
		t.Run(tc.text, func(t *testing.T) {
			var v units.Value
			require.NoError(t, v.UnmarshalText([]byte(tc.text)))
			require.Equal(t, units.One, v.Unit(), "a bare number is dimensionless")
			require.Equal(t, units.Dimensionless, v.Kind())
			sameFloat64f(t, tc.mag, v.Mag(), "%q is %v", tc.text, tc.mag)
		})
	}

	// The zero Value is 0 of One, and it writes and reads as the bare "0".
	text, err := units.Value{}.MarshalText()
	require.NoError(t, err)
	require.Equal(t, "0", string(text))

	var zero units.Value
	require.NoError(t, zero.UnmarshalText(text))
	sameValuef(t, units.Value{}, zero, "the zero Value round-trips")

	// A dimensionless text may not wear a symbol's clothes: an empty symbol after the
	// space is malformed, not One.
	require.ErrorIs(t, new(units.Value).UnmarshalText([]byte("1 ")), units.ErrMalformedText)
}

func TestValueTextRefusesUnnamedKind(t *testing.T) {
	// A value of an unnamed kind carries a synthetic, unregistered symbol ("[L^-1]").
	// Emitting it would write a text nothing can read back, so it is an error — the
	// library's own rule that such a value must not be persisted, enforced at the one
	// place it would be.
	for _, tc := range []struct {
		name string
		val  func() (units.Value, error)
		// symbol is the synthetic one the value carries, and recover is a factor that
		// composes it back into a named kind — which is what the library says to do
		// with such a value instead of persisting it.
		symbol  string
		recover units.Value
		named   units.Kind
	}{
		{
			name:    "inverse length",
			val:     func() (units.Value, error) { return units.Scalar(1).Div(units.Millimeters(4)) },
			symbol:  "[L^-1]",
			recover: units.SquareMillimeters(4),
			named:   units.Length,
		},
		{
			name:    "inverse area",
			val:     func() (units.Value, error) { return units.Scalar(1).Div(units.SquareMillimeters(4)) },
			symbol:  "[L^-2]",
			recover: units.CubicMillimeters(4),
			named:   units.Length,
		},
		{
			name:    "areal density",
			val:     func() (units.Value, error) { return units.Kilograms(1).Div(units.SquareMillimeters(4)) },
			symbol:  "[L^-2*M]",
			recover: units.SquareMillimeters(4),
			named:   units.Mass,
		},
		{
			name:    "length per angle",
			val:     func() (units.Value, error) { return units.Millimeters(1).Div(units.Radians(4)) },
			symbol:  "[L*A^-1]",
			recover: units.Radians(4),
			named:   units.Length,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, err := tc.val()
			require.NoError(t, err)
			require.Equal(t, tc.symbol, v.Unit().Symbol())

			text, err := v.MarshalText()
			require.ErrorIs(t, err, units.ErrUnnamedKind)
			require.Nil(t, text, "nothing is written")

			// And the encoder says so too, rather than writing a symbol it cannot read.
			b, err := json.Marshal(struct {
				V units.Value `json:"v"`
			}{v})
			require.ErrorIs(t, err, units.ErrUnnamedKind, "json.Marshal must not succeed")
			require.Empty(t, string(b))

			// The refusal is not the last word: composed back into a named kind, the
			// same quantity has a text form, and it round-trips.
			named, err := v.Mul(tc.recover)
			require.NoError(t, err)
			require.Equal(t, tc.named, named.Kind(), "composed back into a named kind")
			requireTextRoundTrip(t, named)
		})
	}
}

func TestValueTextRefusesOverflowedKind(t *testing.T) {
	// An overflowed kind is a programming error made visible: its exponents are clamped
	// stand-ins and its symbol is the reserved "[overflow]". It must not reach a
	// document — not as "[overflow]", which resolves to nothing, and not at all.
	v, err := units.New(2, wideLength).Mul(units.New(3, wideLength))
	require.NoError(t, err)
	require.True(t, v.Kind().Overflowed())
	require.Equal(t, "[overflow]", v.Unit().Symbol())

	text, err := v.MarshalText()
	require.ErrorIs(t, err, units.ErrOverflowedKind)
	require.Nil(t, text)

	b, err := json.Marshal(struct {
		V units.Value `json:"v"`
	}{v})
	require.ErrorIs(t, err, units.ErrOverflowedKind)
	require.Empty(t, string(b))

	// The symbol it would have written resolves to nothing, which is why it is refused.
	_, ok := units.Lookup("[overflow]")
	require.False(t, ok)
}

func TestValueTextRefusesNonFinite(t *testing.T) {
	// New, FromBase, Scale and Neg have no error to report, so a Value can be built
	// carrying an infinity or a NaN. It is not a quantity — a persisted "+Inf mm" read
	// back is a length that is not a length — so it has no text form.
	for _, tc := range []struct {
		name string
		val  units.Value
	}{
		{"+Inf mm", units.New(math.Inf(1), units.Millimeter)},
		{"-Inf mm", units.New(math.Inf(-1), units.Millimeter)},
		{"NaN mm", units.New(math.NaN(), units.Millimeter)},
		{"+Inf scalar", units.Scalar(math.Inf(1))},
		{"NaN scalar", units.Scalar(math.NaN())},
		{"scaled past the range", units.Meters(math.MaxFloat64).Scale(2)},
		{"from an infinite base", units.FromBase(math.Inf(1), units.Inch)},
		{"negated infinity", units.New(math.Inf(1), units.Degree).Neg()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text, err := tc.val.MarshalText()
			require.ErrorIs(t, err, units.ErrNotFinite)
			require.Nil(t, text)

			b, err := json.Marshal(struct {
				V units.Value `json:"v"`
			}{tc.val})
			require.ErrorIs(t, err, units.ErrNotFinite)
			require.Empty(t, string(b))
		})
	}

	// The other direction: a literal infinity or NaN in a document is refused too, so
	// nothing that MarshalText will not write can be read.
	for _, text := range []string{"+Inf mm", "-Inf mm", "Inf mm", "NaN mm", "inf", "nan", "NaN", "Infinity mm", "-Infinity"} {
		t.Run("reading "+text, func(t *testing.T) {
			var v units.Value
			require.ErrorIs(t, v.UnmarshalText([]byte(text)), units.ErrNotFinite)
			sameValuef(t, units.Value{}, v, "a rejected text leaves the value alone")
		})
	}
}

func TestValueTextRejectsMalformed(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want error
	}{
		{"empty", "", units.ErrMalformedText},
		{"a space", " ", units.ErrMalformedText},
		{"no magnitude", " mm", units.ErrMalformedText},
		{"a symbol alone", "mm", units.ErrMalformedText},
		{"no space", "10mm", units.ErrMalformedText},
		{"a trailing space", "10 ", units.ErrMalformedText},
		{"a doubled space", "10  mm", units.ErrMalformedText},
		{"a leading space", " 10 mm", units.ErrMalformedText},
		{"an extra token", "10 mm extra", units.ErrMalformedText},
		{"two magnitudes", "10 20 mm", units.ErrMalformedText},
		{"a tab", "10\tmm", units.ErrMalformedText},
		{"a newline", "10 mm\n", units.ErrUnknownUnit}, // the symbol is "mm\n", not "mm"
		{"words", "ten mm", units.ErrMalformedText},
		{"a magnitude with a unit stuck on", "10.5mm mm", units.ErrMalformedText},
		{"past the float64 range", "1e999 mm", units.ErrNotFinite},
		{"past the float64 range, negative", "-1e999 mm", units.ErrNotFinite},
		{"an unregistered symbol", "10 xyzzy", units.ErrUnknownUnit},
		{"a kind's prose name", "10 length", units.ErrUnknownUnit},
		{"a synthetic symbol", "10 [L^-1]", units.ErrUnknownUnit},
		{"the overflow symbol", "10 [overflow]", units.ErrUnknownUnit},
		{"a bracket", "10 [", units.ErrUnknownUnit},
		{"a case-folded symbol", "10 MM", units.ErrUnknownUnit},
		{"a Unicode symbol", "10 mm²", units.ErrUnknownUnit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := units.Millimeters(7) // something to leave alone
			require.ErrorIs(t, v.UnmarshalText([]byte(tc.text)), tc.want)
			sameValuef(t, units.Millimeters(7), v, "a rejected text leaves the value alone")
		})
	}
}

func TestValueTextNeverCarriesNegativeZero(t *testing.T) {
	// A Value never carries a −0, so no text form writes one and no text reads one back.
	for _, u := range append(builtinUnits(), fathom, carat, grad) {
		t.Run(u.String(), func(t *testing.T) {
			for _, v := range []units.Value{
				units.New(math.Copysign(0, -1), u),
				units.New(0, u).Neg(),
				units.New(-1, u).Scale(0),
			} {
				sameFloat64f(t, 0, v.Mag(), "a Value carries no −0")

				text, err := v.MarshalText()
				require.NoError(t, err)
				require.NotContains(t, string(text), "-0", "%q must not write a negative zero", text)

				var got units.Value
				require.NoError(t, got.UnmarshalText(text))
				sameValuef(t, v, got, "%q round-trips", text)
			}

			// A "-0" that arrives from somewhere else is read as +0, bit for bit.
			text := "-0"
			if u.Symbol() != "" {
				text = "-0 " + u.Symbol()
			}
			var got units.Value
			require.NoError(t, got.UnmarshalText([]byte(text)))
			require.Equal(t, u, got.Unit())
			sameFloat64f(t, 0, got.Mag(), "%q reads back as +0", text)
		})
	}
}

// step is the consumer's shape: a document that records a quantity per field. Without
// a text form a Value's unexported fields encode to {} — the quantity gone, and a nil
// error to say so.
type step struct {
	Distance units.Value `json:"distance"`
	Angle    units.Value `json:"angle"`
	Ratio    units.Value `json:"ratio"`
}

func TestValueTextJSON(t *testing.T) {
	want := step{
		Distance: units.Millimeters(10),
		Angle:    units.Degrees(45),
		Ratio:    units.Scalar(0.5),
	}

	b, err := json.Marshal(want)
	require.NoError(t, err)
	require.JSONEq(t, `{"distance":"10 mm","angle":"45 deg","ratio":"0.5"}`, string(b))
	require.NotContains(t, string(b), "{}", "the quantity must not vanish")

	var got step
	require.NoError(t, json.Unmarshal(b, &got))
	sameValuef(t, want.Distance, got.Distance, "distance")
	sameValuef(t, want.Angle, got.Angle, "angle")
	sameValuef(t, want.Ratio, got.Ratio, "ratio")

	// The unit survives the document, not just the number: what comes back is the unit
	// that was written, not the kind's base unit.
	require.Equal(t, units.Millimeter, got.Distance.Unit())
	require.Equal(t, units.Degree, got.Angle.Unit())
	require.Equal(t, units.One, got.Ratio.Unit())

	// The whole magnitude sweep through a document, in every built-in unit, bit for bit.
	for _, u := range append(builtinUnits(), fathom, carat, grad, hugeLength, tinyLength, turn) {
		t.Run(u.String(), func(t *testing.T) {
			for _, m := range textMags() {
				v := units.New(m, u)

				doc, err := json.Marshal(step{Distance: v})
				require.NoError(t, err)

				var back step
				require.NoError(t, json.Unmarshal(doc, &back))
				sameValuef(t, v, back.Distance, "%s through json", doc)
			}
		})
	}
}

func TestValueTextJSONRejectsBadDocuments(t *testing.T) {
	// A document that names a unit the process does not know is an error, never a
	// magnitude with the unit dropped or a base unit guessed at.
	for _, tc := range []struct {
		doc  string
		want error
	}{
		{`{"distance":"10 xyzzy"}`, units.ErrUnknownUnit},
		{`{"distance":"10 [L^-1]"}`, units.ErrUnknownUnit},
		{`{"distance":"+Inf mm"}`, units.ErrNotFinite},
		{`{"distance":"NaN mm"}`, units.ErrNotFinite},
		{`{"distance":"10 mm extra"}`, units.ErrMalformedText},
		{`{"distance":""}`, units.ErrMalformedText},
		{`{"distance":"mm"}`, units.ErrMalformedText},
	} {
		t.Run(tc.doc, func(t *testing.T) {
			var got step
			require.ErrorIs(t, json.Unmarshal([]byte(tc.doc), &got), tc.want)
		})
	}
}

func TestValueTextDefinedUnits(t *testing.T) {
	// An application's own units are ordinary units: Define registers the symbol, so the
	// text form writes it and Lookup reads it back.
	for _, tc := range []struct {
		val  units.Value
		want string
	}{
		{units.New(1, fathom), "1 fathom"},
		{units.New(2.5, carat), "2.5 ct"},
		{units.New(100, grad), "100 grad"},
		{units.New(1e300, tinyLength), "1e+300 Ltiny"},
		{units.New(math.MaxFloat64, hugeLength), "1.7976931348623157e+308 Lhuge"},
		{units.New(0.5, turn), "0.5 turn"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			text, err := tc.val.MarshalText()
			require.NoError(t, err)
			require.Equal(t, tc.want, string(text))
			requireTextRoundTrip(t, tc.val)
		})
	}

	// A unit defined after a document was written resolves once its Define has run —
	// and not before, which is the honest answer for a symbol the process cannot name.
	var v units.Value
	require.ErrorIs(t, v.UnmarshalText([]byte("3 furlong")), units.ErrUnknownUnit)

	furlong := units.Define("furlong", units.Length, 201168)
	require.NoError(t, v.UnmarshalText([]byte("3 furlong")))
	require.Equal(t, furlong, v.Unit())
	sameFloat64f(t, 3, v.Mag(), "3 furlong")
	sameFloat64f(t, 603504, v.Base(), "3 furlong in mm")
}

func TestDefineRejectsUnreadableSymbol(t *testing.T) {
	// The text form is "<magnitude> <symbol>", separated by a space, so a symbol carrying
	// whitespace is one MarshalText could write and UnmarshalText could never read. The
	// grammar is enforced where a symbol enters the registry: Define panics, and registers
	// nothing, so a symbol Lookup resolves is a symbol the parser reads back.
	for _, tc := range []struct{ name, symbol string }{
		{"a space", "probe space"},
		{"a leading space", " probe"},
		{"a trailing space", "probe "},
		{"only a space", " "},
		{"only whitespace", " \t\n"},
		{"a tab", "zz\tunit"},
		{"a newline", "zz\nunit"},
		{"a carriage return", "zz\runit"},
		{"a vertical tab", "zz\vunit"},
		{"a form feed", "zz\funit"},
		{"a next line (U+0085)", "zz\u0085unit"},
		{"a non-breaking space (U+00A0)", "zz\u00a0unit"},
		{"an ogham space mark (U+1680)", "zz\u1680unit"},
		{"a line separator (U+2028)", "zz\u2028unit"},
		{"an em space (U+2003)", "zz\u2003unit"},
		{"an ideographic space (U+3000)", "zz\u3000unit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.True(t, strings.ContainsFunc(tc.symbol, unicode.IsSpace), "the premise: it carries whitespace")

			require.Panics(t, func() { units.Define(tc.symbol, units.Length, 7) },
				"a symbol the text parser cannot read must not be registrable")
			_, ok := units.Lookup(tc.symbol)
			require.False(t, ok, "a rejected symbol must not be registered")
		})
	}

	// The reproducer. Were "probe space" registrable, MarshalText would write "3 probe
	// space" with a nil error and UnmarshalText could not read it back: the parser cuts at
	// the first space and what follows is two tokens, not a symbol. It is unregistrable, so
	// no value carries it — and the text nothing wrote is malformed, as it always was.
	require.ErrorIs(t, new(units.Value).UnmarshalText([]byte("3 probe space")), units.ErrMalformedText)

	// The empty symbol is One's, and it stays One's: Define collides with it, and the
	// dimensionless bare-number form still writes and reads.
	require.Panics(t, func() { units.Define("", units.Length, 7) }, "the empty symbol is One's")
	one, ok := units.Lookup("")
	require.True(t, ok, "One's empty symbol is registered")
	require.Equal(t, units.One, one)
	requireTextRoundTrip(t, units.Scalar(1.5))
	requireTextRoundTrip(t, units.Scalar(0))
}

// hostileSymbolRunes are what a generated symbol is built from: the characters real
// symbols are made of (letters, digits, carets, slashes, asterisks), the brackets of the
// reserved namespace, the punctuation an application might reach for, Unicode, and every
// class of whitespace the text grammar could trip over.
var hostileSymbolRunes = []rune("aBcXyZ019^*/+-_.:,;%'\"()[]{}<>#&|\\?!$=@`µΩ°²³πÅ日\t\n\r\v\f \u0085\u00a0\u1680\u2003\u2028\u3000")

// hostileSymbols generates n plausible-but-hostile symbols. It names no expectation:
// Define decides which of them are registrable, and the property below is asserted over
// whatever it accepts — which is the assertion a suite of hand-picked *safe* symbols
// cannot make.
//
// Each carries a "~" and its index, so a generated symbol collides with nothing else the
// suite registers, or asserts is unregistered; the hostility is in everything around it.
func hostileSymbols(n int) []string {
	r := rand.New(rand.NewPCG(0xba5e, 0x5717b0))
	pick := func(length int) string {
		var b strings.Builder
		for range length {
			b.WriteRune(hostileSymbolRunes[r.IntN(len(hostileSymbolRunes))])
		}
		return b.String()
	}

	seen := map[string]struct{}{}
	symbols := make([]string, 0, n)
	for i := range n {
		length := 8
		if i%16 == 0 {
			length = 64 // a long symbol, too
		}

		// The marker sits at a random place in the junk, so a hostile rune — a space, a
		// bracket — lands at the front and at the back as often as it lands in the middle.
		head := r.IntN(length + 1)
		symbol := pick(head) + "~" + strconv.Itoa(i) + pick(length-head)
		if _, dup := seen[symbol]; dup {
			continue
		}
		seen[symbol] = struct{}{}
		symbols = append(symbols, symbol)
	}
	return symbols
}

// tryDefine registers symbol, reporting whether Define accepted it. Define panics on a
// symbol it refuses, and the property is about what it accepts, so the panic is caught
// rather than provoked.
func tryDefine(symbol string, kind units.Kind, factor float64) (units.Unit, bool) {
	var u units.Unit
	ok := false
	func() {
		defer func() { _ = recover() }()
		u = units.Define(symbol, kind, factor)
		ok = true
	}()
	return u, ok
}

func TestValueTextRoundTripOverHostileSymbols(t *testing.T) {
	// The property, not the instance: for every symbol Define accepts, what MarshalText
	// writes UnmarshalText reads back — the same unit, and the same magnitude bit for bit.
	// The two grammars are one grammar, and Define is where they are made to agree, so the
	// symbols here are the ones a hand-written table would not have thought of.
	accepted, rejected := 0, 0
	for _, symbol := range hostileSymbols(4000) {
		if _, taken := units.Lookup(symbol); taken {
			continue // already a unit; Define would panic on the duplicate, which is not the property
		}

		u, ok := tryDefine(symbol, units.Length, 3)
		if !ok {
			rejected++
			// A refusal registers nothing, and only an unreadable symbol (whitespace) or a
			// reserved one (the "[" namespace) is refused — the factor and the kind are fine.
			_, found := units.Lookup(symbol)
			require.False(t, found, "a rejected symbol %q must not be registered", symbol)
			require.True(t,
				strings.ContainsFunc(symbol, unicode.IsSpace) || strings.HasPrefix(symbol, "["),
				"%q is neither unreadable nor reserved, so Define must accept it", symbol)
			continue
		}

		accepted++
		require.Equal(t, symbol, u.Symbol())
		require.False(t, strings.ContainsFunc(symbol, unicode.IsSpace), "an accepted symbol carries no whitespace")

		for _, m := range []float64{0, math.Copysign(0, -1), 1, -2.5, math.Pi, 1e307, math.MaxFloat64, 5e-324} {
			v := units.New(m, u)

			text, err := v.MarshalText()
			require.NoError(t, err)
			require.Equal(t, strconv.FormatFloat(v.Mag(), 'g', -1, 64)+" "+symbol, string(text),
				"the text is the magnitude, a space, and the symbol")

			// The symbol survives the separator whole: the parser's cut lands on the space
			// MarshalText wrote and nowhere else.
			_, after, found := strings.Cut(string(text), " ")
			require.True(t, found)
			require.Equal(t, symbol, after, "%q carries the symbol as written", text)

			requireTextRoundTrip(t, v)
		}
	}

	// The generator must actually exercise both sides of Define's judgment.
	require.Greater(t, accepted, 100, "the sweep must register real units")
	require.Greater(t, rejected, 100, "…and must offer Define symbols it has to refuse")
}

func TestValueTextPreservesTheQuantity(t *testing.T) {
	// The text form is a document boundary, and the library's exactness claims must
	// survive it: what is read back is the same quantity, in the same unit, and it
	// converts and compares exactly as the value that was written does.
	var mm units.Value
	require.NoError(t, mm.UnmarshalText([]byte("25.4 mm")))
	in, err := mm.In(units.Inch)
	require.NoError(t, err)
	sameFloat64f(t, 1, in, "25.4 mm read from a document is exactly 1 in")

	var big units.Value
	require.NoError(t, big.UnmarshalText([]byte("1e+307 m")))
	sameValuef(t, units.Meters(1e307), big, "an ordinary magnitude whose base magnitude overflows")
	q, err := big.Div(units.Meters(1e307))
	require.NoError(t, err)
	sameFloat64f(t, 1, q.Mag(), "a value read back divides by itself to exactly 1")
	require.True(t, big.Equal(units.Meters(1e307), 0), "and equals itself at tol 0")

	// A subnormal survives too: the text form rounds nothing.
	var sub units.Value
	require.NoError(t, sub.UnmarshalText([]byte("5e-324 mm")))
	sameFloat64f(t, math.SmallestNonzeroFloat64, sub.Mag(), "the smallest subnormal")

	// A literal below the smallest subnormal is the nearest float64 to it — +0, and a
	// positive one — which is the rounding the arithmetic makes at that end of the
	// range. MarshalText never writes such a text; this is what reading one does.
	var tiny units.Value
	require.NoError(t, tiny.UnmarshalText([]byte("-1e-999 mm")))
	require.Equal(t, units.Millimeter, tiny.Unit())
	sameFloat64f(t, 0, tiny.Mag(), "a literal below the last subnormal reads as +0")

	// The other end is not a rounding: a literal past the last float64 is no quantity.
	require.ErrorIs(t, new(units.Value).UnmarshalText([]byte("1e999 mm")), units.ErrNotFinite)

	// And an unmarshal replaces the whole value — the previous quantity is gone, unit
	// and kind included.
	v := units.Kilograms(3)
	require.NoError(t, v.UnmarshalText([]byte("45 deg")))
	sameValuef(t, units.Degrees(45), v, "the value is replaced whole")
	require.Equal(t, units.Angle, v.Kind())
}
