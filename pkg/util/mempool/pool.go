package mempool

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/dustin/go-humanize"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	errSlabExhausted = errors.New("slab exhausted")

	reasonSizeExceeded  = "size-exceeded"
	reasonSlabExhausted = "slab-exhausted"
)

type slab struct {
	buffer      chan unsafe.Pointer
	size, count int
	mtx         sync.Mutex
	metrics     *metrics
	name        string
}

func newSlab(bufferSize, bufferCount int, m *metrics) *slab {
	name := humanize.Bytes(uint64(bufferSize))
	m.availableBuffersPerSlab.WithLabelValues(name).Add(0) // initialize metric with value 0

	return &slab{
		size:    bufferSize,
		count:   bufferCount,
		metrics: m,
		name:    name,
	}
}

func (s *slab) init() {
	s.buffer = make(chan unsafe.Pointer, s.count)
	for i := 0; i < s.count; i++ {
		buf := make([]byte, 0, s.size)
		ptr := unsafe.Pointer(unsafe.SliceData(buf))
		s.buffer <- ptr
	}
	s.metrics.availableBuffersPerSlab.WithLabelValues(s.name).Add(float64(s.count))
}

func (s *slab) get(size int) ([]byte, error) {
	s.mtx.Lock()
	if s.buffer == nil {
		s.init()
	}
	defer s.mtx.Unlock()

	// wait for available buffer on channel
	var buf []byte
	select {
	case ptr := <-s.buffer:
		buf = unsafe.Slice((*byte)(ptr), s.size)
	default:
		s.metrics.errorsCounter.WithLabelValues(s.name, reasonSlabExhausted).Inc()
		return nil, errSlabExhausted
	}

	// Taken from https://github.com/ortuman/nuke/blob/main/monotonic_arena.go#L37-L48
	// This piece of code will be translated into a runtime.memclrNoHeapPointers
	// invocation by the compiler, which is an assembler optimized implementation.
	// Architecture specific code can be found at src/runtime/memclr_$GOARCH.s
	// in Go source (since https://codereview.appspot.com/137880043).
	for i := range buf {
		buf[i] = 0
	}

	return buf[:size], nil
}

func (s *slab) put(buf []byte) {
	if s.buffer == nil {
		panic("slab is not initialized")
	}

	ptr := unsafe.Pointer(unsafe.SliceData(buf))
	s.buffer <- ptr
}

// MemPool is an Allocator implementation that uses a fixed size memory pool
// that is split into multiple slabs of different buffer sizes.
// Buffers are re-cycled and need to be returned back to the pool, otherwise
// the pool runs out of available buffers.
type MemPool struct {
	slabs   []*slab
	metrics *metrics
}

func New(name string, buckets []Bucket, r prometheus.Registerer) *MemPool {
	a := &MemPool{
		slabs:   make([]*slab, 0, len(buckets)),
		metrics: newMetrics(r, name),
	}
	for _, b := range buckets {
		a.slabs = append(a.slabs, newSlab(int(b.Capacity), b.Size, a.metrics))
	}
	return a
}

// Get satisfies Allocator interface
// Allocating a buffer from an exhausted pool/slab, or allocating a buffer that
// exceeds the largest slab size will return an error.
func (a *MemPool) Get(size int) ([]byte, error) {
	for i := 0; i < len(a.slabs); i++ {
		if a.slabs[i].size < size {
			continue
		}
		return a.slabs[i].get(size)
	}
	a.metrics.errorsCounter.WithLabelValues("pool", reasonSizeExceeded).Inc()
	return nil, fmt.Errorf("no slab found for size: %d", size)
}

// Put satisfies Allocator interface
// Every buffer allocated with Get(size int) needs to be returned to the pool
// using Put(buffer []byte) so it can be re-cycled.
func (a *MemPool) Put(buffer []byte) bool {
	size := cap(buffer)
	for i := 0; i < len(a.slabs); i++ {
		if a.slabs[i].size < size {
			continue
		}
		a.slabs[i].put(buffer)
		return true
	}
	return false
}
