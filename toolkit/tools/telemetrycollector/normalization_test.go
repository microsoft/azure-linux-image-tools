// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestNormalizeArrayAttributes(t *testing.T) {
	for _, test := range []struct {
		name  string
		value []any
		want  string
	}{
		{name: "strings", value: []any{"curl", "vim"}, want: `["curl","vim"]`},
		{name: "empty", value: []any{}, want: `[]`},
		{name: "mixed", value: []any{true, int64(2), 1.5}, want: `[true,2,1.5]`},
		{name: "nested", value: []any{[]any{"a"}, map[string]any{"key": "value"}}, want: `[["a"],{"key":"value"}]`},
		{name: "escaping", value: []any{"a\"b", "c\\d", "line\nbreak"}, want: `["a\"b","c\\d","line\nbreak"]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			attributes := pcommon.NewMap()
			require.NoError(t, attributes.PutEmptySlice("array").FromRaw(test.value))
			attributes.PutStr("string", "unchanged")
			attributes.PutInt("integer", 2)
			attributes.PutDouble("double", 1.5)
			attributes.PutBool("boolean", true)

			require.NoError(t, normalizeArrayAttributes(attributes))

			attributesRaw := attributes.AsRaw()
			assert.JSONEq(t, test.want, attributesRaw["array"].(string))
			assert.Equal(t, "unchanged", attributesRaw["string"])
			assert.Equal(t, int64(2), attributesRaw["integer"])
			assert.Equal(t, 1.5, attributesRaw["double"])
			assert.Equal(t, true, attributesRaw["boolean"])
		})
	}
}
