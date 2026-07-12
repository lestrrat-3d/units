package units

import (
	"math"
	"strconv"
	"strings"
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
type Unit struct {
	symbol string
	kind   Kind
	factor float64 // magnitude * factor == magnitude in the kind's base unit
}

// Symbol returns the unit's short symbol (e.g. "mm"); it is empty for the
// dimensionless unit.
func (u Unit) Symbol() string { return u.symbol }

// Kind returns the kind of quantity the unit measures.
func (u Unit) Kind() Kind { return u.kind }

// Factor returns the multiplier that converts a magnitude in this unit to the
// kind's base unit.
func (u Unit) Factor() float64 { return u.factor }

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

// registry maps symbols back to units for serialization and lookup.
var registry = map[string]Unit{}

// baseUnits maps a kind to its base unit — the one whose factor is 1. Only the
// named kinds have an entry; a kind produced by composition need not.
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

func define(symbol string, kind Kind, factor float64) Unit {
	if strings.HasPrefix(symbol, "[") {
		panic("units: unit symbol namespace is reserved: " + strconv.Quote(symbol))
	}
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
// The symbol must be unique: Define panics if it is already registered.
// Redefining a symbol would change the meaning of every value that names it,
// including the built-ins, so a collision is a programming error rather than an
// intent to replace.
//
// Symbols opening with "[" are reserved for the library: Define panics on one.
// That is the namespace of the synthetic units a [Value] of an unnamed kind is
// carried in ("[L^-1]"). Registering a unit there would let a persisted symbol
// deserialize as a kind other than the one it was written with.
func Define(symbol string, kind Kind, factorToBase float64) Unit {
	return define(symbol, kind, factorToBase)
}

// Lookup returns the unit previously registered for symbol. It is intended for
// deserialization; prefer the typed [Unit] constants in normal code.
func Lookup(symbol string) (Unit, bool) {
	u, ok := registry[symbol]
	return u, ok
}
