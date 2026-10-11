package pointerservice

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"time"
)

type Status struct {
	CollectorActive  bool         `json:"collectorActive"`
	WireGuardActive  bool         `json:"wireGuardActive"`
	LANAddress       string       `json:"lanAddress"`
	TunnelAddress    string       `json:"tunnelAddress"`
	FirewallGuidance string       `json:"firewallGuidance"`
	LastError        string       `json:"lastError"`
	Config           PublicConfig `json:"config"`
}

type Manager struct {
	mu        sync.RWMutex
	config    Config
	begin     BeginPunchSession
	tunnel    *userspaceTunnel
	servers   []*http.Server
	listeners []net.Listener
	wg        sync.WaitGroup
	lastError string
}

func NewManager(begin BeginPunchSession) *Manager {
	return &Manager{config: DefaultConfig(), begin: begin}
}

func (manager *Manager) Apply(config Config) error {
	validated, err := ValidateConfig(config)
	if err != nil {
		return err
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.stopLocked()
	manager.config = validated
	manager.lastError = ""
	if validated.WireGuard.Enabled {
		manager.tunnel, err = newUserspaceTunnel(validated.WireGuard)
		if err != nil {
			manager.lastError = err.Error()
			manager.stopLocked()
			return err
		}
	}
	if validated.Hook.Enabled {
		if err := manager.startReceiverLocked(); err != nil {
			manager.lastError = err.Error()
			manager.stopLocked()
			return err
		}
	}
	return nil
}

func (manager *Manager) CurrentConfig() Config {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return cloneConfig(manager.config)
}

func (manager *Manager) Status() Status {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	status := Status{
		WireGuardActive:  manager.tunnel != nil,
		FirewallGuidance: FirewallGuidance(),
		LastError:        manager.lastError,
		Config:           Public(manager.config),
	}
	if manager.config.Hook.Enabled && len(manager.listeners) > 0 {
		status.CollectorActive = true
		status.LANAddress = net.JoinHostPort(manager.config.Hook.BindAddress, strconv.Itoa(manager.config.Hook.Port))
	}
	if manager.tunnel != nil {
		if prefix, err := netip.ParsePrefix(manager.config.WireGuard.Address); err == nil {
			status.TunnelAddress = net.JoinHostPort(prefix.Addr().String(), strconv.Itoa(manager.config.Hook.Port))
		}
	}
	return status
}

func (manager *Manager) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	manager.mu.RLock()
	tunnel := manager.tunnel
	manager.mu.RUnlock()
	if tunnel == nil {
		return nil, errors.New("activez d’abord le tunnel WireGuard des pointeuses")
	}
	return tunnel.DialContext(ctx, network, address)
}

func (manager *Manager) AllowsAddress(value string) bool {
	address, err := netip.ParseAddr(value)
	if err != nil {
		return false
	}
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	for _, value := range manager.config.WireGuard.AllowedIPs {
		prefix, prefixErr := netip.ParsePrefix(value)
		if prefixErr == nil && prefix.Contains(address) {
			return true
		}
	}
	return false
}

func (manager *Manager) Close() {
	manager.mu.Lock()
	manager.stopLocked()
	manager.mu.Unlock()
	manager.wg.Wait()
}

func (manager *Manager) startReceiverLocked() error {
	networks := append([]string{}, manager.config.Hook.AllowedNetworks...)
	if manager.config.WireGuard.Enabled {
		networks = append(networks, manager.config.WireGuard.AllowedIPs...)
	}
	handler, err := newReceiver(networks, manager.begin)
	if err != nil {
		return err
	}
	lanAddress := net.JoinHostPort(manager.config.Hook.BindAddress, strconv.Itoa(manager.config.Hook.Port))
	lanListener, err := net.Listen("tcp", lanAddress)
	if err != nil {
		return fmt.Errorf("ouverture du collecteur LAN %s: %w", lanAddress, err)
	}
	manager.serveLocked(lanListener, handler)
	if manager.tunnel == nil {
		return nil
	}
	prefix, err := netip.ParsePrefix(manager.config.WireGuard.Address)
	if err != nil {
		return fmt.Errorf("adresse WireGuard du collecteur: %w", err)
	}
	tunnelListener, err := manager.tunnel.listenTCP(prefix.Addr(), manager.config.Hook.Port)
	if err != nil {
		return fmt.Errorf("ouverture du collecteur dans WireGuard: %w", err)
	}
	manager.serveLocked(tunnelListener, handler)
	return nil
}

func (manager *Manager) serveLocked(listener net.Listener, handler http.Handler) {
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	manager.listeners = append(manager.listeners, listener)
	manager.servers = append(manager.servers, server)
	manager.wg.Add(1)
	go func() {
		defer manager.wg.Done()
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
			manager.mu.Lock()
			manager.lastError = fmt.Sprintf("arrêt du collecteur de pointeuses: %v", err)
			manager.mu.Unlock()
		}
	}()
}

func (manager *Manager) stopLocked() {
	for _, server := range manager.servers {
		_ = server.Close()
	}
	for _, listener := range manager.listeners {
		_ = listener.Close()
	}
	manager.servers = nil
	manager.listeners = nil
	if manager.tunnel != nil {
		manager.tunnel.Close()
		manager.tunnel = nil
	}
}

func cloneConfig(config Config) Config {
	config.Hook.AllowedNetworks = append([]string{}, config.Hook.AllowedNetworks...)
	config.WireGuard.AllowedIPs = append([]string{}, config.WireGuard.AllowedIPs...)
	return config
}
