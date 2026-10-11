package pointerservice

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/attendance"
)

const (
	maxADMSBody   = 32 << 20
	admsBatchSize = 1000
)

var serialPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

type PunchSession interface {
	ImportBatch(context.Context, []attendance.RawPunch) error
	Complete(context.Context, time.Time) error
}

type BeginPunchSession func(context.Context, string) (PunchSession, error)

type receiver struct {
	allowed []netip.Prefix
	begin   BeginPunchSession
}

func newReceiver(networks []string, begin BeginPunchSession) (*receiver, error) {
	if begin == nil {
		return nil, errors.New("importateur ADMS absent")
	}
	allowed := make([]netip.Prefix, 0, len(networks))
	for _, value := range networks {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, fmt.Errorf("réseau du collecteur invalide: %w", err)
		}
		allowed = append(allowed, prefix.Masked())
	}
	return &receiver{allowed: allowed, begin: begin}, nil
}

func (service *receiver) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("Cache-Control", "no-store")
	if !service.remoteAllowed(request.RemoteAddr) {
		http.Error(response, "Pointeuse non autorisée", http.StatusForbidden)
		return
	}
	serial := strings.TrimSpace(request.URL.Query().Get("SN"))
	if serial == "" {
		serial = strings.TrimSpace(request.URL.Query().Get("sn"))
	}
	if !serialPattern.MatchString(serial) {
		http.Error(response, "Numéro de série invalide", http.StatusBadRequest)
		return
	}

	switch {
	case request.Method == http.MethodGet && request.URL.Path == "/iclock/cdata":
		fmt.Fprintf(response, "GET OPTION FROM: %s\nATTLOGStamp=0\nOPERLOGStamp=0\nATTPHOTOStamp=0\nErrorDelay=30\nDelay=10\nTransInterval=1\nTransFlag=1111000000\nRealtime=1\nEncrypt=0\n", serial)
	case request.Method == http.MethodPost && request.URL.Path == "/iclock/cdata":
		service.receiveAttendance(response, request, serial)
	case request.Method == http.MethodGet && request.URL.Path == "/iclock/getrequest":
		_, _ = response.Write([]byte("OK\n"))
	case request.Method == http.MethodPost && request.URL.Path == "/iclock/devicecmd":
		_, _ = response.Write([]byte("OK\n"))
	default:
		http.NotFound(response, request)
	}
}

func (service *receiver) receiveAttendance(response http.ResponseWriter, request *http.Request, serial string) {
	table := strings.ToUpper(strings.TrimSpace(request.URL.Query().Get("table")))
	if table != "ATTLOG" {
		http.Error(response, "Type de données non pris en charge", http.StatusBadRequest)
		return
	}
	session, err := service.begin(request.Context(), serial)
	if err != nil {
		http.Error(response, "Pointeuse inconnue ou désactivée", http.StatusForbidden)
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, maxADMSBody)
	count, latest, err := parseADMSAttendance(request.Context(), serial, request.Body, session.ImportBatch)
	if err != nil {
		http.Error(response, "Journal de pointage invalide", http.StatusBadRequest)
		return
	}
	if err := session.Complete(request.Context(), latest); err != nil {
		http.Error(response, "Import du journal impossible", http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(response, "OK: %d\n", count)
}

func (service *receiver) remoteAllowed(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	address, err := netip.ParseAddr(strings.Trim(host, "[]"))
	if err != nil {
		return false
	}
	for _, prefix := range service.allowed {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

type batchImporter func(context.Context, []attendance.RawPunch) error

func parseADMSAttendance(ctx context.Context, serial string, input interface{ Read([]byte) (int, error) }, importBatch batchImporter) (int, time.Time, error) {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64<<10), 256<<10)
	batch := make([]attendance.RawPunch, 0, admsBatchSize)
	count := 0
	var latest time.Time
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := importBatch(ctx, batch); err != nil {
			return err
		}
		clear(batch)
		batch = batch[:0]
		return nil
	}
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return count, latest, err
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		punch, err := parseADMSLine(serial, line)
		if err != nil {
			return count, latest, err
		}
		batch = append(batch, punch)
		count++
		if punch.OccurredAt.After(latest) {
			latest = punch.OccurredAt
		}
		if len(batch) == cap(batch) {
			if err := flush(); err != nil {
				return count, latest, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return count, latest, fmt.Errorf("lecture du journal ADMS: %w", err)
	}
	if err := flush(); err != nil {
		return count, latest, err
	}
	return count, latest, nil
}

func parseADMSLine(serial, line string) (attendance.RawPunch, error) {
	fields := strings.Split(line, "\t")
	if len(fields) < 2 {
		fields = strings.Fields(line)
	}
	if len(fields) < 2 {
		return attendance.RawPunch{}, errors.New("ligne ADMS incomplète")
	}
	userID := strings.TrimSpace(fields[0])
	if matched, _ := regexp.MatchString(`^[0-9]{1,24}$`, userID); !matched {
		return attendance.RawPunch{}, errors.New("identifiant utilisateur ADMS invalide")
	}
	dateValue := strings.TrimSpace(fields[1])
	if len(fields) > 2 && len(dateValue) == 10 && strings.Contains(fields[2], ":") {
		dateValue += " " + strings.TrimSpace(fields[2])
		fields = append(fields[:2], fields[3:]...)
	}
	occurredAt, err := time.ParseInLocation("2006-01-02 15:04:05", dateValue, time.Local)
	if err != nil {
		return attendance.RawPunch{}, errors.New("date de pointage ADMS invalide")
	}
	status := integerField(fields, 2)
	verification := integerField(fields, 3)
	return attendance.RawPunch{
		ExternalID:   fmt.Sprintf("adms|%s|%s|%s|%d|%d", serial, userID, occurredAt.Format(time.RFC3339), status, verification),
		DeviceUserID: userID,
		OccurredAt:   occurredAt,
		Direction:    admsDirection(status),
		Verification: admsVerification(verification),
	}, nil
}

func integerField(fields []string, index int) int {
	if index >= len(fields) {
		return 0
	}
	value, _ := strconv.Atoi(strings.TrimSpace(fields[index]))
	return value
}

func admsDirection(status int) string {
	switch status {
	case 0, 2, 4:
		return attendance.DirectionEntry
	case 1, 3, 5:
		return attendance.DirectionExit
	default:
		return attendance.DirectionUnknown
	}
}

func admsVerification(code int) string {
	switch code {
	case 1:
		return attendance.VerificationFingerprint
	case 3, 4:
		return attendance.VerificationPassword
	case 15:
		return attendance.VerificationFace
	case 2, 5, 6:
		return attendance.VerificationCard
	default:
		return attendance.VerificationOther
	}
}
