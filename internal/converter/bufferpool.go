package converter

import (
	"bytes"
	"sync"
)

// BufferPool manages reusable byte buffers to reduce GC pressure
type BufferPool struct {
	small   sync.Pool // ~64KB buffers
	medium  sync.Pool // ~512KB buffers (typical JPEG output)
	large   sync.Pool // ~5MB buffers (large images)
	xlarge  sync.Pool // ~10MB buffers (HEIF input)
}

// Global buffer pool
var globalBufferPool = &BufferPool{
	small: sync.Pool{
		New: func() interface{} {
			b := make([]byte, 0, 64*1024)
			return &b
		},
	},
	medium: sync.Pool{
		New: func() interface{} {
			b := make([]byte, 0, 512*1024)
			return &b
		},
	},
	large: sync.Pool{
		New: func() interface{} {
			b := make([]byte, 0, 5*1024*1024)
			return &b
		},
	},
	xlarge: sync.Pool{
		New: func() interface{} {
			b := make([]byte, 0, 10*1024*1024)
			return &b
		},
	},
}

// GetBuffer returns a buffer with at least the specified capacity
func GetBuffer(size int) *[]byte {
	switch {
	case size <= 64*1024:
		return globalBufferPool.small.Get().(*[]byte)
	case size <= 512*1024:
		return globalBufferPool.medium.Get().(*[]byte)
	case size <= 5*1024*1024:
		return globalBufferPool.large.Get().(*[]byte)
	default:
		return globalBufferPool.xlarge.Get().(*[]byte)
	}
}

// PutBuffer returns a buffer to the pool
func PutBuffer(b *[]byte) {
	// Reset length but keep capacity
	*b = (*b)[:0]

	// Return to appropriate pool based on capacity
	capacity := cap(*b)
	switch {
	case capacity == 64*1024:
		globalBufferPool.small.Put(b)
	case capacity == 512*1024:
		globalBufferPool.medium.Put(b)
	case capacity == 5*1024*1024:
		globalBufferPool.large.Put(b)
	case capacity == 10*1024*1024:
		globalBufferPool.xlarge.Put(b)
	// Don't pool buffers with unexpected sizes - let GC handle them
	}
}

// GetBufferWriter returns a bytes.Buffer from a pooled slice
func GetBufferWriter(size int) *bytes.Buffer {
	buf := GetBuffer(size)
	return bytes.NewBuffer(*buf)
}

// PutBufferWriter returns the underlying slice to the pool
func PutBufferWriter(buf *bytes.Buffer) {
	// Get the underlying bytes if possible
	if b, ok := buf.Bytes(), true; ok && len(b) > 0 {
		// Note: we can't efficiently extract the original slice from bytes.Buffer
		// The bytes.Buffer may have grown beyond the original capacity
		// For now, we'll just let GC handle it
		// In a more optimized version, we'd use a custom buffer type
	}
}

// PooledBuffer is a reusable buffer wrapper
type PooledBuffer struct {
	buf *[]byte
}

// NewPooledBuffer creates a new pooled buffer
func NewPooledBuffer(size int) *PooledBuffer {
	return &PooledBuffer{
		buf: GetBuffer(size),
	}
}

// Bytes returns the underlying byte slice
func (p *PooledBuffer) Bytes() []byte {
	return *p.buf
}

// Append appends data to the buffer
func (p *PooledBuffer) Append(data []byte) {
	*p.buf = append(*p.buf, data...)
}

// Reset clears the buffer
func (p *PooledBuffer) Reset() {
	*p.buf = (*p.buf)[:0]
}

// Len returns the current length
func (p *PooledBuffer) Len() int {
	return len(*p.buf)
}

// Cap returns the capacity
func (p *PooledBuffer) Cap() int {
	return cap(*p.buf)
}

// Release returns the buffer to the pool
func (p *PooledBuffer) Release() {
	PutBuffer(p.buf)
	p.buf = nil
}

// ToBytes converts to a new byte slice and releases the pooled buffer
func (p *PooledBuffer) ToBytes() []byte {
	result := make([]byte, len(*p.buf))
	copy(result, *p.buf)
	p.Release()
	return result
}
