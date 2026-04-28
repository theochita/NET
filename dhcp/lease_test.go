package dhcp

import (
	"net"
	"os"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *LeaseStore {
	t.Helper()
	dir := t.TempDir()
	store, err := NewLeaseStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestLeaseStore_AssignAndGetAll(t *testing.T) {
	store := newTestStore(t)
	store.Assign("aa:bb:cc:dd:ee:ff", "aa:bb:cc:dd:ee:ff", "10.0.0.1", "host1", time.Hour)

	leases := store.GetAll()
	if len(leases) != 1 {
		t.Fatalf("expected 1 lease, got %d", len(leases))
	}
	if leases[0].IP != "10.0.0.1" {
		t.Errorf("expected IP 10.0.0.1, got %s", leases[0].IP)
	}
}

func TestLeaseStore_Release(t *testing.T) {
	store := newTestStore(t)
	store.Assign("aa:bb:cc:dd:ee:ff", "aa:bb:cc:dd:ee:ff", "10.0.0.1", "host1", time.Hour)
	store.Release("aa:bb:cc:dd:ee:ff")

	if len(store.GetAll()) != 0 {
		t.Fatal("expected no leases after release")
	}
}

func TestLeaseStore_Sweep(t *testing.T) {
	store := newTestStore(t)
	store.Assign("aa:bb:cc:dd:ee:ff", "aa:bb:cc:dd:ee:ff", "10.0.0.1", "expired", -time.Second) // already expired
	store.Assign("11:22:33:44:55:66", "11:22:33:44:55:66", "10.0.0.2", "active", time.Hour)

	store.Sweep()

	leases := store.GetAll()
	if len(leases) != 1 {
		t.Fatalf("expected 1 lease after sweep, got %d", len(leases))
	}
	if leases[0].IP != "10.0.0.2" {
		t.Errorf("wrong lease survived sweep: %s", leases[0].IP)
	}
}

func TestLeaseStore_NextFreeIP(t *testing.T) {
	store := newTestStore(t)
	start := net.ParseIP("10.0.0.1").To4()
	end := net.ParseIP("10.0.0.5").To4()

	ip := store.NextFreeIP(start, end)
	if ip.String() != "10.0.0.1" {
		t.Fatalf("expected 10.0.0.1, got %s", ip)
	}

	store.Assign("aa:bb:cc:dd:ee:ff", "aa:bb:cc:dd:ee:ff", "10.0.0.1", "host1", time.Hour)
	ip = store.NextFreeIP(start, end)
	if ip.String() != "10.0.0.2" {
		t.Fatalf("expected 10.0.0.2 after first assigned, got %s", ip)
	}
}

func TestLeaseStore_NextFreeIP_PoolExhausted(t *testing.T) {
	store := newTestStore(t)
	start := net.ParseIP("10.0.0.1").To4()
	end := net.ParseIP("10.0.0.1").To4() // pool of 1

	store.Assign("aa:bb:cc:dd:ee:ff", "aa:bb:cc:dd:ee:ff", "10.0.0.1", "host1", time.Hour)
	ip := store.NextFreeIP(start, end)
	if ip != nil {
		t.Fatalf("expected nil for exhausted pool, got %s", ip)
	}
}

func TestLeaseStore_IsAssigned(t *testing.T) {
	store := newTestStore(t)
	store.Assign("aa:bb:cc:dd:ee:ff", "aa:bb:cc:dd:ee:ff", "10.0.0.1", "host1", time.Hour)

	lease, ok := store.IsAssigned("10.0.0.1")
	if !ok {
		t.Fatal("expected IP to be assigned")
	}
	if lease.MAC != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("wrong MAC: %s", lease.MAC)
	}

	_, ok = store.IsAssigned("10.0.0.99")
	if ok {
		t.Fatal("unassigned IP should not be found")
	}
}

func TestLeaseStore_Clear(t *testing.T) {
	store := newTestStore(t)
	store.Assign("aa:bb:cc:dd:ee:ff", "aa:bb:cc:dd:ee:ff", "10.0.0.1", "host1", time.Hour)
	store.Assign("11:22:33:44:55:66", "11:22:33:44:55:66", "10.0.0.2", "host2", time.Hour)

	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	if len(store.GetAll()) != 0 {
		t.Fatal("expected no leases after clear")
	}
}

func TestLeaseStore_Persistence(t *testing.T) {
	dir := t.TempDir()

	store1, _ := NewLeaseStore(dir)
	store1.Assign("aa:bb:cc:dd:ee:ff", "aa:bb:cc:dd:ee:ff", "10.0.0.1", "host1", time.Hour)

	// New store from same dir should load the persisted lease
	store2, err := NewLeaseStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	leases := store2.GetAll()
	if len(leases) != 1 || leases[0].IP != "10.0.0.1" {
		t.Fatalf("lease not persisted; got %v", leases)
	}
}

func TestLeaseStore_NextFreeIP_TopOfRangeNoOverflow(t *testing.T) {
	// Regression: a fully-leased pool ending at 255.255.255.255 used to loop
	// forever because incrementIP wraps to 0.0.0.0 and ipAfter never trips.
	store := newTestStore(t)
	start := net.ParseIP("255.255.255.254").To4()
	end := net.ParseIP("255.255.255.255").To4()
	store.Assign("aa:bb:cc:dd:ee:ff", "aa:bb:cc:dd:ee:ff", "255.255.255.254", "h1", time.Hour)
	store.Assign("11:22:33:44:55:66", "11:22:33:44:55:66", "255.255.255.255", "h2", time.Hour)

	done := make(chan net.IP, 1)
	go func() { done <- store.NextFreeIP(start, end) }()
	select {
	case ip := <-done:
		if ip != nil {
			t.Fatalf("expected nil for exhausted top-of-range pool, got %s", ip)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("NextFreeIP looped forever at top of address space")
	}
}

func TestLeaseStore_NextFreeIP_InvertedRange(t *testing.T) {
	store := newTestStore(t)
	start := net.ParseIP("10.0.0.10").To4()
	end := net.ParseIP("10.0.0.5").To4()
	if ip := store.NextFreeIP(start, end); ip != nil {
		t.Fatalf("inverted range must return nil, got %s", ip)
	}
}

func TestLeaseStore_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/leases.json", nil, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLeaseStore(dir); err != nil {
		t.Fatalf("empty leases.json must be tolerated: %v", err)
	}
}

func TestLeaseStore_ClientID_KeyAndPersistence(t *testing.T) {
	dir := t.TempDir()
	store1, _ := NewLeaseStore(dir)
	// Assign with a client-id key (option 61 style)
	store1.Assign("id:aabbccdd", "aa:bb:cc:dd:ee:ff", "10.0.0.1", "host1", time.Hour)

	l, ok := store1.GetByClient("id:aabbccdd")
	if !ok {
		t.Fatal("expected lease keyed by client-id")
	}
	if l.ClientID != "aabbccdd" {
		t.Errorf("ClientID = %q, want %q", l.ClientID, "aabbccdd")
	}
	if l.MAC != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("MAC = %q, want %q", l.MAC, "aa:bb:cc:dd:ee:ff")
	}

	// Persisted and reloaded correctly
	store2, err := NewLeaseStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	l2, ok := store2.GetByClient("id:aabbccdd")
	if !ok {
		t.Fatal("client-id lease not restored after reload")
	}
	if l2.IP != "10.0.0.1" {
		t.Errorf("reloaded IP = %q, want 10.0.0.1", l2.IP)
	}
}

func TestLeaseStore_MissingFile(t *testing.T) {
	dir := t.TempDir()
	os.Remove(dir + "/leases.json") // ensure absent

	_, err := NewLeaseStore(dir) // must not error on missing file
	if err != nil {
		t.Fatalf("unexpected error on missing leases file: %v", err)
	}
}
