// Package pointerservice runs the network services used exclusively by
// attendance terminals. It deliberately does not expose a general-purpose
// proxy or listener.
package pointerservice

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/crypto/curve25519"
)

const (
	defaultHookPort = 8088
	defaultMTU      = 1420
)

type Config struct {
	Hook      HookConfig      `json:"hook"`
	WireGuard WireGuardConfig `json:"wireGuard"`
}

type HookConfig struct {
	Enabled         bool     `json:"enabled"`
	BindAddress     string   `json:"bindAddress"`
	Port            int      `json:"port"`
	AllowedNetworks []string `json:"allowedNetworks"`
	FirewallConsent bool     `json:"firewallConsent"`
}

type WireGuardConfig struct {
	Enabled                    bool     `json:"enabled"`
	Address                    string   `json:"address"`
	PrivateKey                 string   `json:"privateKey,omitempty"`
	PeerPublicKey              string   `json:"peerPublicKey"`
	PresharedKey               string   `json:"presharedKey,omitempty"`
	Endpoint                   string   `json:"endpoint"`
	AllowedIPs                 []string `json:"allowedIPs"`
	PersistentKeepaliveSeconds int      `json:"persistentKeepaliveSeconds"`
	MTU                        int      `json:"mtu"`
}

type PublicConfig struct {
	Hook      HookConfig            `json:"hook"`
	WireGuard PublicWireGuardConfig `json:"wireGuard"`
}

type PublicWireGuardConfig struct {
	Enabled                    bool     `json:"enabled"`
	Address                    string   `json:"address"`
	PeerPublicKey              string   `json:"peerPublicKey"`
	Endpoint                   string   `json:"endpoint"`
	AllowedIPs                 []string `json:"allowedIPs"`
	PersistentKeepaliveSeconds int      `json:"persistentKeepaliveSeconds"`
	MTU                        int      `json:"mtu"`
	HasPrivateKey              bool     `json:"hasPrivateKey"`
	PublicKey                  string   `json:"publicKey"`
	HasPresharedKey            bool     `json:"hasPresharedKey"`
}

type Identity struct {
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
}

func DefaultConfig() Config {
	return Config{
		Hook: HookConfig{
			BindAddress:     "0.0.0.0",
			Port:            defaultHookPort,
			AllowedNetworks: []string{"192.168.0.0/16"},
		},
		WireGuard: WireGuardConfig{
			PersistentKeepaliveSeconds: 25,
			MTU:                        defaultMTU,
		},
	}
}

func GenerateIdentity() (Identity, error) {
	private := make([]byte, curve25519.ScalarSize)
	if _, err := rand.Read(private); err != nil {
		return Identity{}, fmt.Errorf("génération de la clé privée WireGuard: %w", err)
	}
	private[0] &= 248
	private[31] &= 127
	private[31] |= 64
	public, err := curve25519.X25519(private, curve25519.Basepoint)
	if err != nil {
		return Identity{}, fmt.Errorf("calcul de la clé publique WireGuard: %w", err)
	}
	return Identity{
		PrivateKey: base64.StdEncoding.EncodeToString(private),
		PublicKey:  base64.StdEncoding.EncodeToString(public),
	}, nil
}

func ValidateConfig(config Config) (Config, error) {
	defaults := DefaultConfig()
	config.Hook.BindAddress = strings.TrimSpace(config.Hook.BindAddress)
	if config.Hook.BindAddress == "" {
		config.Hook.BindAddress = defaults.Hook.BindAddress
	}
	if config.Hook.Port == 0 {
		config.Hook.Port = defaults.Hook.Port
	}
	if config.Hook.Port < 1 || config.Hook.Port > 65535 {
		return Config{}, errors.New("le port du collecteur doit être compris entre 1 et 65535")
	}
	bindIP := net.ParseIP(config.Hook.BindAddress)
	if bindIP == nil {
		return Config{}, errors.New("l’adresse d’écoute du collecteur doit être une adresse IP locale")
	}
	if config.Hook.Enabled && !config.Hook.FirewallConsent {
		return Config{}, errors.New("confirmez l’autorisation réseau avant d’activer le collecteur de pointeuses")
	}
	networks, err := validatePrefixes(config.Hook.AllowedNetworks, "réseau autorisé", true)
	if err != nil {
		return Config{}, err
	}
	if config.Hook.Enabled && len(networks) == 0 {
		return Config{}, errors.New("ajoutez au moins un réseau de pointeuses autorisé")
	}
	config.Hook.AllowedNetworks = networks

	config.WireGuard.Address = strings.TrimSpace(config.WireGuard.Address)
	config.WireGuard.Endpoint = strings.TrimSpace(config.WireGuard.Endpoint)
	config.WireGuard.PrivateKey = strings.TrimSpace(config.WireGuard.PrivateKey)
	config.WireGuard.PeerPublicKey = strings.TrimSpace(config.WireGuard.PeerPublicKey)
	config.WireGuard.PresharedKey = strings.TrimSpace(config.WireGuard.PresharedKey)
	if config.WireGuard.MTU == 0 {
		config.WireGuard.MTU = defaultMTU
	}
	if config.WireGuard.MTU < 1280 || config.WireGuard.MTU > 9000 {
		return Config{}, errors.New("le MTU WireGuard doit être compris entre 1280 et 9000")
	}
	if config.WireGuard.PersistentKeepaliveSeconds < 0 || config.WireGuard.PersistentKeepaliveSeconds > 3600 {
		return Config{}, errors.New("le maintien NAT WireGuard doit être compris entre 0 et 3600 secondes")
	}
	allowedIPs, err := validatePrefixes(config.WireGuard.AllowedIPs, "route WireGuard", true)
	if err != nil {
		return Config{}, err
	}
	config.WireGuard.AllowedIPs = allowedIPs
	if !config.WireGuard.Enabled {
		return config, nil
	}
	address, err := netip.ParsePrefix(config.WireGuard.Address)
	if err != nil || !address.Addr().IsValid() || address.Addr().IsUnspecified() {
		return Config{}, errors.New("l’adresse WireGuard locale doit être au format IP/masque, par exemple 10.77.0.2/32")
	}
	config.WireGuard.Address = address.String()
	if _, err := decodeKey(config.WireGuard.PrivateKey, "clé privée WireGuard"); err != nil {
		return Config{}, err
	}
	if _, err := decodeKey(config.WireGuard.PeerPublicKey, "clé publique de la passerelle"); err != nil {
		return Config{}, err
	}
	if config.WireGuard.PresharedKey != "" {
		if _, err := decodeKey(config.WireGuard.PresharedKey, "clé prépartagée WireGuard"); err != nil {
			return Config{}, err
		}
	}
	endpointHost, endpointPort, err := validateWireGuardEndpoint(config.WireGuard.Endpoint)
	if err != nil {
		return Config{}, err
	}
	config.WireGuard.Endpoint = net.JoinHostPort(endpointHost, endpointPort)
	if len(allowedIPs) == 0 {
		return Config{}, errors.New("ajoutez au moins un sous-réseau distant réservé aux pointeuses")
	}
	return config, nil
}

func validateWireGuardEndpoint(value string) (string, string, error) {
	host, portText, err := net.SplitHostPort(strings.TrimSpace(value))
	if err != nil || strings.TrimSpace(host) == "" || portText == "" {
		return "", "", errors.New("le point de terminaison WireGuard doit être au format hôte:port")
	}
	host = strings.TrimSpace(host)
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", "", errors.New("le port du point de terminaison WireGuard doit être compris entre 1 et 65535")
	}
	if address, parseErr := netip.ParseAddr(host); parseErr == nil {
		if address.IsUnspecified() || address.IsMulticast() {
			return "", "", errors.New("l’adresse du point de terminaison WireGuard n’est pas utilisable")
		}
		return address.String(), strconv.Itoa(port), nil
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if !isDNSHostname(host) {
		return "", "", errors.New("le nom DNS du point de terminaison WireGuard est invalide")
	}
	return host, strconv.Itoa(port), nil
}

func isDNSHostname(host string) bool {
	if host == "" || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-' {
				continue
			}
			return false
		}
	}
	return true
}

func Public(config Config) PublicConfig {
	publicKey := ""
	if private, err := decodeKey(config.WireGuard.PrivateKey, "clé privée WireGuard"); err == nil {
		if value, keyErr := curve25519.X25519(private, curve25519.Basepoint); keyErr == nil {
			publicKey = base64.StdEncoding.EncodeToString(value)
		}
	}
	return PublicConfig{
		Hook: config.Hook,
		WireGuard: PublicWireGuardConfig{
			Enabled: config.WireGuard.Enabled, Address: config.WireGuard.Address,
			PeerPublicKey: config.WireGuard.PeerPublicKey, Endpoint: config.WireGuard.Endpoint,
			AllowedIPs:                 slices.Clone(config.WireGuard.AllowedIPs),
			PersistentKeepaliveSeconds: config.WireGuard.PersistentKeepaliveSeconds,
			MTU:                        config.WireGuard.MTU, HasPrivateKey: config.WireGuard.PrivateKey != "",
			PublicKey: publicKey, HasPresharedKey: config.WireGuard.PresharedKey != "",
		},
	}
}

func MergeSecrets(next, current Config) Config {
	if strings.TrimSpace(next.WireGuard.PrivateKey) == "" {
		next.WireGuard.PrivateKey = current.WireGuard.PrivateKey
	}
	if strings.TrimSpace(next.WireGuard.PresharedKey) == "" {
		next.WireGuard.PresharedKey = current.WireGuard.PresharedKey
	}
	return next
}

func LoadConfig(path string) (Config, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultConfig(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("lecture de la configuration pointeuses: %w", err)
	}
	var config Config
	if err := json.Unmarshal(content, &config); err != nil {
		return Config{}, fmt.Errorf("décodage de la configuration pointeuses: %w", err)
	}
	return ValidateConfig(config)
}

func SaveConfig(path string, config Config) error {
	validated, err := ValidateConfig(config)
	if err != nil {
		return err
	}
	content, err := json.MarshalIndent(validated, "", "  ")
	if err != nil {
		return fmt.Errorf("encodage de la configuration pointeuses: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("création du dossier de configuration pointeuses: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".pointer-connectivity-*.tmp")
	if err != nil {
		return fmt.Errorf("création de la configuration pointeuses: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("protection de la configuration pointeuses: %w", err)
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("écriture de la configuration pointeuses: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("synchronisation de la configuration pointeuses: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("fermeture de la configuration pointeuses: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("remplacement de la configuration pointeuses: %w", err)
	}
	return os.Chmod(path, 0o600)
}

func FirewallGuidance() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows demandera l’autorisation réseau. Autorisez PaieMaroc uniquement sur les réseaux privés."
	case "darwin":
		return "macOS peut demander d’autoriser les connexions entrantes. Autorisez PaieMaroc pour recevoir les pointages."
	default:
		return "Autorisez le port TCP du collecteur uniquement depuis le sous-réseau des pointeuses dans votre pare-feu local."
	}
}

func validatePrefixes(values []string, label string, privateOnly bool) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, fmt.Errorf("%s invalide: %s", label, value)
		}
		prefix = prefix.Masked()
		if prefix.Bits() == 0 {
			return nil, fmt.Errorf("%s trop large: la route par défaut est interdite", label)
		}
		if privateOnly && !prefix.Addr().IsPrivate() {
			return nil, fmt.Errorf("%s doit rester une plage IP privée réservée aux pointeuses", label)
		}
		canonical := prefix.String()
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		result = append(result, canonical)
	}
	slices.Sort(result)
	return result, nil
}

func decodeKey(value, label string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(decoded) != curve25519.ScalarSize {
		return nil, fmt.Errorf("%s invalide: 32 octets encodés en base64 sont requis", label)
	}
	return decoded, nil
}

func keyHex(value, label string) (string, error) {
	decoded, err := decodeKey(value, label)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(decoded), nil
}
