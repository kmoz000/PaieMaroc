package pointerservice

import (
	"context"
	"net"
	"testing"
)

func TestManagerStartsAndStopsLocalCollector(t *testing.T) {
	t.Parallel()
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(func(context.Context, string) (PunchSession, error) {
		return &recordingSession{}, nil
	})
	config := DefaultConfig()
	config.Hook.Enabled = true
	config.Hook.FirewallConsent = true
	config.Hook.BindAddress = "127.0.0.1"
	config.Hook.Port = port
	config.Hook.AllowedNetworks = []string{"192.168.0.0/16"}
	if err := manager.Apply(config); err != nil {
		t.Fatal(err)
	}
	if status := manager.Status(); !status.CollectorActive || status.LANAddress == "" {
		t.Fatalf("status = %+v", status)
	}
	manager.Close()
	if status := manager.Status(); status.CollectorActive || status.WireGuardActive {
		t.Fatalf("status after close = %+v", status)
	}
}
