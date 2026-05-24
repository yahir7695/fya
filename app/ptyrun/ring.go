package ptyrun

import "sync"

// tailBuffer is a capped byte sink: writes append, and once total bytes exceed
// limit the oldest bytes are dropped. It is NOT a ring buffer (no circular
// indexing) — the name reflects that callers want the tail of recent output.
type tailBuffer struct {
	mu    sync.Mutex
	data  []byte
	limit int
}

// Write appends data and trims the buffer so its length never exceeds limit.
// A single write larger than limit is clipped to its trailing limit bytes
// before being appended, so the backing array never has to grow beyond ~limit
// regardless of input size.
func (r *tailBuffer) Write(data []byte) {
	if len(data) > r.limit {
		data = data[len(data)-r.limit:]
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data = append(r.data, data...)
	if len(r.data) <= r.limit {
		return
	}
	keep := r.data[len(r.data)-r.limit:]
	copy(r.data, keep)
	r.data = r.data[:r.limit]
}

// String returns a snapshot of the current buffered bytes. The returned string
// is a value-copy; the underlying slice is not exposed.
func (r *tailBuffer) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return string(r.data)
}
