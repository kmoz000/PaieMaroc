package pointerservice

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGenerateIdentityAndValidateUserspaceConfig(t *testing.T) {
	t.Parallel()
	identity, err := GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}
	peer, err := GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}
	config := DefaultConfig()
	config.WireGuard = WireGuardConfig{
		Enabled: true, Address: "10.77.0.2/32", PrivateKey: identity.PrivateKey,
		PeerPublicKey: peer.PublicKey, Endpoint: "vpn.example.test:51820",
		AllowedIPs: []string{"10.90.0.0/24"}, PersistentKeepaliveSeconds: 25, MTU: 1420,
	}
	validated, err := ValidateConfig(config)
	if err != nil {
		t.Fatalf("ValidateConfig() error = %v", err)
	}
	public := Public(validated)
	if !public.WireGuard.HasPrivateKey || public.WireGuard.PublicKey != identity.PublicKey {
		t.Fatalf("Public() = %+v", public.WireGuard)
	}
	if public.WireGuard.PublicKey == public.WireGuard.PeerPublicKey {
		t.Fatal("local and peer keys unexpectedly match")
	}
}

func TestValidateConfigRejectsDefaultRouteAndMissingConsent(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.Hook.Enabled = true
	if _, err := ValidateConfig(config); err == nil {
		t.Fatal("enabled hook without consent was accepted")
	}
	config.Hook.FirewallConsent = true
	identity, _ := GenerateIdentity()
	peer, _ := GenerateIdentity()
	config.WireGuard = WireGuardConfig{
		Enabled: true, Address: "10.77.0.2/32", PrivateKey: identity.PrivateKey,
		PeerPublicKey: peer.PublicKey, Endpoint: "vpn.example.test:51820",
		AllowedIPs: []string{"0.0.0.0/0"}, MTU: 1420,
	}
	if _, err := ValidateConfig(config); err == nil {
		t.Fatal("default WireGuard route was accepted")
	}
}

func TestConfigPersistsSecretsWithRestrictedPermissions(t *testing.T) {
	t.Parallel()
	identity, _ := GenerateIdentity()
	peer, _ := GenerateIdentity()
	config := DefaultConfig()
	config.WireGuard = WireGuardConfig{
		Enabled: true, Address: "10.77.0.2/32", PrivateKey: identity.PrivateKey,
		PeerPublicKey: peer.PublicKey, Endpoint: "127.0.0.1:51820",
		AllowedIPs: []string{"10.90.0.0/24"}, MTU: 1420,
	}
	path := filepath.Join(t.TempDir(), "pointer-connectivity.json")
	if err := SaveConfig(path, config); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.WireGuard.PrivateKey != identity.PrivateKey {
		t.Fatal("private key was not persisted")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("config permissions = %o", info.Mode().Perm())
	}
}

func TestManagerAllowsOnlyConfiguredPointerSubnets(t *testing.T) {
	t.Parallel()
	manager := NewManager(func(context.Context, string) (PunchSession, error) { return nil, nil })
	manager.config.WireGuard.AllowedIPs = []string{"10.90.0.0/24"}
	if !manager.AllowsAddress("10.90.0.42") {
		t.Fatal("configured pointer address was rejected")
	}
	if manager.AllowsAddress("10.91.0.42") || manager.AllowsAddress("pointeuse.local") {
		t.Fatal("address outside the pointer subnet was accepted")
	}
}
