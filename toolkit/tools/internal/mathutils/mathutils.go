package mathutils

type UnsignedInteger interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

func RoundUp[I UnsignedInteger](size I, alignment I) I {
	div := size / alignment
	mod := size % alignment
	if mod == 0 {
		return size
	}
	return (div + 1) * alignment
}

func RoundDown[I UnsignedInteger](size I, alignment I) I {
	div := size / alignment
	mod := size % alignment
	if mod == 0 {
		return size
	}
	return div * alignment
}

func DivRoundUp[I UnsignedInteger](numerator I, denominator I) I {
	sizeInSectors := numerator / denominator
	rem := numerator % denominator
	if rem != 0 {
		sizeInSectors += 1
	}

	return sizeInSectors
}
