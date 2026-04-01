package bloom

import (
	"sync"

	"github.com/bits-and-blooms/bloom/v3"
)

type BloomFilter interface {
	Add(data []byte)
	Test(data []byte) bool
}

type filter struct {
	mu sync.RWMutex
	bf *bloom.BloomFilter
}

func NewBloomFilter(n uint, p float64) BloomFilter {
	return &filter{
		bf: bloom.NewWithEstimates(n, p),
	}
}

func (f *filter) Add(data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bf.Add(data)
}

func (f *filter) Test(data []byte) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.bf.Test(data)
}
