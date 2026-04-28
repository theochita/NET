package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/theochita/NET/dhcp"
)

func main() {
	configDir := flag.String("config-dir", "/tmp/net", "directory for config and lease files")
	iface := flag.String("interface", "", "network interface to bind (auto-detected if empty)")
	flag.Parse()

	if *iface == "" {
		detected, err := firstNonLoopback()
		if err != nil {
			log.Fatalf("interface detection: %v", err)
		}
		*iface = detected
	}
	log.Printf("using interface %s", *iface)

	server, err := dhcp.NewServer(*configDir)
	if err != nil {
		log.Fatalf("new server: %v", err)
	}

	cfg := dhcp.DHCPConfig{
		Interface:    *iface,
		PoolStart:    "172.99.0.100",
		PoolEnd:      "172.99.0.111",
		Router:       "172.99.0.1",
		Mask:         "255.255.255.0",
		DNS:          []string{"8.8.8.8", "8.8.4.4"},
		NTP:          []string{"172.99.0.1"},
		WINS:         []string{"172.99.0.1"},
		DomainName:   "test.local",
		DomainSearch: []string{"test.local", "local"},
		BootFile:     "pxelinux.0",
		TFTPServer:   "172.99.0.1",
		LeaseTime:    3600,
		Options: []dhcp.CustomOption{
			{Code: 200, Type: "ip",     Value: "10.10.10.10"},
			{Code: 201, Type: "string", Value: "hello"},
			{Code: 202, Type: "uint8",  Value: "42"},
			{Code: 203, Type: "uint16", Value: "1234"},
			{Code: 204, Type: "uint32", Value: "98765"},
			{Code: 205, Type: "bool",   Value: "true"},
		},
	}
	if err := server.SaveConfig(cfg); err != nil {
		log.Fatalf("save config: %v", err)
	}

	if err := server.Start(); err != nil {
		log.Fatalf("start: %v", err)
	}
	log.Printf("DHCP server ready on %s:67", *iface)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	<-sig
	server.Stop()
}

func firstNonLoopback() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				return iface.Name, nil
			}
		}
	}
	return "", fmt.Errorf("no suitable interface found")
}
