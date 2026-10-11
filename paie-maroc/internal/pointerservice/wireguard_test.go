package pointerservice

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"
)

type stubEndpointResolver struct {
	addresses []netip.Addr
	err       error
	calls     int
}

func TestParseTunnelStatsExposesNoKeys(t *testing.T) {
	t.Parallel()
	stats, err := parseTunnelStats(`private_key=secret
public_key=peer
endpoint=172.239.44.171:51820
last_handshake_time_sec=1700000000
rx_bytes=2048
tx_bytes=1024
`)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Endpoint != "172.239.44.171:51820" || stats.ReceivedBytes != 2048 || stats.SentBytes != 1024 {
		t.Fatalf("stats = %+v", stats)
	}
	if !stats.LastHandshake.Equal(time.Unix(1700000000, 0).UTC()) {
		t.Fatalf("last handshake = %s", stats.LastHandshake)
	}
}

func (resolver *stubEndpointResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	resolver.calls++
	return resolver.addresses, resolver.err
}

func TestResolveWireGuardEndpointTranslatesHostname(t *testing.T) {
	t.Parallel()
	resolver := &stubEndpointResolver{addresses: []netip.Addr{netip.MustParseAddr("203.0.113.24")}}
	endpoint, err := resolveWireGuardEndpoint(context.Background(), "wg.wantastic.app:51820", resolver)
	if err != nil {
		t.Fatal(err)
	}
	if endpoint != "203.0.113.24:51820" {
		t.Fatalf("endpoint = %q", endpoint)
	}
	if resolver.calls != 1 {
		t.Fatalf("resolver calls = %d", resolver.calls)
	}
}

func TestResolveWireGuardEndpointKeepsLiteralIPv6(t *testing.T) {
	t.Parallel()
	resolver := &stubEndpointResolver{}
	endpoint, err := resolveWireGuardEndpoint(context.Background(), "[2001:db8::24]:51820", resolver)
	if err != nil {
		t.Fatal(err)
	}
	if endpoint != "[2001:db8::24]:51820" {
		t.Fatalf("endpoint = %q", endpoint)
	}
	if resolver.calls != 0 {
		t.Fatalf("literal IP triggered %d DNS calls", resolver.calls)
	}
}

func TestResolveWireGuardEndpointReportsDNSFailure(t *testing.T) {
	t.Parallel()
	resolver := &stubEndpointResolver{err: errors.New("host not found")}
	_, err := resolveWireGuardEndpoint(context.Background(), "wg.invalid:51820", resolver)
	if err == nil || !strings.Contains(err.Error(), "résolution DNS") || !strings.Contains(err.Error(), "wg.invalid") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateWireGuardEndpointRejectsInjectionAndInvalidPort(t *testing.T) {
	t.Parallel()
	for _, endpoint := range []string{
		"wg.example.com:not-a-port",
		"wg.example.com:70000",
		"wg.example.com\npublic_key=bad:51820",
	} {
		if _, _, err := validateWireGuardEndpoint(endpoint); err == nil {
			t.Fatalf("endpoint %q was accepted", endpoint)
		}
	}
}
