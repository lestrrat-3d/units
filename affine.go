package units

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// ErrNotAffine is returned when a conversion names a unit of the wrong sort: an
// [AffineValue] asked for an [AffineUnit] that does not exist, or a [Value] asked
// to become an affine quantity through a unit that is not affine. The two sorts
// of unit are separate types, so this is reachable only through the zero
// AffineUnit, which names no unit at all.
var ErrNotAffine = errors.New("units: not an affine unit")

// AffineUnit is a unit whose zero is not its kind's base zero, so that reaching
// the base unit is a scale and a shift: base == magnitude × factor + offset. The
// degree Celsius is one — 0 °C is 273.15 K — and so is the degree Fahrenheit.
//
// It is a separate type from [Unit], and that is the whole design. A quantity on
// such a scale cannot be scaled, negated, added or multiplied in any way that has
// a single defensible answer: 20 °C doubled is not 40 °C, because doubling states
// a ratio and 0 °C is not the absence of temperature; 20 °C plus 5 °C is not
// 25 °C, because two absolute temperatures do not add at all. Rather than offer
// those operations and refuse them at run time, the library gives affine
// quantities their own type, [AffineValue], which simply does not have them.
//
// Everything an affine quantity can honestly do it does: it converts to another
// affine unit, converts to a ratio [Value] on the same kind ([AffineValue.ToRatio],
// which is how you reach [Kelvin] and the arithmetic), compares, prints and
// persists.
//
// Symbols live in one namespace shared with [Unit], so no symbol names both a
// ratio and an affine unit. Register your own with [DefineAffine]; resolve one by
// symbol with [LookupAffine].
type AffineUnit struct {
	symbol string
	kind   Kind
	factor float64 // magnitude * factor + offset == magnitude in the kind's base unit
	offset float64 // what makes it affine; never zero for a registered AffineUnit
}

// Celsius and Fahrenheit are the built-in affine units. Both measure
// [Temperature], whose base unit is the ratio unit [Kelvin].
var (
	Celsius    = defineAffine("degC", Temperature, 1, 273.15)
	Fahrenheit = defineAffine("degF", Temperature, 5.0/9.0, 459.67*5.0/9.0)
)

// Symbol returns the unit's short symbol ("degC").
func (u AffineUnit) Symbol() string { return u.symbol }

// Kind returns the kind of quantity the unit measures.
func (u AffineUnit) Kind() Kind { return u.kind }

// Factor returns the multiplier applied before the offset on the way to the
// kind's base unit.
func (u AffineUnit) Factor() float64 { return u.factor }

// Offset returns the constant added after scaling to reach the kind's base unit:
// magnitude × factor + offset. It is never zero for a registered unit, because a
// zero offset is a ratio unit and belongs to [Unit].
func (u AffineUnit) Offset() float64 { return u.offset }

// Valid reports whether u names a unit at all. The zero AffineUnit has a zero
// factor, which is not a usable multiplier; unlike the zero [Unit], which reads
// as [One], it has no sensible reading, because there is no natural affine unit
// to fall back to. Every method that could be asked to convert through one
// reports [ErrNotAffine] rather than invent a unit.
func (u AffineUnit) Valid() bool { return u.factor != 0 }

// String returns the unit's symbol, or "(invalid)" for the zero AffineUnit.
func (u AffineUnit) String() string {
	if !u.Valid() {
		return "(invalid)"
	}
	return u.symbol
}

// defineAffine registers an affine unit. The offset must be finite and nonzero:
// a zero offset is a ratio unit, and [Define] is where those are built.
func defineAffine(symbol string, kind Kind, factor, offset float64) AffineUnit {
	checkFactor(factor)
	if math.IsInf(offset, 0) || math.IsNaN(offset) {
		panic("units: unit offset must be finite: " + strconv.FormatFloat(offset, 'g', -1, 64))
	}
	if offset == 0 {
		panic("units: an affine unit needs a nonzero offset; use Define for a ratio unit: " + strconv.Quote(symbol))
	}

	registryMu.Lock()
	defer registryMu.Unlock()

	claimSymbol(symbol, kind)
	u := AffineUnit{symbol: symbol, kind: kind, factor: factor, offset: offset}
	affineRegistry[symbol] = u
	return u
}

// DefineAffine registers and returns a new affine unit measuring kind, whose
// magnitudes reach the kind's base unit by magnitude × factorToBase +
// offsetToBase. Every rule [Define] documents — the symbol grammar, the reserved
// "[" namespace, uniqueness, the positive finite factor, the non-overflowed kind,
// and registering nothing when it panics — applies here unchanged, and the symbol
// namespace is shared, so a symbol already registered as a ratio unit is refused.
//
// offsetToBase must be finite AND nonzero: DefineAffine panics on an infinite or
// NaN offset, for the reason Define panics on an unusable factor, and on a zero
// offset, because a unit that shares its kind's base zero is a ratio unit and
// belongs to [Define]. There is no unit that is both.
//
// # Use it only where the zero really does move
//
// An affine unit buys a shifted zero and gives up arithmetic: an [AffineValue]
// converts, compares, prints and persists, and has no Add, Sub, Mul, Div, Scale
// or Neg at all. Reach for it only where a unit's zero genuinely differs from its
// kind's base zero — the degree Celsius against the kelvin — and never as a way to
// fold a datum, a bias or a calibration constant into a unit. Those are
// quantities, and they belong in the arithmetic an affine unit does not have.
//
// DefineAffine is safe to call from multiple goroutines, concurrently with
// [Define], [Lookup], [LookupAffine] and [BaseUnit].
func DefineAffine(symbol string, kind Kind, factorToBase, offsetToBase float64) AffineUnit {
	return defineAffine(symbol, kind, factorToBase, offsetToBase)
}

// LookupAffine returns the affine unit previously registered for symbol. It is
// intended for deserialization; prefer the typed [AffineUnit] constants in normal
// code. It is safe to call concurrently with [DefineAffine].
//
// It resolves affine units only; [Lookup] resolves the ratio ones. A symbol names
// at most one of the two.
func LookupAffine(symbol string) (AffineUnit, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	u, ok := affineRegistry[symbol]
	return u, ok
}

// AffineValue is a magnitude paired with the [AffineUnit] it is expressed in: a
// temperature on the Celsius or Fahrenheit scale, say.
//
// It deliberately has no arithmetic. There is no Add, Sub, Mul, Div, Scale or Neg,
// because none of them has an answer that is right for every caller — see
// [AffineUnit]. What it has is conversion ([AffineValue.Convert],
// [AffineValue.In], [AffineValue.ToRatio]), comparison ([AffineValue.Equal]),
// printing and the text form. To compute with a temperature, convert it to the
// kind's base unit first:
//
//	k, err := units.DegreesCelsius(20).ToRatio(units.Kelvin)  // 293.15 K, a Value
//	doubled, err := k.Mul(units.Scalar(2))                    // 586.3 K
//
// The zero AffineValue is not a quantity: unlike the zero [Value], which reads as
// 0 of [One], there is no natural affine unit to fall back on. Its methods report
// [ErrNotAffine] rather than invent one.
//
// Like [Value] it never carries a negative zero, and its text form is the same
// "<magnitude> <symbol>" shape — "210 degC" — read back by
// [AffineValue.UnmarshalText] through [LookupAffine].
type AffineValue struct {
	mag  float64
	unit AffineUnit
}

// NewAffine returns an AffineValue of mag in unit u. Like [New] it reports no
// error and so does not check mag; a mag of −0 is a +0.
func NewAffine(mag float64, u AffineUnit) AffineValue {
	return AffineValue{mag: canonicalZero(mag), unit: u}
}

// DegreesCelsius and DegreesFahrenheit build an [AffineValue] in each of the
// built-in affine units.
func DegreesCelsius(x float64) AffineValue    { return NewAffine(x, Celsius) }
func DegreesFahrenheit(x float64) AffineValue { return NewAffine(x, Fahrenheit) }

// Mag returns the magnitude in the value's own unit. It is never a −0.
func (a AffineValue) Mag() float64 { return a.mag }

// Unit returns the value's unit.
func (a AffineValue) Unit() AffineUnit { return a.unit }

// Kind returns the kind of quantity the value measures.
func (a AffineValue) Kind() Kind { return a.unit.kind }

// Base returns the magnitude expressed in the kind's base unit — kelvin for a
// temperature — as [Value.Base] does for a ratio quantity. The shift is applied
// after the scale, so DegreesCelsius(0).Base() is 273.15.
//
// It is an accessor, not an operation: it may honestly report an infinity for a
// magnitude whose conversion overflows. The zero AffineValue names no unit and
// has no base magnitude; it reports a NaN.
func (a AffineValue) Base() float64 {
	if !a.unit.Valid() {
		return math.NaN()
	}
	return canonicalZero(a.mag*a.unit.factor + a.unit.offset)
}

// In returns the magnitude expressed in the ratio unit u — the whole point being
// that [Kelvin] is a ratio unit, so this is how a temperature becomes a number
// you can compute with. It reports [ErrIncompatible] if u measures a different
// kind, and [ErrNotFinite] if the result is not finite.
func (a AffineValue) In(u Unit) (float64, error) {
	if !a.unit.Valid() {
		return 0, fmt.Errorf("%w: the zero AffineValue names no unit", ErrNotAffine)
	}
	if a.unit.kind != u.kind {
		return 0, fmt.Errorf("%w: cannot express %s in %s", ErrIncompatible, a.unit.kind, u.kind)
	}
	m := affineConvert(a.mag, a.unit.factor, a.unit.offset, u.Factor(), 0)
	if !isFinite(m) {
		return 0, fmt.Errorf("%w: cannot express %s in %s", ErrNotFinite, a, u)
	}
	return m, nil
}

// InAffine returns the magnitude expressed in another affine unit, under the same
// rules as [AffineValue.In].
func (a AffineValue) InAffine(u AffineUnit) (float64, error) {
	if !a.unit.Valid() || !u.Valid() {
		return 0, fmt.Errorf("%w: the zero AffineUnit names no unit", ErrNotAffine)
	}
	if a.unit.kind != u.kind {
		return 0, fmt.Errorf("%w: cannot express %s in %s", ErrIncompatible, a.unit.kind, u.kind)
	}
	m := affineConvert(a.mag, a.unit.factor, a.unit.offset, u.factor, u.offset)
	if !isFinite(m) {
		return 0, fmt.Errorf("%w: cannot express %s in %s", ErrNotFinite, a, u)
	}
	return m, nil
}

// Convert returns the same quantity carried in the affine unit u, under the same
// rules as [AffineValue.InAffine].
func (a AffineValue) Convert(u AffineUnit) (AffineValue, error) {
	m, err := a.InAffine(u)
	if err != nil {
		return AffineValue{}, err
	}
	return AffineValue{m, u}, nil
}

// ToRatio returns the same quantity as a ratio [Value] carried in the ratio unit
// u, which is the crossing between the two types: a temperature on an affine
// scale becomes one on a ratio scale, and gains arithmetic by doing so.
//
//	k, err := units.DegreesCelsius(20).ToRatio(units.Kelvin)  // 293.15 K
//
// It reports [ErrIncompatible] for a u of another kind and [ErrNotFinite] for a
// result that is not finite.
func (a AffineValue) ToRatio(u Unit) (Value, error) {
	m, err := a.In(u)
	if err != nil {
		return Value{}, err
	}
	return Value{m, u.normalize()}, nil
}

// Equal reports whether a and o are the same quantity to within tol of the kind's
// base unit, reading the same rule as [Value.Equal]: the comparison is on the true
// difference in base units, computed in exact rationals where float64 cannot hold
// it. Values of different kinds are never equal, and a non-finite magnitude is not
// a quantity.
func (a AffineValue) Equal(o AffineValue, tol float64) bool {
	if a.unit.kind != o.unit.kind || !a.unit.Valid() || !o.unit.Valid() {
		return false
	}
	if !exactly(a.mag, o.mag) {
		return sameInfinity(a.mag, o.mag) && tol >= 0
	}
	// Two operands in the very same unit differ by exactly their magnitudes, and a
	// factor is positive and finite, so no arithmetic stands between them and the
	// answer.
	if a.unit == o.unit {
		if a.mag == o.mag {
			return tol >= 0
		}
		if tol == 0 {
			return false
		}
	}
	if !exactly(tol) {
		return math.IsInf(tol, 1)
	}
	if tol < 0 {
		return false
	}
	d := new(big.Rat).Sub(a.baseRat(), o.baseRat())
	return d.Abs(d).Cmp(new(big.Rat).SetFloat64(tol)) <= 0
}

// EqualValue reports whether a and the ratio quantity v are the same quantity to
// within tol of the kind's base unit. It is [AffineValue.Equal] across the two
// types, so DegreesCelsius(0) and Kelvins(273.15) compare equal.
func (a AffineValue) EqualValue(v Value, tol float64) bool {
	if !a.unit.Valid() || a.unit.kind != v.unit.kind {
		return false
	}
	if !exactly(a.mag, v.mag) {
		return sameInfinity(a.mag, v.mag) && tol >= 0
	}
	if !exactly(tol) {
		return math.IsInf(tol, 1)
	}
	if tol < 0 {
		return false
	}
	vu := v.Unit()
	d := new(big.Rat).Sub(a.baseRat(), baseRat(v.mag, vu.factor))
	return d.Abs(d).Cmp(new(big.Rat).SetFloat64(tol)) <= 0
}

// baseRat returns a's base magnitude as an exact rational: mag × factor + offset,
// which — unlike the float64 [AffineValue.Base] — can neither overflow nor
// underflow.
func (a AffineValue) baseRat() *big.Rat {
	r := baseRat(a.mag, a.unit.factor)
	return r.Add(r, new(big.Rat).SetFloat64(a.unit.offset))
}

// String renders the value as "<magnitude> <symbol>".
func (a AffineValue) String() string {
	var buf [64]byte
	n := strconv.AppendFloat(buf[:0], a.mag, 'g', -1, 64)
	n = append(n, ' ')
	n = append(n, a.unit.String()...)
	return string(n)
}

// MarshalText implements [encoding.TextMarshaler], rendering the value as
// "<magnitude> <symbol>" — "210 degC". It reads the same rules as
// [Value.MarshalText]: it never writes a text [AffineValue.UnmarshalText] cannot
// read back, so an unregistered unit (the zero AffineUnit among them) is
// [ErrNotAffine] and a non-finite magnitude is [ErrNotFinite].
func (a AffineValue) MarshalText() ([]byte, error) {
	if r, ok := LookupAffine(a.unit.symbol); !ok || r != a.unit {
		return nil, fmt.Errorf("%w: %s carries the unregistered symbol %q", ErrNotAffine, a, a.unit.symbol)
	}
	if !isFinite(a.mag) {
		return nil, fmt.Errorf("%w: cannot marshal %s", ErrNotFinite, a)
	}
	return []byte(strconv.FormatFloat(a.mag, 'g', -1, 64) + " " + a.unit.symbol), nil
}

// UnmarshalText implements [encoding.TextUnmarshaler], reading what
// [AffineValue.MarshalText] writes: a magnitude, one ASCII space, and a registered
// affine unit symbol.
//
// It resolves the symbol through [LookupAffine], so a ratio symbol such as "K" is
// [ErrUnknownUnit] here even though it is a perfectly good [Unit] — a kelvin is a
// ratio quantity and belongs in a [Value]. That is the same wall from the other
// side: neither type can be made to hold the other's quantity by way of a
// document.
func (a *AffineValue) UnmarshalText(text []byte) error {
	s := string(text)
	before, after, found := strings.Cut(s, " ")
	if !found || after == "" || strings.Contains(after, " ") {
		return fmt.Errorf("%w: %q is not a magnitude and a symbol", ErrMalformedText, s)
	}

	mag, err := strconv.ParseFloat(before, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return fmt.Errorf("%w: %q is not a magnitude: %w", ErrMalformedText, before, err)
	}
	if !isFinite(mag) {
		return fmt.Errorf("%w: %q is not a quantity", ErrNotFinite, before)
	}

	u, ok := LookupAffine(after)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownUnit, after)
	}
	*a = NewAffine(mag, u)
	return nil
}

// ToAffine returns the same quantity as an [AffineValue] carried in the affine
// unit u: the crossing back from a ratio quantity to an affine scale.
//
//	c, err := units.Kelvins(293.15).ToAffine(units.Celsius)  // 20 degC
//
// It reports [ErrNotAffine] for the zero [AffineUnit], [ErrIncompatible] for a u
// of another kind, and [ErrNotFinite] for a result that is not finite.
func (v Value) ToAffine(u AffineUnit) (AffineValue, error) {
	if !u.Valid() {
		return AffineValue{}, fmt.Errorf("%w: the zero AffineUnit names no unit", ErrNotAffine)
	}
	if v.unit.kind != u.kind {
		return AffineValue{}, fmt.Errorf("%w: cannot express %s in %s", ErrIncompatible, v.unit.kind, u.kind)
	}
	m := affineConvert(v.mag, v.Unit().factor, 0, u.factor, u.offset)
	if !isFinite(m) {
		return AffineValue{}, fmt.Errorf("%w: cannot express %s in %s", ErrNotFinite, v, u)
	}
	return AffineValue{m, u}, nil
}

// affineConvert returns m — carried in a unit of factor fFactor and offset
// fOffset — expressed in a unit of factor tFactor and offset tOffset:
// (m × fFactor + fOffset − tOffset) ÷ tFactor.
//
// It is one step, not a shift composed with a [rescale]: composing them would
// round the shifted magnitude before the division could use it, which is the
// composition this package refuses everywhere else. The whole expression is
// evaluated in exact rationals and rounded once, so the result is correctly
// rounded at both ends of the range. Temperatures sit in the middle of the
// float64 range, so the cost of always taking the rational path is paid where
// nothing notices it.
//
// Where both offsets are zero there is no shift to carry and the conversion is
// exactly [rescale], bit for bit — the path [Value] has always taken. A
// non-finite operand has no exact rational and keeps the float64 expression.
func affineConvert(m, fFactor, fOffset, tFactor, tOffset float64) float64 {
	if fOffset == 0 && tOffset == 0 {
		return rescale(m, fFactor, tFactor)
	}
	if !exactly(m, fFactor, fOffset, tFactor, tOffset) || tFactor == 0 {
		return canonicalZero((m*fFactor + fOffset - tOffset) / tFactor)
	}
	n := baseRat(m, fFactor)
	n.Add(n, new(big.Rat).SetFloat64(fOffset))
	n.Sub(n, new(big.Rat).SetFloat64(tOffset))
	return exact(n.Quo(n, new(big.Rat).SetFloat64(tFactor)))
}
