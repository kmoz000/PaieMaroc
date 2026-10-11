package pointerservice

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

// userspaceTunnel owns a WireGuard device backed by gVisor netstack. It never
// creates a kernel interface and therefore cannot alter host routes or DNS.
type userspaceTunnel struct {
	mu     sync.RWMutex
	device *device.Device
	net    *netstack.Net
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
	fmt.Fprintf(&ipc, "endpoint=%s\npersistent_keepalive_interval=%d\n", config.Endpoint, config.PersistentKeepaliveSeconds)
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
	return &userspaceTunnel{device: wgDevice, net: tunnelNet}, nil
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
}
