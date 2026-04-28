package tftp

import (
	"io"
	"sync/atomic"
)

// countingReader wraps an io.Reader and invokes onProgress after each read
// with the new cumulative byte count.
type countingReader struct {
	r          io.Reader
	bytes      atomic.Int64
	onProgress func(int64)
}

func newCountingReader(r io.Reader, onProgress func(int64)) *countingReader {
	return &countingReader{r: r, onProgress: onProgress}
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		total := c.bytes.Add(int64(n))
		if c.onProgress != nil {
			c.onProgress(total)
		}
	}
	return n, err
}

// countingWriter wraps an io.Writer with the same progress semantics.
type countingWriter struct {
	w          io.Writer
	bytes      atomic.Int64
	onProgress func(int64)
}

func newCountingWriter(w io.Writer, onProgress func(int64)) *countingWriter {
	return &countingWriter{w: w, onProgress: onProgress}
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	if n > 0 {
		total := c.bytes.Add(int64(n))
		if c.onProgress != nil {
			c.onProgress(total)
		}
	}
	return n, err
}

// progressThrottle returns a func that invokes emit whenever total crosses a
// 5%-of-size boundary (or every 64 KiB if size is 0).
func progressThrottle(size int64, emit func(bytes int64)) func(int64) {
	var step int64 = 64 * 1024
	if size > 0 {
		step = size / 20
		if step < 64*1024 {
			step = 64 * 1024
		}
	}
	var next atomic.Int64
	next.Store(step)
	return func(total int64) {
		for {
			threshold := next.Load()
			if total < threshold {
				return
			}
			if next.CompareAndSwap(threshold, threshold+step) {
				emit(total)
				return
			}
		}
	}
}
