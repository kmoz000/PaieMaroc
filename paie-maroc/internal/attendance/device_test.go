package attendance_test

import (
	"context"
	"net"
	"testing"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/attendance"
)

func TestValidateDevice(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		device  attendance.Device
		wantErr bool
	}{
		{name: "mb10 vl", device: attendance.Device{Name: "Entrée", Model: "MB10-VL", Protocol: attendance.ProtocolStandalone, Host: "192.168.1.201", Port: 4370, Enabled: true}},
		{name: "invalid host", device: attendance.Device{Name: "Entrée", Model: "MB10-VL", Protocol: attendance.ProtocolStandalone, Host: "bad host", Port: 4370}, wantErr: true},
		{name: "invalid protocol", device: attendance.Device{Name: "Entrée", Model: "MB10-VL", Protocol: "magic", Host: "192.168.1.201", Port: 4370}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := attendance.ValidateDevice(tt.device)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateDevice() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMB10VLProfile(t *testing.T) {
	t.Parallel()

	profile := attendance.MB10VLProfile()
	if profile.FaceCapacity != 100 || profile.FingerCapacity != 500 || profile.MaxEvents != 50_000 {
		t.Fatalf("MB10VLProfile() = %+v", profile)
	}
	if len(profile.Protocols) != 2 {
		t.Fatalf("Protocols = %v", profile.Protocols)
	}
}

func TestProbeReachableEndpoint(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			_ = connection.Close()
		}
	}()
	port := listener.Addr().(*net.TCPAddr).Port
	result, err := attendance.Probe(context.Background(), attendance.Device{
		Name: "Test", Model: "MB10-VL", Protocol: attendance.ProtocolStandalone,
		Host: "127.0.0.1", Port: port, Enabled: true,
	})
	if err != nil || !result.Reachable {
		t.Fatalf("Probe() = %+v, %v", result, err)
	}
}
