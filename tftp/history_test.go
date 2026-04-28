package tftp

import (
	"testing"
	"time"
)

func TestHistory_AddBeyondCap(t *testing.T) {
	h := NewHistory(3)
	for i := 0; i < 5; i++ {
		h.Add(Transfer{ID: string(rune('a' + i))})
	}
	got := h.Snapshot()
	if len(got) != 3 {
		t.Fatalf("want 3 items, got %d", len(got))
	}
	wantIDs := []string{"e", "d", "c"}
	for i, id := range wantIDs {
		if got[i].ID != id {
			t.Errorf("index %d: want ID %q, got %q", i, id, got[i].ID)
		}
	}
}

func TestHistory_SnapshotIsCopy(t *testing.T) {
	h := NewHistory(3)
	h.Add(Transfer{ID: "a", Bytes: 1})
	snap := h.Snapshot()
	snap[0].Bytes = 999
	if h.Snapshot()[0].Bytes != 1 {
		t.Error("Snapshot must return a copy, not a shared slice")
	}
}

func TestHistory_Clear(t *testing.T) {
	h := NewHistory(3)
	h.Add(Transfer{ID: "a"})
	h.Add(Transfer{ID: "b"})
	h.Clear()
	if len(h.Snapshot()) != 0 {
		t.Error("Clear did not empty history")
	}
}

func TestHistory_EndedAtPreserved(t *testing.T) {
	h := NewHistory(3)
	now := time.Now()
	h.Add(Transfer{ID: "a", EndedAt: now, Status: "ok"})
	got := h.Snapshot()[0]
	if !got.EndedAt.Equal(now) || got.Status != "ok" {
		t.Errorf("fields not preserved: %+v", got)
	}
}
