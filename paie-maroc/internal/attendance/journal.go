package attendance

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	DirectionEntry   = "entry"
	DirectionExit    = "exit"
	DirectionUnknown = "unknown"

	VerificationFace        = "face"
	VerificationFingerprint = "fingerprint"
	VerificationCard        = "card"
	VerificationPassword    = "password"
	VerificationManual      = "manual"
	VerificationOther       = "other"
)

// Punch is the corrected journal view. Device payload identity and the audit
// trail remain stored separately so a correction never destroys evidence.
type Punch struct {
	ID           int64  `json:"id"`
	EmployeeID   string `json:"employeeId"`
	EmployeeName string `json:"employeeName"`
	DeviceID     int64  `json:"deviceId"`
	DeviceName   string `json:"deviceName"`
	DeviceUserID string `json:"deviceUserId"`
	OccurredAt   string `json:"occurredAt"`
	Direction    string `json:"direction"`
	Verification string `json:"verification"`
	Source       string `json:"source"`
	Note         string `json:"note"`
	Edited       bool   `json:"edited"`
	UpdatedAt    string `json:"updatedAt"`
	Unlinked     bool   `json:"unlinked"`
}

type PunchInput struct {
	ID           int64  `json:"id"`
	EmployeeID   string `json:"employeeId"`
	DeviceID     int64  `json:"deviceId"`
	OccurredAt   string `json:"occurredAt"`
	Direction    string `json:"direction"`
	Verification string `json:"verification"`
	Note         string `json:"note"`
	Reason       string `json:"reason"`
}

type PunchFilter struct {
	From       string `json:"from"`
	To         string `json:"to"`
	EmployeeID string `json:"employeeId"`
	DeviceID   int64  `json:"deviceId"`
	Query      string `json:"query"`
	Cursor     int64  `json:"cursor"`
	Limit      int    `json:"limit"`
}

type PunchPage struct {
	Items      []Punch `json:"items"`
	NextCursor int64   `json:"nextCursor"`
	Total      int64   `json:"total"`
	Unlinked   int64   `json:"unlinked"`
}

type PunchChange struct {
	ID         int64  `json:"id"`
	PunchID    int64  `json:"punchId"`
	Action     string `json:"action"`
	Reason     string `json:"reason"`
	ChangedBy  string `json:"changedBy"`
	BeforeJSON string `json:"beforeJson"`
	AfterJSON  string `json:"afterJson"`
	ChangedAt  string `json:"changedAt"`
}

type RawPunch struct {
	ExternalID   string
	DeviceUserID string
	OccurredAt   time.Time
	Direction    string
	Verification string
}

type SyncResult struct {
	DeviceID int64  `json:"deviceId"`
	Read     int64  `json:"read"`
	Skipped  int64  `json:"skipped"`
	Imported int64  `json:"imported"`
	Ignored  int64  `json:"ignored"`
	Unlinked int64  `json:"unlinked"`
	Message  string `json:"message"`
}

type Holiday struct {
	Date               string `json:"date"`
	Name               string `json:"name"`
	Kind               string `json:"kind"`
	Paid               bool   `json:"paid"`
	PremiumBasisPoints int    `json:"premiumBasisPoints"`
	Status             string `json:"status"`
	Source             string `json:"source"`
}

type WorkPolicy struct {
	Sector                      string `json:"sector"`
	WeeklyMinutes               int    `json:"weeklyMinutes"`
	DayStart                    string `json:"dayStart"`
	DayEnd                      string `json:"dayEnd"`
	BreakMinutes                int    `json:"breakMinutes"`
	GraceMinutes                int    `json:"graceMinutes"`
	DayPremiumBasisPoints       int    `json:"dayPremiumBasisPoints"`
	NightPremiumBasisPoints     int    `json:"nightPremiumBasisPoints"`
	RestDayPremiumBasisPoints   int    `json:"restDayPremiumBasisPoints"`
	RestNightPremiumBasisPoints int    `json:"restNightPremiumBasisPoints"`
	RulesVersion                string `json:"rulesVersion"`
}

type DailySummary struct {
	EmployeeID      string   `json:"employeeId"`
	EmployeeName    string   `json:"employeeName"`
	Date            string   `json:"date"`
	FirstEntry      string   `json:"firstEntry"`
	LastExit        string   `json:"lastExit"`
	WorkedMinutes   int      `json:"workedMinutes"`
	LateMinutes     int      `json:"lateMinutes"`
	OvertimeMinutes int      `json:"overtimeMinutes"`
	HolidayMinutes  int      `json:"holidayMinutes"`
	Anomalies       []string `json:"anomalies"`
}

func DefaultWorkPolicy() WorkPolicy {
	return WorkPolicy{
		Sector: "non_agricultural", WeeklyMinutes: 44 * 60,
		DayStart: "06:00", DayEnd: "21:00", BreakMinutes: 60, GraceMinutes: 5,
		DayPremiumBasisPoints: 2500, NightPremiumBasisPoints: 5000,
		RestDayPremiumBasisPoints: 5000, RestNightPremiumBasisPoints: 10000,
		RulesVersion: "MA-CODE-TRAVAIL-184-201",
	}
}

func ValidatePunch(input PunchInput) (PunchInput, error) {
	input.EmployeeID = strings.TrimSpace(input.EmployeeID)
	input.Note = strings.TrimSpace(input.Note)
	input.Reason = strings.TrimSpace(input.Reason)
	if input.EmployeeID == "" {
		return PunchInput{}, errors.New("employeeId: choisissez un salarié")
	}
	if _, err := time.Parse(time.RFC3339, input.OccurredAt); err != nil {
		return PunchInput{}, errors.New("occurredAt: date et heure invalides")
	}
	if input.Direction != DirectionEntry && input.Direction != DirectionExit && input.Direction != DirectionUnknown {
		return PunchInput{}, errors.New("direction: sens de pointage invalide")
	}
	validVerification := map[string]bool{
		VerificationFace: true, VerificationFingerprint: true, VerificationCard: true,
		VerificationPassword: true, VerificationManual: true, VerificationOther: true,
	}
	if !validVerification[input.Verification] {
		return PunchInput{}, errors.New("verification: méthode de vérification invalide")
	}
	if input.ID > 0 && len([]rune(input.Reason)) < 5 {
		return PunchInput{}, errors.New("reason: expliquez la correction en au moins 5 caractères")
	}
	if len([]rune(input.Note)) > 500 || len([]rune(input.Reason)) > 500 {
		return PunchInput{}, errors.New("note: 500 caractères maximum")
	}
	return input, nil
}

// SummarizeDay pairs entry/exit events without changing the raw journal. It
// flags incomplete sequences for human review instead of inventing work time.
func SummarizeDay(punches []Punch, policy WorkPolicy, holiday bool) DailySummary {
	result := DailySummary{Anomalies: []string{}}
	if len(punches) == 0 {
		return result
	}
	sort.Slice(punches, func(i, j int) bool { return punches[i].OccurredAt < punches[j].OccurredAt })
	result.EmployeeID, result.EmployeeName = punches[0].EmployeeID, punches[0].EmployeeName
	result.Date = punches[0].OccurredAt[:10]
	var entry time.Time
	for _, punch := range punches {
		at, err := time.Parse(time.RFC3339, punch.OccurredAt)
		if err != nil {
			result.Anomalies = append(result.Anomalies, "Horodatage invalide")
			continue
		}
		direction := punch.Direction
		if direction == DirectionUnknown {
			if entry.IsZero() {
				direction = DirectionEntry
			} else {
				direction = DirectionExit
			}
		}
		if direction == DirectionEntry {
			if !entry.IsZero() {
				result.Anomalies = append(result.Anomalies, "Deux entrées sans sortie")
			}
			entry = at
			if result.FirstEntry == "" {
				result.FirstEntry = punch.OccurredAt
			}
			continue
		}
		if entry.IsZero() {
			result.Anomalies = append(result.Anomalies, "Sortie sans entrée")
			continue
		}
		if !at.After(entry) {
			result.Anomalies = append(result.Anomalies, "Sortie antérieure à l’entrée")
			entry = time.Time{}
			continue
		}
		result.WorkedMinutes += int(at.Sub(entry) / time.Minute)
		result.LastExit = punch.OccurredAt
		entry = time.Time{}
	}
	if !entry.IsZero() {
		result.Anomalies = append(result.Anomalies, "Entrée sans sortie")
	}
	if holiday {
		result.HolidayMinutes = result.WorkedMinutes
	}
	if result.WorkedMinutes > policy.WeeklyMinutes/6 {
		result.OvertimeMinutes = result.WorkedMinutes - policy.WeeklyMinutes/6
	}
	if result.FirstEntry != "" {
		first, _ := time.Parse(time.RFC3339, result.FirstEntry)
		scheduled, err := time.ParseInLocation("2006-01-02 15:04", result.Date+" "+policy.DayStart, first.Location())
		if err == nil && first.After(scheduled.Add(time.Duration(policy.GraceMinutes)*time.Minute)) {
			result.LateMinutes = int(first.Sub(scheduled) / time.Minute)
		}
	}
	return result
}

func ValidateExportColumns(columns []string) ([]string, error) {
	allowed := map[string]bool{
		"employeeId": true, "employeeName": true, "occurredAt": true, "direction": true,
		"deviceName": true, "deviceUserId": true, "verification": true, "source": true,
		"note": true, "edited": true,
	}
	if len(columns) == 0 {
		return nil, errors.New("sélectionnez au moins une colonne")
	}
	seen := make(map[string]bool, len(columns))
	result := make([]string, 0, len(columns))
	for _, column := range columns {
		if !allowed[column] {
			return nil, fmt.Errorf("colonne d’export inconnue: %s", column)
		}
		if !seen[column] {
			seen[column] = true
			result = append(result, column)
		}
	}
	return result, nil
}
