package bloom

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBloomFilter(t *testing.T) {
	bf := NewBloomFilter(1000, 0.01)
	assert.NotNil(t, bf)
	f, ok := bf.(*filter)
	require.True(t, ok)
	assert.NotNil(t, f.bf)
}

func TestMultipleAdd(t *testing.T) {
	bf := NewBloomFilter(1000, 0.01)
	items := [][]byte{
		[]byte("item1"),
		[]byte("item2"),
		[]byte("item3"),
	}
	for _, item := range items {
		bf.Add(item)
	}
	for _, item := range items {
		assert.True(t, bf.Test(item))
	}
	assert.False(t, bf.Test([]byte("nonexistent")))
}

func TestEmptyData(t *testing.T) {
	bf := NewBloomFilter(1000, 0.01)
	empty := []byte("")
	bf.Add(empty)
	assert.True(t, bf.Test(empty))
}

func TestFalsePositiveProbability(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping false positive probability test in short mode")
	}
	const n = 10000
	const p = 0.01
	bf := NewBloomFilter(n, p)

	for i := 0; i < n; i++ {
		bf.Add([]byte{byte(i % 256), byte(i / 256)})
	}

	falsePositives := 0
	const testCount = n
	for i := 0; i < testCount; i++ {
		if bf.Test([]byte{byte(i%256 + 128), byte(i/256 + 128)}) {
			falsePositives++
		}
	}
	actualP := float64(falsePositives) / float64(testCount)
	assert.LessOrEqual(t, actualP, p*3, "actual false positive rate: %f", actualP)
}
