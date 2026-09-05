package units

// System records the current default units for the kinds a sketch uses: a
// default length unit and a default angle unit. It is how the library "knows"
// which units to present results in.
//
// The fields are exported and the zero System is usable: a field left unset —
// or set to a unit of the wrong kind — is ignored, and the kind's base unit is
// used instead. A System can therefore never present a quantity as something
// other than what it measures.
type System struct {
	Length Unit // default length unit; ignored unless it measures [Length]
	Angle  Unit // default angle unit; ignored unless it measures [Angle]
}

// Metric returns a system using millimetres and degrees — a common CAD default.
func Metric() System { return System{Length: Millimeter, Angle: Degree} }

// SI returns a system using metres and radians.
func SI() System { return System{Length: Meter, Angle: Radian} }

// Imperial returns a system using inches and degrees.
func Imperial() System { return System{Length: Inch, Angle: Degree} }

// UnitFor returns the system's default unit for a kind: the configured length
// or angle unit, and for any other kind its base unit — square millimetres for
// an area, kilograms for a mass.
//
// The returned unit always measures k. A configured field that does not measure
// the kind it is configured for — the zero [Unit], which reads as [One], or a
// unit of some other kind — is ignored, and k's base unit is used instead. A
// kind with no registered base unit ([BaseUnit] reports false for it) yields the
// synthetic factor-1 unit of that kind, never a unit of some other kind. A
// presentation unit that changed the kind would be the coercion this library
// exists to prevent.
func (s System) UnitFor(k Kind) Unit {
	switch k {
	case Length:
		if u := s.Length.normalize(); u.kind == Length {
			return u
		}
	case Angle:
		if u := s.Angle.normalize(); u.kind == Angle {
			return u
		}
	}
	return baseUnitFor(k)
}

// In returns v's magnitude expressed in the system's default unit for v's kind,
// as [System.UnitFor] reports it. The default unit measures v's kind, so the
// conversion is never a kind error; the returned number is always in that unit.
//
// It has no error to report, so a magnitude that is not finite in that unit — a
// v built from an infinity, or one whose conversion genuinely overflows — is
// returned as the infinity or NaN it is, never as a finite number that would be
// read as a quantity it is not.
//
// The conversion is [convert], the whole of it: the one helper [Value.In] itself
// uses, called for the number it returns rather than for the error it cannot. A
// base magnitude formed here instead — v.Base() ÷ u.Factor() — would be a
// composition, and would round twice on its way to a boundary that the helper
// decides on the true value. It carries the units' zeros as well as their sizes,
// so a temperature presented in an affine unit is the temperature it is.
func (s System) In(v Value) float64 {
	u := s.UnitFor(v.Kind())
	return convert(v.mag, v.Unit(), u.normalize())
}

// Display returns v converted to the system's default unit for its kind, as
// [System.UnitFor] reports it. A v whose magnitude is not finite in that unit
// cannot be carried in it, so v is returned unchanged — in its own unit, as the
// quantity it already is.
func (s System) Display(v Value) Value {
	c, err := v.Convert(s.UnitFor(v.Kind()))
	if err != nil {
		return v
	}
	return c
}

// LengthFromBase wraps a base-unit (millimetre) length in the system's default
// length unit, as [System.UnitFor] reports it, so the result is a length
// whatever the system carries in its Length field.
func (s System) LengthFromBase(base float64) Value { return FromBase(base, s.UnitFor(Length)) }

// AngleFromBase wraps a base-unit (radian) angle in the system's default angle
// unit, as [System.UnitFor] reports it, so the result is an angle whatever the
// system carries in its Angle field.
func (s System) AngleFromBase(base float64) Value { return FromBase(base, s.UnitFor(Angle)) }
