package pointerservice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/attendance"
)

type recordingSession struct {
	punches  []attendance.RawPunch
	complete time.Time
}

func (session *recordingSession) ImportBatch(_ context.Context, batch []attendance.RawPunch) error {
	session.punches = append(session.punches, batch...)
	return nil
}

func (session *recordingSession) Complete(_ context.Context, latest time.Time) error {
	session.complete = latest
	return nil
}

func TestReceiverAcceptsConfiguredADMSJournal(t *testing.T) {
	t.Parallel()
	session := &recordingSession{}
	handler, err := newReceiver([]string{"192.168.10.0/24"}, func(_ context.Context, serial string) (PunchSession, error) {
		if serial != "MB10-TEST-01" {
			t.Fatalf("serial = %q", serial)
		}
		return session, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "http://paie.local:8088/iclock/cdata?SN=MB10-TEST-01&table=ATTLOG",
		strings.NewReader("42\t2026-10-11 08:15:00\t0\t15\t0\n42\t2026-10-11 17:30:00\t1\t1\t0\n"))
	request.RemoteAddr = "192.168.10.42:4370"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "OK: 2\n" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if len(session.punches) != 2 || session.punches[0].Verification != attendance.VerificationFace || session.punches[1].Direction != attendance.DirectionExit {
		t.Fatalf("punches = %+v", session.punches)
	}
	if session.complete.IsZero() || session.complete.Hour() != 17 {
		t.Fatalf("complete = %v", session.complete)
	}
}

func TestReceiverRejectsUntrustedNetworkBeforeImport(t *testing.T) {
	t.Parallel()
	called := false
	handler, err := newReceiver([]string{"192.168.10.0/24"}, func(context.Context, string) (PunchSession, error) {
		called = true
		return &recordingSession{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "http://paie.local:8088/iclock/cdata?SN=MB10-TEST-01&table=ATTLOG",
		strings.NewReader("42\t2026-10-11 08:15:00\t0\t15\n"))
	request.RemoteAddr = "203.0.113.9:4370"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || called {
		t.Fatalf("response=%d called=%v", response.Code, called)
	}
}

func TestADMSParserFlushesBoundedBatches(t *testing.T) {
	t.Parallel()
	var input strings.Builder
	for index := 0; index < admsBatchSize+7; index++ {
		input.WriteString("42\t2026-10-11 08:15:00\t0\t15\n")
	}
	maxBatch, imported := 0, 0
	count, _, err := parseADMSAttendance(context.Background(), "TEST", strings.NewReader(input.String()), func(_ context.Context, batch []attendance.RawPunch) error {
		maxBatch = max(maxBatch, len(batch))
		imported += len(batch)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != admsBatchSize+7 || imported != count || maxBatch != admsBatchSize {
		t.Fatalf("count=%d imported=%d maxBatch=%d", count, imported, maxBatch)
	}
}
