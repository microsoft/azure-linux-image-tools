package mathutils

type Integer interface {
	~uint | ~int | ~uint8 | ~int8 | ~uint16 | ~int16 | ~uint32 | ~int32 | ~uint64 | ~int64 | ~uintptr
}

func RoundUp[I Integer](size I, alignment I) I {
	div := size / alignment
	mod := size % alignment
	if mod == 0 {
		return size
	}
	if size < 0 {
		return div * alignment
	}
	return (div + 1) * alignment
}

func RoundDown[I Integer](size I, alignment I) I {
	div := size / alignment
	mod := size % alignment
	if mod == 0 {
		return size
	}
	if size < 0 {
		return (div - 1) * alignment
	}
	return div * alignment
}

func DivRoundUp[I Integer](numerator I, denominator I) I {
	div := numerator / denominator
	rem := numerator % denominator
	if rem != 0 && numerator >= 0 {
		div += 1
	}

	return div
}
