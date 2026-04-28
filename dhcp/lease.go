package dhcp

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const leasesFilename = "leases.json"

// LeaseStore holds active leases in memory and persists them to JSON.
type LeaseStore struct {
	mu     sync.RWMutex
	leases map[string]Lease // key: client key ("id:<hex>" for option-61 clients, MAC string otherwise)
	path   string
}

// NewLeaseStore creates a store backed by dir/leases.json.
// Missing file is not an error — the store starts empty.
func NewLeaseStore(dir string) (*LeaseStore, error) {
	s := &LeaseStore{
		leases: make(map[string]Lease),
		path:   filepath.Join(dir, leasesFilename),
	}
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

// Assign creates or updates a lease. clientKey is the map key (option-61 "id:<hex>" or MAC);
// mac is the hardware address from chaddr, stored for display and IP-conflict detection.
func (s *LeaseStore) Assign(clientKey, mac, ip, hostname string, duration time.Duration) Lease {
	s.mu.Lock()
	defer s.mu.Unlock()
	clientID := ""
	if strings.HasPrefix(clientKey, "id:") {
		clientID = clientKey[3:]
	}
	l := Lease{
		MAC:       mac,
		ClientID:  clientID,
		IP:        ip,
		Hostname:  hostname,
		ExpiresAt: time.Now().Add(duration),
	}
	s.leases[clientKey] = l
	_ = s.persist()
	return l
}

// Release removes the lease for the given client key.
func (s *LeaseStore) Release(mac string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.leases, mac)
	_ = s.persist()
}

// GetAll returns a snapshot of all non-expired leases.
func (s *LeaseStore) GetAll() []Lease {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	out := make([]Lease, 0, len(s.leases))
	for _, l := range s.leases {
		if l.ExpiresAt.After(now) {
			out = append(out, l)
		}
	}
	return out
}

// NextFreeIP returns the first IP in [start, end] not currently leased.
// Returns nil if the pool is exhausted or the input range is invalid.
func (s *LeaseStore) NextFreeIP(start, end net.IP) net.IP {
	s.mu.RLock()
	defer s.mu.RUnlock()

	start4 := start.To4()
	end4 := end.To4()
	if start4 == nil || end4 == nil || ipAfter(start4, end4) {
		return nil
	}

	now := time.Now()
	used := make(map[string]bool, len(s.leases))
	for _, l := range s.leases {
		if l.ExpiresAt.After(now) {
			used[l.IP] = true
		}
	}
	ip := cloneIP(start4)
	for {
		if !used[ip.String()] {
			return cloneIP(ip)
		}
		// Stop before incrementing past end — incrementIP wraps 255.255.255.255
		// back to 0.0.0.0 and ipAfter would never trigger, looping forever.
		if ipEqual(ip, end4) {
			return nil
		}
		incrementIP(ip)
	}
}

// IsAssigned returns the lease for the given IP if it is active (not expired).
func (s *LeaseStore) IsAssigned(ip string) (Lease, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	for _, l := range s.leases {
		if l.IP == ip && l.ExpiresAt.After(now) {
			return l, true
		}
	}
	return Lease{}, false
}

// Sweep removes expired leases and persists if anything changed.
func (s *LeaseStore) Sweep() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	changed := false
	for mac, l := range s.leases {
		if !l.ExpiresAt.After(now) {
			delete(s.leases, mac)
			changed = true
		}
	}
	if changed {
		_ = s.persist()
	}
}

// Clear removes all leases from memory and disk.
func (s *LeaseStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.leases = make(map[string]Lease)
	return s.persist()
}

// load reads leases from disk. Missing or empty file returns os.ErrNotExist
// (caller in NewLeaseStore swallows that — empty store is a valid state).
func (s *LeaseStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return os.ErrNotExist
	}
	var list []Lease
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	for _, l := range list {
		key := l.MAC
		if l.ClientID != "" {
			key = "id:" + l.ClientID
		}
		s.leases[key] = l
	}
	return nil
}

// persist writes all leases to disk atomically. Caller must hold s.mu.
func (s *LeaseStore) persist() error {
	list := make([]Lease, 0, len(s.leases))
	for _, l := range s.leases {
		list = append(list, l)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(s.path, data)
}

// atomicWriteFile writes data to path via a temp file + rename to avoid
// leaving a truncated file if the process is killed mid-write.
func atomicWriteFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// GetByClient returns the active (non-expired) lease for the given client key.
func (s *LeaseStore) GetByClient(clientKey string) (Lease, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.leases[clientKey]
	if !ok || time.Now().After(l.ExpiresAt) {
		return Lease{}, false
	}
	return l, true
}

func cloneIP(ip net.IP) net.IP {
	c := make(net.IP, len(ip))
	copy(c, ip)
	return c
}

func incrementIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}

// ipAfter returns true if a > b (byte-wise comparison). Caller must ensure
// both slices are the same length (use .To4() before passing).
func ipAfter(a, b net.IP) bool {
	for i := range a {
		if a[i] > b[i] {
			return true
		}
		if a[i] < b[i] {
			return false
		}
	}
	return false
}

// ipEqual reports whether a and b are byte-wise equal. Caller must ensure
// both slices are the same length.
func ipEqual(a, b net.IP) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
