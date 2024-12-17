// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package diskutils

type Version []int

func (v Version) Cmp(other Version) int {
	for i := range v {
		c1 := v[i]
		c2 := other[i]

		if i == len(other) || c1 > c2 {
			return 1
		} else if c1 < c2 {
			return -1
		}
	}

	return 0
}

func (v Version) Gt(other Version) bool {
	return v.Cmp(other) > 0
}

func (v Version) Ge(other Version) bool {
	return v.Cmp(other) >= 0
}

func (v Version) Lt(other Version) bool {
	return v.Cmp(other) < 0
}

func (v Version) Le(other Version) bool {
	return v.Cmp(other) <= 0
}

func (v Version) Eq(other Version) bool {
	return v.Cmp(other) == 0
}
