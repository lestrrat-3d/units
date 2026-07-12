package units

import (
	"math"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// Unit is a unit of measure. Units are values, compared by identity of their
// (symbol, kind, factor); obtain them from the package constants such as
// [Millimeter] or [Degree], or register new ones with [Define]. There is no
// way to name a unit by a bare string in the value-building API.
//
// Symbols are ASCII. A dimension exponent is written with a caret and a digit:
// "mm^2" is the square millimetre, "kg/mm^3" the kilogram per cubic millimetre.
// A symbol opening with "[" is reserved for the library's synthetic units (see
// [Define]).
//
// A registered symbol is one the text form can carry back byte-identically, through
// this package's own parser and through a standard text encoder alike: [Define]
// rejects a symbol [Value.UnmarshalText] could not parse (one carrying whitespace,
// the form's separator) and one an encoder would rewrite (invalid UTF-8, U+FFFD, a
// control character, the noncharacters U+FFFE and U+FFFF). [One]'s symbol is the
// empty one, and a dimensionless value is written as the bare magnitude.
//
// Every unit's factor is positive and finite ([Define] rejects anything else),
// so a conversion through a unit is always well defined. The zero Unit is [One]:
// its factor field is 0, which is not a usable multiplier, so it is read as the
// dimensionless factor-1 unit it otherwise already is.
type Unit struct {
	symbol string
	kind   Kind
	factor float64 // magnitude * factor == magnitude in the kind's base unit
}

// normalize reads the zero Unit as [One]. Every registered or synthetic unit has
// a positive, finite factor, so a factor of 0 identifies the zero value and
// nothing else — and dividing or multiplying by it would turn a quantity into an
// infinity or a NaN rather than the 0-of-[One] the zero Unit means.
func (u Unit) normalize() Unit {
	if u.factor == 0 {
		return One
	}
	return u
}

// Symbol returns the unit's short symbol (e.g. "mm"); it is empty for the
// dimensionless unit.
func (u Unit) Symbol() string { return u.symbol }

// Kind returns the kind of quantity the unit measures.
func (u Unit) Kind() Kind { return u.kind }

// Factor returns the multiplier that converts a magnitude in this unit to the
// kind's base unit. It is always positive and finite; for the zero Unit it is 1,
// the factor of [One].
func (u Unit) Factor() float64 { return u.normalize().factor }

// String returns the unit's symbol, or "(dimensionless)" for [One].
func (u Unit) String() string {
	if u.symbol == "" {
		return "(dimensionless)"
	}
	return u.symbol
}

// The built-in units. Every kind with a name has a base unit, whose factor is 1:
// [One], [Millimeter], [SquareMillimeter], [CubicMillimeter], [Kilogram],
// [KilogramPerCubicMillimeter], [Radian], [KilogramSquareMillimeter] and
// [QuarticMillimeter].
var (
	// One is the dimensionless unit.
	One = defineBase("", Dimensionless)

	// Millimeter, and the length units below it, measure [Length]; the
	// millimetre is the base unit.
	Millimeter = defineBase("mm", Length)
	Centimeter = define("cm", Length, 10)
	Meter      = define("m", Length, 1000)
	Inch       = define("in", Length, 25.4)
	Foot       = define("ft", Length, 304.8)
	Thou       = define("thou", Length, 0.0254) // a.k.a. mil; 1/1000 inch

	// SquareMillimeter, and the area units below it, measure [Area]; the square
	// millimetre is the base unit.
	SquareMillimeter = defineBase("mm^2", Area)
	SquareCentimeter = define("cm^2", Area, 100)
	SquareMeter      = define("m^2", Area, 1e6)
	SquareInch       = define("in^2", Area, 645.16)

	// CubicMillimeter, and the volume units below it, measure [Volume]; the
	// cubic millimetre is the base unit.
	CubicMillimeter = defineBase("mm^3", Volume)
	CubicCentimeter = define("cm^3", Volume, 1000) // the millilitre
	CubicMeter      = define("m^3", Volume, 1e9)
	CubicInch       = define("in^3", Volume, 16387.064)
	Liter           = define("L", Volume, 1e6)

	// Kilogram, and the mass units below it, measure [Mass]; the kilogram is the
	// base unit.
	Kilogram = defineBase("kg", Mass)
	Gram     = define("g", Mass, 0.001)
	Pound    = define("lb", Mass, 0.45359237)

	// KilogramPerCubicMillimeter, and the density units below it, measure
	// [Density]; the kilogram per cubic millimetre is the base unit.
	KilogramPerCubicMillimeter = defineBase("kg/mm^3", Density)
	KilogramPerCubicMeter      = define("kg/m^3", Density, 1e-9)
	GramPerCubicCentimeter     = define("g/cm^3", Density, 1e-6)

	// KilogramSquareMillimeter measures [MomentOfInertia] (M·L²); it is the base
	// unit.
	KilogramSquareMillimeter = defineBase("kg*mm^2", MomentOfInertia)

	// QuarticMillimeter measures [SecondMomentOfArea] (L⁴); it is the base unit.
	QuarticMillimeter = defineBase("mm^4", SecondMomentOfArea)

	// Radian and Degree measure [Angle]; the radian is the base unit.
	Radian = defineBase("rad", Angle)
	Degree = define("deg", Angle, math.Pi/180)
)

// registry maps symbols back to units for serialization and lookup. It is
// mutable after init — [Define] adds to it — so registryMu guards every read and
// write of it: an application may register its units from any goroutine while
// others look symbols up.
var (
	registryMu sync.RWMutex
	registry   = map[string]Unit{}
)

// baseUnits maps a kind to its base unit — the one whose factor is 1. Only the
// named kinds have an entry; a kind produced by composition need not. It is
// written only by defineBase, which runs during package initialization, and is
// read-only thereafter, so it needs no lock.
var baseUnits = map[Kind]Unit{}

// BaseUnit returns the base unit registered for a kind (the unit whose factor
// is 1), and whether there is one. An unnamed kind — an inverse length, say,
// produced by dividing a dimensionless value by a length — has no base unit,
// and reports false.
func BaseUnit(k Kind) (Unit, bool) {
	u, ok := baseUnits[k]
	return u, ok
}

// baseUnitFor returns the unit a composed [Value] is carried in: the kind's
// registered base unit when it has one, and otherwise a synthetic unit of factor
// 1 whose symbol is the kind's canonical bracketed form ("[L^-1]"). That
// synthetic unit is not added to the registry, so [Lookup] does not find it and
// [BaseUnit] still reports the kind as having none. A value carrying one is a
// transient intermediate and must not be persisted: convert it to a named kind
// first.
func baseUnitFor(k Kind) Unit {
	if u, ok := baseUnits[k]; ok {
		return u
	}
	return Unit{symbol: k.canonicalSymbol(), kind: k, factor: 1}
}

// hasWhitespace reports whether symbol carries a rune the text form cannot carry:
// any whitespace ([unicode.IsSpace]) — the ASCII space, a tab, a newline, a carriage
// return, a vertical tab, a form feed, and the Unicode ones (NEL, NBSP, the ideographic
// space) besides.
//
// It is read straight off [Value.UnmarshalText]'s grammar. A value's text is
// "<magnitude> <symbol>", and the parser cuts it at the first ASCII space:
// [strconv.FormatFloat] never writes a space, so the cut lands on the separator, and
// everything after it is the symbol — which the parser then refuses if it holds a space
// of its own. So a symbol containing a space is a symbol [Value.MarshalText] can write
// and nothing can read: "probe space" comes back as two tokens, not a unit.
//
// The whole whitespace class is rejected and not the ASCII space alone. Whitespace is
// what separates a magnitude from its symbol in this form, and a symbol must be one
// token of it — a symbol that another whitespace rune could split, or that a document
// could trim, is not one this form can promise to carry back.
func hasWhitespace(symbol string) bool {
	return strings.IndexFunc(symbol, unicode.IsSpace) >= 0
}

// hasEncoderUnsafeRune reports whether symbol carries a rune the standard text
// encoders do not deliver byte-identically. The text form exists so that
// encoding/json and encoding/xml can carry a [Value], and neither fails on text
// it cannot represent — each rewrites it as U+FFFD and carries on. A symbol
// containing such a rune is one [Value.MarshalText] writes and the decoder hands
// back changed: the whitespace failure again, one encoder further out.
//
// The runes are read straight off what the encoders do:
//
//   - U+FFFD itself. It is the rune every lossy rewrite lands on, so a registered
//     symbol containing it would be a standing target: any other symbol, corrupted
//     anywhere and normalized by any encoder, would resolve to it — and a quantity
//     would come back from a document as a different unit of a different kind.
//     With it unregistrable, a rewrite can only produce a symbol [Lookup] refuses.
//   - A control character ([unicode.IsControl]). XML 1.0 has no representation
//     for a C0 control, so encoding/xml writes U+FFFD in its place (encoding/json
//     escapes and restores one, but a symbol must survive every encoder in the
//     loop, not the friendliest). The class is rejected whole — DEL and the C1
//     range with it — for the whitespace rule's reason: a control character is
//     not text a document format promises to carry, and "no controls" is a
//     boundary that does not move with the encoder list.
//   - The noncharacters U+FFFE and U+FFFF, which are no XML characters either:
//     encoding/xml writes U+FFFD for them too.
func hasEncoderUnsafeRune(symbol string) bool {
	return strings.ContainsFunc(symbol, func(r rune) bool {
		return unicode.IsControl(r) || r == '\uFFFD' || r == '\uFFFE' || r == '\uFFFF'
	})
}

func define(symbol string, kind Kind, factor float64) Unit {
	// Valid UTF-8 first: encoding.TextMarshaler is a contract to produce UTF-8
	// text, so a symbol that is not UTF-8 breaks MarshalText's contract outright —
	// and encoding/json replaces every invalid byte with U+FFFD, so the bytes
	// written are never the bytes read. It also makes the rune checks below
	// well-defined.
	if !utf8.ValidString(symbol) {
		panic("units: unit symbol must be valid UTF-8: " + strconv.Quote(symbol))
	}
	if hasWhitespace(symbol) {
		panic("units: unit symbol must not contain whitespace: " + strconv.Quote(symbol))
	}
	if hasEncoderUnsafeRune(symbol) {
		panic("units: unit symbol must not contain a rune a text encoder rewrites: " + strconv.Quote(symbol))
	}
	if strings.HasPrefix(symbol, "[") {
		panic("units: unit symbol namespace is reserved: " + strconv.Quote(symbol))
	}
	if factor <= 0 || math.IsInf(factor, 0) || math.IsNaN(factor) {
		panic("units: unit factor must be positive and finite: " + strconv.FormatFloat(factor, 'g', -1, 64))
	}
	if kind.Overflowed() {
		panic("units: unit kind has overflowed: " + strconv.Quote(symbol))
	}

	registryMu.Lock()
	defer registryMu.Unlock()

	if _, dup := registry[symbol]; dup {
		panic("units: unit symbol already defined: " + strconv.Quote(symbol))
	}
	u := Unit{symbol: symbol, kind: kind, factor: factor}
	registry[symbol] = u
	return u
}

func defineBase(symbol string, kind Kind) Unit {
	u := define(symbol, kind, 1)
	baseUnits[kind] = u
	return u
}

// Define registers and returns a new unit measuring kind, whose magnitudes
// convert to the kind's base unit by multiplying by factorToBase. It enables
// callers to extend the built-in set (e.g. a "yard").
//
// # Every registered symbol is one the text form can carry back
//
// A registered symbol must survive the round trip byte-identically, through this
// package's own parser and through a standard text encoder alike. Define enforces
// both where a symbol enters the registry rather than where one is written, and
// panics on a symbol that fails either.
//
// The parser: the symbol must hold no whitespace — no space, tab, newline, carriage
// return, vertical tab, form feed, or any other [unicode.IsSpace] rune — and the
// empty symbol, which is [One]'s, is registered already. [Value.MarshalText] renders
// a value as "<magnitude> <symbol>" and [Value.UnmarshalText] cuts the text at the
// first space, so a symbol carrying one could be written and never read: "3 probe
// space" is a magnitude and two tokens, and the value is lost at the document
// boundary.
//
// The encoders: the symbol must be valid UTF-8, and it must carry no rune a text
// encoder rewrites. [encoding.TextMarshaler] is a contract to produce UTF-8 text,
// and an encoder does not fail on text that is not — encoding/json replaces every
// invalid byte with U+FFFD and carries on, so the bytes written are never the bytes
// read. U+FFFD itself is refused as the other half of that rewrite: it is the rune
// every lossy normalization lands on, so a registered symbol containing it would be
// a standing target that any corrupted document resolves to — a quantity coming
// back from a document as a different unit of a different kind, which is the one
// failure this library exists to prevent. And a control character (any
// [unicode.IsControl] rune) or the noncharacter U+FFFE or U+FFFF is refused because
// encoding/xml — which carries a [Value] through the same text form — has no
// representation for one and writes U+FFFD in its place; the control class is
// rejected whole, as whitespace is, rather than admitting the corners of it
// today's encoders happen to pass through.
//
// So a symbol [Lookup] resolves is a symbol [Value.UnmarshalText] can read and a
// standard encoder delivers untouched, and MarshalText's [Lookup] guard is the
// whole of what it needs: registered means readable, by construction.
//
// The symbol must be unique: Define panics if it is already registered.
// Redefining a symbol would change the meaning of every value that names it,
// including the built-ins, so a collision is a programming error rather than an
// intent to replace.
//
// Symbols opening with "[" are reserved for the library: Define panics on one.
// That is the namespace of the synthetic units a [Value] of an unnamed kind is
// carried in ("[L^-1]"). Registering a unit there would let a persisted symbol
// deserialize as a kind other than the one it was written with.
//
// factorToBase must be positive and finite: Define panics on a zero, negative,
// infinite or NaN factor. Such a unit could not convert — every magnitude
// expressed in it would come back as an infinity or a NaN — so it, too, is a
// programming error rather than an exotic unit.
//
// kind must not have overflowed ([Kind.Overflowed]): Define panics on one. An
// overflowed kind is a programming error made visible — its exponents are clamped
// stand-ins, it has no base unit, and it is carried only by the unregistered
// synthetic symbol "[overflow]" — so registering it under an ordinary symbol
// would launder it into a legitimate, resolvable, persistable unit.
//
// Nothing is registered when Define panics: the symbol stays free.
//
// Define is safe to call from multiple goroutines, concurrently with [Lookup]
// and [BaseUnit].
func Define(symbol string, kind Kind, factorToBase float64) Unit {
	return define(symbol, kind, factorToBase)
}

// Lookup returns the unit previously registered for symbol. It is intended for
// deserialization; prefer the typed [Unit] constants in normal code. It is safe
// to call concurrently with [Define].
func Lookup(symbol string) (Unit, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	u, ok := registry[symbol]
	return u, ok
}
