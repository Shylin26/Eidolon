package context

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitAtCursor(t *testing.T) {
	content := "func add(a, b int) int {\n\treturn a + b\n}"
	offset := 25

	prefix, suffix := splitAtCursor(content, offset)

	assert.NotEmpty(t, prefix)
	assert.NotEmpty(t, suffix)
	assert.Equal(t, content, prefix+suffix)
}

func TestSplitAtCursorEdgeCases(t *testing.T) {
	p, s := splitAtCursor("hello world", 0)
	assert.Equal(t, "", p)
	assert.Equal(t, "hello world", s)

	p, s = splitAtCursor("hello world", 11)
	assert.Equal(t, "hello world", p)
	assert.Equal(t, "", s)

	p, s = splitAtCursor("hi", 100)
	assert.Equal(t, "hi", p)
	assert.Equal(t, "", s)
}

func TestPrefixTrimming(t *testing.T) {
	long := make([]byte, 5000)
	for i := range long {
		long[i] = 'x'
	}
	content := string(long) + "|cursor|"
	offset := len(long)

	prefix, _ := splitAtCursor(content, offset)
	if len(prefix) > maxPrefixChars {
		prefix = prefix[len(prefix)-maxPrefixChars:]
	}
	assert.LessOrEqual(t, len(prefix), maxPrefixChars)
}

func TestSplitUnicode(t *testing.T) {
	content := "func 你好(n int) int {\n\treturn"
	offset := 10
	prefix, suffix := splitAtCursor(content, offset)
	require.Equal(t, content, prefix+suffix)
}

func TestEmptyContent(t *testing.T) {
	prefix, suffix := splitAtCursor("", 0)
	assert.Equal(t, "", prefix)
	assert.Equal(t, "", suffix)
}
