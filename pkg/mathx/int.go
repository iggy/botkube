// Package mathx provides helper math utility functions.
package mathx

// IncreaseWithMax increases in by 1 but only up to maxVal value.
func IncreaseWithMax(in, maxVal int) int {
	in++
	if in > maxVal {
		return maxVal
	}
	return in
}

// DecreaseWithMin decreases in by 1 but only to minVal value.
func DecreaseWithMin(in, minVal int) int {
	in--
	if in < minVal {
		return minVal
	}
	return in
}

// Min returns the smallest of a or b.
func Min(a, b int) int {
	if a > b {
		return b
	}
	return a
}
