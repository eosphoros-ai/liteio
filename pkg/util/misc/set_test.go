package misc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSet(t *testing.T) {
	s := NewEmptySet()
	s.Add("1")
	s.Add("2")
	s.Add("3")
	s.Add("4")

	s2 := NewEmptySet()
	s2.Add("3")
	s2.Add("4")
	s2.Add("5")
	s2.Add("6")

	s3 := s.Difference(s2)
	assert.Equal(t, 2, s3.Size())
	assert.True(t, s3.Contains("1"))
	assert.True(t, s3.Contains("2"))
}
