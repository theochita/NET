// dhcpdebug — headless DHCP server for live debugging without the Wails GUI.
// Usage: sudo ./dhcpdebug -iface virbr0 -start 192.168.122.100 -end 192.168.122.200
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/theochita/NET/dhcp"
)

func main() {
	iface := flag.String("iface", "virbr0", "interface to bind")
	start := flag.String("start", "192.168.122.100", "pool start IP")
	end := flag.String("end", "192.168.122.200", "pool end IP")
	router := flag.String("router", "192.168.122.1", "default gateway to advertise")
	mask := flag.String("mask", "255.255.255.0", "subnet mask to advertise")
	dns := flag.String("dns", "8.8.8.8", "DNS server to advertise")
	flag.Parse()

	dir, err := os.MkdirTemp("", "dhcpdebug-*")
	if err != nil {
		log.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(dir)

	srv, err := dhcp.NewServer(dir)
	if err != nil {
		log.Fatalf("new server: %v", err)
	}

	cfg := dhcp.DHCPConfig{
		Interface: *iface,
		PoolStart: *start,
		PoolEnd:   *end,
		Router:    *router,
		Mask:      *mask,
		DNS:       []string{*dns},
		LeaseTime: 3600,
	}
	if err := srv.SaveConfig(cfg); err != nil {
		log.Fatalf("save config: %v", err)
	}

	log.Printf("Starting DHCP server on %s  pool %s – %s", *iface, *start, *end)
	if err := srv.Start(); err != nil {
		log.Fatalf("start: %v", err)
	}
	log.Printf("Listening on UDP :67 — send DHCPDISCOVER to test")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("Stopping…")
	srv.Stop()
}
