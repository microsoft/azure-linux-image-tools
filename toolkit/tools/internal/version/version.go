// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package version

import (
	"fmt"
	"strings"
)

type Version []int

func (v Version) Cmp(other Version) int {
	count := len(v)
	if len(other) > count {
		count = len(other)
	}

	for i := 0; i < count; i++ {
		c1 := 0
		if i < len(v) {
			c1 = v[i]
		}

		c2 := 0
		if i < len(other) {
			c2 = other[i]
		}

		if c1 > c2 {
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

func (v Version) String() string {
	builder := strings.Builder{}
	for i, p := range v {
		if i != 0 {
			builder.WriteString(".")
		}
		builder.WriteString(fmt.Sprintf("%d", p))
	}
	return builder.String()
}
