package tftp

import "time"

// No JSON tags: Wails serialises field names as-is (PascalCase).
// Vue accesses them as cfg.Root, cfg.WriteEnabled, etc.

// TFTPConfig is the persisted server configuration.
type TFTPConfig struct {
	Interface    string // "" = all interfaces
	Root         string // absolute path to serve root
	ReadEnabled  bool   // default true
	WriteEnabled bool   // default true
	BlockSize    uint16 // 0 = negotiate, else override (e.g. 1468)
}

// Transfer describes a single in-flight or completed TFTP transfer.
// Emitted to the frontend via the tftp:transfer event and returned from
// GetActiveTransfers / GetTransferHistory.
type Transfer struct {
	ID        string    // uuid, unique per transfer
	Peer      string    // ip:port
	Filename  string    // relative to Root
	Direction string    // "read" | "write"
	Bytes     int64
	Size      int64 // from tsize option, 0 if unknown
	StartedAt time.Time
	EndedAt   time.Time // zero while in progress
	Status    string    // "active" | "ok" | "error"
	Error     string    // empty unless Status == "error"
}
