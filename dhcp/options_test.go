package dhcp

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestEncodeCustomOption_IP(t *testing.T) {
	b, err := EncodeCustomOption(CustomOption{Code: 100, Type: "ip", Value: "192.168.1.1"})
	if err != nil {
		t.Fatal(err)
	}
	if !net.IP(b).Equal(net.ParseIP("192.168.1.1").To4()) {
		t.Fatalf("got %v", b)
	}
}

func TestEncodeCustomOption_IP_Invalid(t *testing.T) {
	_, err := EncodeCustomOption(CustomOption{Code: 100, Type: "ip", Value: "not-an-ip"})
	if err == nil {
		t.Fatal("expected error for invalid IP")
	}
}

func TestEncodeCustomOption_String(t *testing.T) {
	b, err := EncodeCustomOption(CustomOption{Code: 100, Type: "string", Value: "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "example.com" {
		t.Fatalf("got %q", string(b))
	}
}

func TestEncodeCustomOption_Uint8(t *testing.T) {
	b, err := EncodeCustomOption(CustomOption{Code: 100, Type: "uint8", Value: "42"})
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 1 || b[0] != 42 {
		t.Fatalf("got %v", b)
	}
}

func TestEncodeCustomOption_Uint16(t *testing.T) {
	b, err := EncodeCustomOption(CustomOption{Code: 100, Type: "uint16", Value: "1000"})
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 2 || binary.BigEndian.Uint16(b) != 1000 {
		t.Fatalf("got %v", b)
	}
}

func TestEncodeCustomOption_Uint32(t *testing.T) {
	b, err := EncodeCustomOption(CustomOption{Code: 100, Type: "uint32", Value: "70000"})
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 4 || binary.BigEndian.Uint32(b) != 70000 {
		t.Fatalf("got %v", b)
	}
}

func TestEncodeCustomOption_Bool_True(t *testing.T) {
	b, err := EncodeCustomOption(CustomOption{Code: 100, Type: "bool", Value: "true"})
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 1 || b[0] != 0x01 {
		t.Fatalf("got %v", b)
	}
}

func TestEncodeCustomOption_Bool_False(t *testing.T) {
	b, err := EncodeCustomOption(CustomOption{Code: 100, Type: "bool", Value: "false"})
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 1 || b[0] != 0x00 {
		t.Fatalf("got %v", b)
	}
}

func TestEncodeCustomOption_UnknownType(t *testing.T) {
	_, err := EncodeCustomOption(CustomOption{Code: 100, Type: "hex", Value: "0xAB"})
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestEncodeDNSName(t *testing.T) {
	got := encodeDNSName("foo.bar.com")
	// \x03foo\x03bar\x03com\x00
	want := []byte{3, 'f', 'o', 'o', 3, 'b', 'a', 'r', 3, 'c', 'o', 'm', 0}
	if string(got) != string(want) {
		t.Errorf("encodeDNSName = %v, want %v", got, want)
	}
}

func TestBuildOptions_DomainSearch(t *testing.T) {
	cfg := DHCPConfig{DomainSearch: []string{"foo.local", "bar.local"}}
	mods, err := BuildOptions(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 1 {
		t.Fatalf("expected 1 modifier for option 119, got %d", len(mods))
	}
}

func TestBuildOptions_TFTPServer(t *testing.T) {
	cfg := DHCPConfig{TFTPServer: "192.168.1.1"}
	mods, err := BuildOptions(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 1 {
		t.Fatalf("expected 1 modifier for option 150, got %d", len(mods))
	}
}

func TestBuildOptions_TFTPServer_Invalid(t *testing.T) {
	cfg := DHCPConfig{TFTPServer: "not-an-ip"}
	_, err := BuildOptions(cfg, nil)
	if err == nil {
		t.Fatal("expected error for invalid TFTPServer")
	}
}

func TestBuildOptions_CustomOverridesStandard(t *testing.T) {
	cfg := DHCPConfig{
		Router: "10.0.0.1",
		Options: []CustomOption{
			{Code: 3, Type: "ip", Value: "10.0.0.254"}, // option 3 = router
		},
	}
	mods, err := BuildOptions(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Should have exactly 1 router modifier (custom), not 2
	if len(mods) != 1 {
		t.Fatalf("expected 1 modifier (custom router only), got %d", len(mods))
	}
}
