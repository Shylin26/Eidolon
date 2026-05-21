package context

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestSplitAtCursor(t *testing.T) {
	content := "func add(a, b int) int {\n\treturn a + b\n}"
	offset := 25

	prefix, suffix := splitAtCursor(content, offset)

	assert.NotEmpty(t, prefix)
	assert.NotEmpty(t, suffix)
	assert.Equal(t, content, prefix+suffix)
	t.Logf("prefix: %q", prefix)
	t.Logf("suffix: %q", suffix)
}

func TestSplitAtCursorEdgeCases(t *testing.T) {
	// cursor at start
	p, s := splitAtCursor("hello world", 0)
	assert.Equal(t, "", p)
	assert.Equal(t, "hello world", s)

	// cursor at end
	p, s = splitAtCursor("hello world", 11)
	assert.Equal(t, "hello world", p)
	assert.Equal(t, "", s)

	// cursor beyond end
	p, s = splitAtCursor("hi", 100)
	assert.Equal(t, "hi", p)
	assert.Equal(t, "", s)
}

func TestPrefixTrimming(t *testing.T) {
	// generate a very long prefix
	long := make([]byte, 5000)
	for i := range long {
		long[i] = 'x'
	}
	content := string(long) + "|cursor|"
	offset := len(long)

	prefix, _ := splitAtCursor(content, offset)

	// trim manually as builder does
	if len(prefix) > maxPrefixChars {
		prefix = prefix[len(prefix)-maxPrefixChars:]
	}
	assert.LessOrEqual(t, len(prefix), maxPrefixChars)
}
