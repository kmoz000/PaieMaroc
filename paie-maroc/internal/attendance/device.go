// Package attendance contains safe, vendor-adapter-neutral attendance types.
package attendance

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

const (
	ProtocolStandalone = "standalone"
	ProtocolADMS       = "adms"
	ProtocolBioTime    = "biotime"
)

var hostnamePattern = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9.-]{0,251}[a-zA-Z0-9])?$`)

type Device struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Model         string `json:"model"`
	Protocol      string `json:"protocol"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Enabled       bool   `json:"enabled"`
	LastStatus    string `json:"lastStatus"`
	LastCheckedAt string `json:"lastCheckedAt"`
	LinkedUsers   int64  `json:"linkedUsers"`
}

type Profile struct {
	Model          string   `json:"model"`
	Protocols      []string `json:"protocols"`
	Authentication []string `json:"authentication"`
	FaceCapacity   int      `json:"faceCapacity"`
	FingerCapacity int      `json:"fingerCapacity"`
	MaxEvents      int      `json:"maxEvents"`
	Notes          string   `json:"notes"`
}

type ProbeResult struct {
	Reachable bool   `json:"reachable"`
	LatencyMS int64  `json:"latencyMs"`
	Message   string `json:"message"`
}

func MB10VLProfile() Profile {
	return Profile{
		Model:          "MB10-VL",
		Protocols:      []string{ProtocolADMS, ProtocolStandalone},
		Authentication: []string{"visage", "empreinte", "badge optionnel"},
		FaceCapacity:   100,
		FingerCapacity: 500,
		MaxEvents:      50_000,
		Notes:          "Profil prioritaire PaieMaroc; les gabarits biométriques restent dans la pointeuse.",
	}
}

func ValidateDevice(device Device) (Device, error) {
	device.Name = strings.TrimSpace(device.Name)
	device.Model = strings.TrimSpace(device.Model)
	device.Host = strings.TrimSpace(device.Host)
	if len(device.Name) < 2 || len(device.Name) > 80 {
		return Device{}, errors.New("le nom de la pointeuse doit contenir entre 2 et 80 caractères")
	}
	if device.Model == "" || len(device.Model) > 80 {
		return Device{}, errors.New("le modèle de la pointeuse est obligatoire")
	}
	if device.Protocol != ProtocolStandalone && device.Protocol != ProtocolADMS && device.Protocol != ProtocolBioTime {
		return Device{}, errors.New("protocole ZKTeco non pris en charge")
	}
	if net.ParseIP(device.Host) == nil && !hostnamePattern.MatchString(device.Host) {
		return Device{}, errors.New("adresse IP ou nom d’hôte invalide")
	}
	if device.Port < 1 || device.Port > 65535 {
		return Device{}, errors.New("port réseau invalide")
	}
	return device, nil
}

// Probe verifies network reachability only. Protocol authentication and event
// synchronization are performed by the selected vendor adapter.
func Probe(ctx context.Context, device Device) (ProbeResult, error) {
	validated, err := ValidateDevice(device)
	if err != nil {
		return ProbeResult{}, err
	}
	startedAt := time.Now()
	connection, err := (&net.Dialer{Timeout: 3 * time.Second}).DialContext(
		ctx,
		"tcp",
		net.JoinHostPort(validated.Host, fmt.Sprintf("%d", validated.Port)),
	)
	latency := time.Since(startedAt).Milliseconds()
	if err != nil {
		return ProbeResult{LatencyMS: latency, Message: "Pointeuse injoignable; vérifiez IP, port et pare-feu."}, nil
	}
	_ = connection.Close()
	return ProbeResult{Reachable: true, LatencyMS: latency, Message: "Pointeuse joignable sur le réseau local."}, nil
}
