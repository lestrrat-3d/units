package units

import (
	"math"
	"math/bits"
)

// exactWordCount covers the complete exponent range of the product of two
// finite float64 values. The shared scale is 2^exactMinExponent.
const (
	exactMinExponent = -2148
	exactWordCount   = 66
)

type exactMagnitude struct {
	words [exactWordCount]uint64
}

type exactTerm struct {
	magnitude exactMagnitude
	negative  bool
}

// exactFloat decomposes a finite float64 into sign * significand * 2^exponent.
// The significand is an integer with at most 53 bits. Subnormals use the same
// exact representation, with an exponent of -1074.
func exactFloat(x float64) (negative bool, significand uint64, exponent int, ok bool) {
	if !isFinite(x) {
		return false, 0, 0, false
	}

	b := math.Float64bits(x)
	negative = b>>63 != 0
	frac := b & (uint64(1)<<52 - 1)
	exp := int((b >> 52) & 0x7ff)
	if exp == 0 {
		return negative, frac, -1074, true
	}
	return negative, frac | uint64(1)<<52, exp - 1075, true
}

func exactProduct(a, af float64, negative bool) (exactTerm, bool) {
	aneg, asig, aexp, ok := exactFloat(a)
	if !ok {
		return exactTerm{}, false
	}
	fneg, fsig, fexp, ok := exactFloat(af)
	if !ok {
		return exactTerm{}, false
	}
	if asig == 0 || fsig == 0 {
		return exactTerm{}, true
	}

	hi, lo := bits.Mul64(asig, fsig)
	var magnitude exactMagnitude
	addShifted(&magnitude, hi, lo, aexp+fexp-exactMinExponent)
	return exactTerm{magnitude: magnitude, negative: negative != aneg != fneg}, true
}

func exactFloatMagnitude(x float64) (exactMagnitude, bool) {
	negative, significand, exponent, ok := exactFloat(x)
	_ = negative
	if !ok {
		return exactMagnitude{}, false
	}
	var magnitude exactMagnitude
	if significand != 0 {
		addShifted(&magnitude, 0, significand, exponent-exactMinExponent)
	}
	return magnitude, true
}

func addShifted(dst *exactMagnitude, hi, lo uint64, shift int) {
	word := shift / 64
	offset := uint(shift % 64)
	if offset == 0 {
		addWord(dst, word, lo)
		addWord(dst, word+1, hi)
		return
	}
	addWord(dst, word, lo<<offset)
	addWord(dst, word+1, lo>>(64-offset)|hi<<offset)
	addWord(dst, word+2, hi>>(64-offset))
}

func addWord(dst *exactMagnitude, word int, value uint64) {
	if value == 0 {
		return
	}
	var carry uint64
	dst.words[word], carry = bits.Add64(dst.words[word], value, 0)
	for word++; carry != 0; word++ {
		dst.words[word], carry = bits.Add64(dst.words[word], 0, carry)
	}
}

func (m exactMagnitude) isZero() bool {
	for _, word := range m.words {
		if word != 0 {
			return false
		}
	}
	return true
}

func (m exactMagnitude) bitLen() int {
	for i := exactWordCount - 1; i >= 0; i-- {
		if m.words[i] != 0 {
			return i*64 + bits.Len64(m.words[i])
		}
	}
	return 0
}

func compareMagnitude(a, b exactMagnitude) int {
	for i := exactWordCount - 1; i >= 0; i-- {
		if a.words[i] < b.words[i] {
			return -1
		}
		if a.words[i] > b.words[i] {
			return 1
		}
	}
	return 0
}

func addMagnitude(dst *exactMagnitude, src exactMagnitude) {
	var carry uint64
	for i := range exactWordCount {
		dst.words[i], carry = bits.Add64(dst.words[i], src.words[i], carry)
	}
}

func subtractMagnitude(dst *exactMagnitude, src exactMagnitude) {
	var borrow uint64
	for i := range exactWordCount {
		dst.words[i], borrow = bits.Sub64(dst.words[i], src.words[i], borrow)
	}
}

func signedTerms(a, b exactTerm) (exactMagnitude, bool) {
	if a.magnitude.isZero() {
		return b.magnitude, b.negative
	}
	if b.magnitude.isZero() {
		return a.magnitude, a.negative
	}
	if a.negative == b.negative {
		addMagnitude(&a.magnitude, b.magnitude)
		return a.magnitude, a.negative
	}
	if compareMagnitude(a.magnitude, b.magnitude) >= 0 {
		subtractMagnitude(&a.magnitude, b.magnitude)
		return a.magnitude, a.negative
	}
	subtractMagnitude(&b.magnitude, a.magnitude)
	return b.magnitude, b.negative
}

func shiftedUint(value uint64, shift int) exactMagnitude {
	var magnitude exactMagnitude
	if value != 0 {
		addShifted(&magnitude, 0, value, shift)
	}
	return magnitude
}

func shiftedMagnitude(m exactMagnitude, shift int) exactMagnitude {
	if shift == 0 {
		return m
	}
	var result exactMagnitude
	for i, word := range m.words {
		if word == 0 {
			continue
		}
		addShifted(&result, 0, word, i*64+shift)
	}
	return result
}

// ratioExponent returns floor(log2(n / denominator)). denominator is positive.
func ratioExponent(n exactMagnitude, denominator uint64) int {
	k := n.bitLen() - bits.Len64(denominator)
	if k >= 0 {
		if compareMagnitude(n, shiftedUint(denominator, k)) < 0 {
			return k - 1
		}
		return k
	}
	if compareMagnitude(shiftedMagnitude(n, -k), shiftedUint(denominator, 0)) < 0 {
		return k - 1
	}
	return k
}

func divideMagnitude(n exactMagnitude, denominator uint64) (exactMagnitude, uint64) {
	var quotient exactMagnitude
	var remainder uint64
	for i := exactWordCount - 1; i >= 0; i-- {
		quotient.words[i], remainder = bits.Div64(remainder, n.words[i], denominator)
	}
	return quotient, remainder
}

func lowBits(m exactMagnitude, count int) exactMagnitude {
	if count <= 0 {
		return exactMagnitude{}
	}
	if count >= exactWordCount*64 {
		return m
	}
	result := m
	word := count / 64
	if count%64 != 0 {
		result.words[word] &= uint64(1)<<uint(count%64) - 1
		word++
	}
	for i := word; i < exactWordCount; i++ {
		result.words[i] = 0
	}
	return result
}

func multiplyByUint(m exactMagnitude, multiplier uint64) exactMagnitude {
	var result exactMagnitude
	var carry uint64
	for i, word := range m.words {
		hi, lo := bits.Mul64(word, multiplier)
		lo, carryIn := bits.Add64(lo, carry, 0)
		hi, carryOut := bits.Add64(hi, 0, carryIn)
		result.words[i] = lo
		carry = hi
		if carryOut != 0 {
			addWord(&result, i+1, carryOut)
		}
	}
	return result
}

func magnitudeUint64(m exactMagnitude) uint64 {
	return m.words[0]
}

func shiftedTop(m exactMagnitude, shift int) uint64 {
	word := shift / 64
	offset := uint(shift % 64)
	if word < 0 || word >= exactWordCount {
		return 0
	}
	value := m.words[word] >> offset
	if offset != 0 && word+1 < exactWordCount {
		value |= m.words[word+1] << (64 - offset)
	}
	return value
}

// roundedRatio rounds n * 2^exponent / denominator to a normal float64. It
// returns false near the float64 boundaries, where the existing rational path
// remains the reference implementation for subnormal and overflow decisions.
func roundedRatio(n exactMagnitude, negative bool, exponent int, denominator uint64) (float64, bool) {
	if n.isZero() {
		return 0, true
	}
	trailing := bits.TrailingZeros64(denominator)
	denominator >>= uint(trailing)
	exponent -= trailing
	k := ratioExponent(n, denominator)
	top := exponent + k
	if top <= -1000 || top >= 1000 {
		return 0, false
	}

	const precision = 52
	shift := precision - k
	var significand uint64
	var roundNumerator, roundDenominator exactMagnitude
	if shift >= 0 {
		quotient, remainder := divideMagnitude(shiftedMagnitude(n, shift), denominator)
		significand = magnitudeUint64(quotient)
		roundNumerator = shiftedUint(remainder, 0)
		roundDenominator = shiftedUint(denominator, 0)
	} else {
		quotient, remainder := divideMagnitude(n, denominator)
		rightShift := -shift
		significand = shiftedTop(quotient, rightShift)
		low := lowBits(quotient, rightShift)
		roundNumerator = multiplyByUint(low, denominator)
		addWord(&roundNumerator, 0, remainder)
		roundDenominator = shiftedUint(denominator, rightShift)
	}

	denominatorForRound := roundDenominator
	if shift >= 0 {
		if roundNumerator.words[0]*2 > denominatorForRound.words[0] ||
			(roundNumerator.words[0]*2 == denominatorForRound.words[0] && significand&1 != 0) {
			significand++
		}
	} else {
		half := shiftedMagnitude(denominatorForRound, -1)
		cmp := compareMagnitude(roundNumerator, half)
		if cmp > 0 || (cmp == 0 && significand&1 != 0) {
			significand++
		}
	}
	if significand == uint64(1)<<53 {
		significand >>= 1
		top++
	}
	if top <= -1000 || top >= 1000 {
		return 0, false
	}

	resultBits := uint64(top+1023)<<52 | significand&(uint64(1)<<52-1)
	if negative {
		resultBits |= uint64(1) << 63
	}
	return canonicalZero(math.Float64frombits(resultBits)), true
}

func exactSum(a, af, sign, b, bf, to float64) (float64, bool) {
	first, ok := exactProduct(a, af, false)
	if !ok {
		return 0, false
	}
	second, ok := exactProduct(b, bf, sign < 0)
	if !ok {
		return 0, false
	}
	_, denominator, exponent, ok := exactFloat(to)
	if !ok || denominator == 0 {
		return 0, false
	}
	numerator, negative := signedTerms(first, second)
	return roundedRatio(numerator, negative, exactMinExponent-exponent, denominator)
}

// exactEqual compares the true difference of two finite base-unit products
// against a finite non-negative tolerance without constructing a rational.
func exactEqual(a, af, b, bf, tolerance float64) (bool, bool) {
	left, ok := exactProduct(a, af, false)
	if !ok {
		return false, false
	}
	right, ok := exactProduct(b, bf, true)
	if !ok {
		return false, false
	}
	difference, _ := signedTerms(left, right)
	limit, ok := exactFloatMagnitude(tolerance)
	if !ok {
		return false, false
	}
	return compareMagnitude(difference, limit) <= 0, true
}
