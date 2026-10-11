package pointerservice

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

// userspaceTunnel owns a WireGuard device backed by gVisor netstack. It never
// creates a kernel interface and therefore cannot alter host routes or DNS.
type userspaceTunnel struct {
	mu       sync.RWMutex
	device   *device.Device
	net      *netstack.Net
	endpoint string
}

const endpointResolutionTimeout = 5 * time.Second

type endpointResolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

func newUserspaceTunnel(config WireGuardConfig) (*userspaceTunnel, error) {
	address, err := netip.ParsePrefix(config.Address)
	if err != nil {
		return nil, fmt.Errorf("adresse WireGuard locale: %w", err)
	}
	tunDevice, tunnelNet, err := netstack.CreateNetTUN([]netip.Addr{address.Addr()}, nil, config.MTU)
	if err != nil {
		return nil, fmt.Errorf("création du réseau WireGuard en espace utilisateur: %w", err)
	}
	wgDevice := device.NewDevice(tunDevice, conn.NewDefaultBind(), device.NewLogger(device.LogLevelSilent, "PaieMaroc"))
	privateKey, err := keyHex(config.PrivateKey, "clé privée WireGuard")
	if err != nil {
		wgDevice.Close()
		return nil, err
	}
	peerKey, err := keyHex(config.PeerPublicKey, "clé publique de la passerelle")
	if err != nil {
		wgDevice.Close()
		return nil, err
	}
	resolutionContext, cancelResolution := context.WithTimeout(context.Background(), endpointResolutionTimeout)
	resolvedEndpoint, err := resolveWireGuardEndpoint(resolutionContext, config.Endpoint, net.DefaultResolver)
	cancelResolution()
	if err != nil {
		wgDevice.Close()
		return nil, err
	}
	var ipc strings.Builder
	fmt.Fprintf(&ipc, "private_key=%s\nreplace_peers=true\npublic_key=%s\n", privateKey, peerKey)
	if config.PresharedKey != "" {
		presharedKey, keyErr := keyHex(config.PresharedKey, "clé prépartagée WireGuard")
		if keyErr != nil {
			wgDevice.Close()
			return nil, keyErr
		}
		fmt.Fprintf(&ipc, "preshared_key=%s\n", presharedKey)
	}
	fmt.Fprintf(&ipc, "endpoint=%s\npersistent_keepalive_interval=%d\n", resolvedEndpoint, config.PersistentKeepaliveSeconds)
	for _, allowedIP := range config.AllowedIPs {
		fmt.Fprintf(&ipc, "allowed_ip=%s\n", allowedIP)
	}
	if err := wgDevice.IpcSet(ipc.String()); err != nil {
		wgDevice.Close()
		return nil, fmt.Errorf("configuration WireGuard: %w", err)
	}
	if err := wgDevice.Up(); err != nil {
		wgDevice.Close()
		return nil, fmt.Errorf("activation WireGuard: %w", err)
	}
	return &userspaceTunnel{device: wgDevice, net: tunnelNet, endpoint: resolvedEndpoint}, nil
}

func resolveWireGuardEndpoint(ctx context.Context, endpoint string, resolver endpointResolver) (string, error) {
	host, port, err := validateWireGuardEndpoint(endpoint)
	if err != nil {
		return "", err
	}
	if address, parseErr := netip.ParseAddr(host); parseErr == nil {
		return net.JoinHostPort(address.String(), port), nil
	}
	addresses, err := resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return "", fmt.Errorf("résolution DNS du point de terminaison WireGuard %q: %w", host, err)
	}
	for _, address := range addresses {
		address = address.Unmap()
		if address.IsValid() && !address.IsUnspecified() && !address.IsMulticast() {
			return net.JoinHostPort(address.String(), port), nil
		}
	}
	return "", fmt.Errorf("résolution DNS du point de terminaison WireGuard %q: aucune adresse IP utilisable", host)
}

func (tunnel *userspaceTunnel) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	tunnel.mu.RLock()
	tunnelNet := tunnel.net
	tunnel.mu.RUnlock()
	if tunnelNet == nil {
		return nil, fmt.Errorf("le tunnel WireGuard des pointeuses n’est pas actif")
	}
	connection, err := tunnelNet.DialContext(ctx, network, address)
	if err != nil {
		return nil, fmt.Errorf("connexion via le tunnel WireGuard: %w", err)
	}
	return connection, nil
}

type tunnelStats struct {
	Endpoint      string
	LastHandshake time.Time
	ReceivedBytes uint64
	SentBytes     uint64
}

func (tunnel *userspaceTunnel) stats() (tunnelStats, error) {
	tunnel.mu.RLock()
	defer tunnel.mu.RUnlock()
	if tunnel.device == nil {
		return tunnelStats{}, errors.New("le tunnel WireGuard des pointeuses n’est pas actif")
	}
	state, err := tunnel.device.IpcGet()
	if err != nil {
		return tunnelStats{}, fmt.Errorf("lecture de l’état WireGuard: %w", err)
	}
	stats, err := parseTunnelStats(state)
	if err != nil {
		return tunnelStats{}, err
	}
	if stats.Endpoint == "" {
		stats.Endpoint = tunnel.endpoint
	}
	return stats, nil
}

func parseTunnelStats(state string) (tunnelStats, error) {
	var stats tunnelStats
	var handshakeSeconds int64
	scanner := bufio.NewScanner(strings.NewReader(state))
	for scanner.Scan() {
		name, value, found := strings.Cut(scanner.Text(), "=")
		if !found {
			continue
		}
		switch name {
		case "endpoint":
			stats.Endpoint = value
		case "last_handshake_time_sec":
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return tunnelStats{}, fmt.Errorf("lecture de la date du handshake WireGuard: %w", err)
			}
			handshakeSeconds = parsed
		case "rx_bytes":
			parsed, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return tunnelStats{}, fmt.Errorf("lecture des octets WireGuard reçus: %w", err)
			}
			stats.ReceivedBytes = parsed
		case "tx_bytes":
			parsed, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return tunnelStats{}, fmt.Errorf("lecture des octets WireGuard envoyés: %w", err)
			}
			stats.SentBytes = parsed
		}
	}
	if err := scanner.Err(); err != nil {
		return tunnelStats{}, fmt.Errorf("lecture de l’état WireGuard: %w", err)
	}
	if handshakeSeconds > 0 {
		stats.LastHandshake = time.Unix(handshakeSeconds, 0).UTC()
	}
	return stats, nil
}

func (tunnel *userspaceTunnel) ping(ctx context.Context, target netip.Addr) (latency time.Duration, err error) {
	tunnel.mu.RLock()
	tunnelNet := tunnel.net
	tunnel.mu.RUnlock()
	if tunnelNet == nil {
		return 0, errors.New("le tunnel WireGuard des pointeuses n’est pas actif")
	}
	connection, err := tunnelNet.DialPingAddr(netip.Addr{}, target)
	if err != nil {
		return 0, fmt.Errorf("préparation du ping WireGuard: %w", err)
	}
	defer func() {
		if closeErr := connection.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("fermeture du ping WireGuard: %w", closeErr)
		}
	}()
	deadline := time.Now().Add(7 * time.Second)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err := connection.SetReadDeadline(deadline); err != nil {
		return 0, fmt.Errorf("délai du ping WireGuard: %w", err)
	}
	select {
	case <-ctx.Done():
		return 0, fmt.Errorf("ping WireGuard annulé: %w", ctx.Err())
	default:
	}
	random := make([]byte, 18)
	if _, err := rand.Read(random); err != nil {
		return 0, fmt.Errorf("préparation sécurisée du ping WireGuard: %w", err)
	}
	echo := &icmp.Echo{ID: int(binary.BigEndian.Uint16(random[:2])), Seq: 1, Data: random[2:]}
	messageType := icmp.Type(ipv4.ICMPTypeEcho)
	protocol := 1
	if target.Is6() {
		messageType = ipv6.ICMPTypeEchoRequest
		protocol = 58
	}
	packet, err := (&icmp.Message{Type: messageType, Code: 0, Body: echo}).Marshal(nil)
	if err != nil {
		return 0, fmt.Errorf("encodage du ping WireGuard: %w", err)
	}
	started := time.Now()
	if _, err := connection.Write(packet); err != nil {
		return 0, fmt.Errorf("envoi du ping WireGuard: %w", err)
	}
	replyBuffer := make([]byte, 1500)
	received, err := connection.Read(replyBuffer)
	if err != nil {
		if ctx.Err() != nil {
			return 0, fmt.Errorf("ping WireGuard annulé: %w", ctx.Err())
		}
		return 0, fmt.Errorf("aucune réponse ICMP de %s: %w", target, err)
	}
	latency = time.Since(started)
	reply, err := icmp.ParseMessage(protocol, replyBuffer[:received])
	if err != nil {
		return 0, fmt.Errorf("décodage de la réponse ICMP de %s: %w", target, err)
	}
	replyEcho, ok := reply.Body.(*icmp.Echo)
	if !ok || replyEcho.ID != echo.ID || replyEcho.Seq != echo.Seq || !bytes.Equal(replyEcho.Data, echo.Data) {
		return 0, fmt.Errorf("réponse ICMP inattendue de %s", target)
	}
	return latency, nil
}

func (tunnel *userspaceTunnel) listenTCP(address netip.Addr, port int) (net.Listener, error) {
	tunnel.mu.RLock()
	tunnelNet := tunnel.net
	tunnel.mu.RUnlock()
	if tunnelNet == nil {
		return nil, fmt.Errorf("le tunnel WireGuard des pointeuses n’est pas actif")
	}
	return tunnelNet.ListenTCP(&net.TCPAddr{IP: address.AsSlice(), Port: port})
}

func (tunnel *userspaceTunnel) Close() {
	tunnel.mu.Lock()
	defer tunnel.mu.Unlock()
	if tunnel.device != nil {
		tunnel.device.Close()
	}
	tunnel.device = nil
	tunnel.net = nil
	tunnel.endpoint = ""
}
