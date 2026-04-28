package tftp

import "sync"

// History is a bounded, newest-first, thread-safe transfer log.
type History struct {
	mu    sync.Mutex
	items []Transfer
	cap   int
}

func NewHistory(cap int) *History {
	return &History{cap: cap}
}

func (h *History) Add(t Transfer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.items = append([]Transfer{t}, h.items...)
	if len(h.items) > h.cap {
		h.items = h.items[:h.cap]
	}
}

func (h *History) Snapshot() []Transfer {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]Transfer, len(h.items))
	copy(out, h.items)
	return out
}

func (h *History) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.items = nil
}
