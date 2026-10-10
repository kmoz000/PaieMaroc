package attendance

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	zkteco "github.com/farizfadian/go-zkteco"
)

// PullStandalone reads attendance transactions over the ZKTeco standalone
// protocol. It never writes users, templates, settings, or logs to the device.
func PullStandalone(ctx context.Context, device Device, since time.Time) ([]RawPunch, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	validated, err := ValidateDevice(device)
	if err != nil {
		return nil, err
	}
	client, err := zkteco.Connect(
		net.JoinHostPort(validated.Host, strconv.Itoa(validated.Port)),
		zkteco.WithTimeout(8*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("connexion ZKTeco: %w", err)
	}
	defer client.Disconnect()

	logs, err := client.GetAttendanceSince(since)
	if err != nil {
		return nil, fmt.Errorf("lecture des pointages ZKTeco: %w", err)
	}
	result := make([]RawPunch, 0, len(logs))
	for _, log := range logs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		userID := strconv.Itoa(log.UserID)
		result = append(result, RawPunch{
			ExternalID:   fmt.Sprintf("%s|%s|%d|%d|%d", userID, log.Time.Format(time.RFC3339), log.State, log.VerifyType, log.WorkCode),
			DeviceUserID: userID,
			OccurredAt:   log.Time,
			Direction:    directionFromState(log.State),
			Verification: verificationFromCode(log.VerifyType),
		})
	}
	return result, nil
}

func directionFromState(state int) string {
	switch state {
	case 0, 3, 4:
		return DirectionEntry
	case 1, 2, 5:
		return DirectionExit
	default:
		return DirectionUnknown
	}
}

func verificationFromCode(code int) string {
	switch code {
	case 0:
		return VerificationPassword
	case 1:
		return VerificationFingerprint
	case 2:
		return VerificationCard
	case 15:
		return VerificationFace
	default:
		return VerificationOther
	}
}
