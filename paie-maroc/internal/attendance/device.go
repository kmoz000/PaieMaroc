// Package attendance contains safe, vendor-adapter-neutral attendance types.
package attendance

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	ProtocolStandalone = "standalone"
	ProtocolADMS       = "adms"
	ProtocolBioTime    = "biotime"
)

var hostnamePattern = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9.-]{0,251}[a-zA-Z0-9])?$`)

const (
	standaloneTimeout   = 8 * time.Second
	standaloneBatchSize = 4096
	zkTCPHeaderSize     = 8
	zkCommandHeaderSize = 8
	zkMaxPacketSize     = 16 << 20
	zkMaxJournalSize    = 512 << 20

	zkTCPMagic1 uint16 = 0x5050
	zkTCPMagic2 uint16 = 0x7d82

	zkCommandConnect         uint16 = 1000
	zkCommandExit            uint16 = 1001
	zkCommandEnable          uint16 = 1002
	zkCommandDisable         uint16 = 1003
	zkCommandAttendance      uint16 = 13
	zkCommandGetFreeSizes    uint16 = 50
	zkCommandPrepareData     uint16 = 1500
	zkCommandData            uint16 = 1501
	zkCommandFreeData        uint16 = 1502
	zkCommandAckOK           uint16 = 2000
	zkCommandAckError        uint16 = 2001
	zkCommandAckData         uint16 = 2002
	zkCommandAckUnauthorized uint16 = 2005
)

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

// StandaloneReadStats reports a bounded standalone-device transfer. Read is
// the number of valid events on or after the requested cursor; Skipped counts
// malformed records rejected before they reach storage.
type StandaloneReadStats struct {
	Read    int64
	Skipped int64
	Latest  time.Time
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

// StreamStandalone reads ZKTeco attendance records directly from a standalone
// TCP device. The callback receives at most 4,096 records and must not retain the
// slice. This keeps live memory bounded independently of the device log size.
//
// The narrow protocol implementation is adapted from the MIT-licensed
// github.com/farizfadian/go-zkteco project. PaieMaroc intentionally includes
// only read-only attendance operations; see THIRD_PARTY_NOTICES.md.
func StreamStandalone(ctx context.Context, device Device, since time.Time, yield func([]RawPunch) error) (StandaloneReadStats, error) {
	if yield == nil {
		return StandaloneReadStats{}, errors.New("traitement des pointages absent")
	}
	validated, err := ValidateDevice(device)
	if err != nil {
		return StandaloneReadStats{}, err
	}
	if validated.Protocol != ProtocolStandalone {
		return StandaloneReadStats{}, errors.New("la lecture directe exige le protocole Standalone TCP/IP")
	}
	client, err := dialZKDevice(ctx, validated)
	if err != nil {
		return StandaloneReadStats{}, fmt.Errorf("connexion ZKTeco: %w", err)
	}
	defer client.close()

	recordCount, err := client.attendanceCount(ctx)
	if err != nil {
		return StandaloneReadStats{}, fmt.Errorf("lecture du compteur ZKTeco: %w", err)
	}
	// Disabling is best-effort and creates a stable snapshot on devices that
	// support it. The deferred enable is attempted even after parse/import errors.
	disabled := client.commandOK(ctx, zkCommandDisable)
	if disabled {
		defer client.commandOK(context.Background(), zkCommandEnable)
	}

	decoder := newZKAttendanceDecoder(ctx, since, standaloneBatchSize, yield)
	if err := client.streamAttendance(ctx, recordCount, decoder); err != nil {
		return decoder.stats, fmt.Errorf("lecture des pointages ZKTeco: %w", err)
	}
	if err := decoder.finish(); err != nil {
		return decoder.stats, err
	}
	return decoder.stats, nil
}

type zkPacketHeader struct {
	command      uint16
	checksum     uint16
	sessionID    uint16
	replyID      uint16
	payloadBytes int
}

type zkDeviceClient struct {
	conn      net.Conn
	timeout   time.Duration
	sessionID uint16
	replyID   uint16
	stopWatch chan struct{}
}

func dialZKDevice(ctx context.Context, device Device) (*zkDeviceClient, error) {
	connection, err := (&net.Dialer{Timeout: standaloneTimeout}).DialContext(
		ctx,
		"tcp",
		net.JoinHostPort(device.Host, strconv.Itoa(device.Port)),
	)
	if err != nil {
		return nil, err
	}
	client := &zkDeviceClient{conn: connection, timeout: standaloneTimeout, replyID: 0xfffe, stopWatch: make(chan struct{})}
	go func() {
		select {
		case <-ctx.Done():
			_ = connection.Close()
		case <-client.stopWatch:
		}
	}()
	if err := client.send(ctx, zkCommandConnect, nil); err != nil {
		client.close()
		return nil, err
	}
	header, payload, err := client.readSmall(ctx, 256)
	if err != nil {
		client.close()
		return nil, err
	}
	_ = payload
	if header.command != zkCommandAckOK && header.command != zkCommandAckData {
		client.close()
		if header.command == zkCommandAckUnauthorized {
			return nil, errors.New("la pointeuse exige une clé de communication non prise en charge")
		}
		return nil, fmt.Errorf("connexion refusée par la pointeuse (réponse %d)", header.command)
	}
	client.sessionID = header.sessionID
	client.replyID = header.replyID
	return client, nil
}

func (client *zkDeviceClient) close() {
	if client.conn == nil {
		return
	}
	_ = client.send(context.Background(), zkCommandExit, nil)
	_ = client.conn.Close()
	client.conn = nil
	close(client.stopWatch)
}

func (client *zkDeviceClient) commandOK(ctx context.Context, command uint16) bool {
	if err := client.send(ctx, command, nil); err != nil {
		return false
	}
	header, _, err := client.readSmall(ctx, 256)
	return err == nil && (header.command == zkCommandAckOK || header.command == zkCommandAckData)
}

func (client *zkDeviceClient) attendanceCount(ctx context.Context) (int, error) {
	if err := client.send(ctx, zkCommandGetFreeSizes, nil); err != nil {
		return 0, err
	}
	header, payload, err := client.readSmall(ctx, 1024)
	if err != nil {
		return 0, err
	}
	if header.command != zkCommandAckOK && header.command != zkCommandAckData {
		return 0, nil
	}
	if len(payload) < 20 {
		return 0, nil
	}
	return int(binary.LittleEndian.Uint32(payload[16:20])), nil
}

func (client *zkDeviceClient) streamAttendance(ctx context.Context, recordCount int, decoder *zkAttendanceDecoder) error {
	if err := client.send(ctx, zkCommandAttendance, nil); err != nil {
		return err
	}
	header, err := client.readHeader(ctx)
	if err != nil {
		return err
	}
	switch header.command {
	case zkCommandAckError, zkCommandAckUnauthorized:
		if err := client.discardPayload(ctx, header.payloadBytes); err != nil {
			return err
		}
		return fmt.Errorf("commande de pointage refusée (réponse %d)", header.command)
	case zkCommandData:
		decoder.setFormat(header.payloadBytes, recordCount)
		return client.consumePayload(ctx, header.payloadBytes, decoder.write)
	case zkCommandAckOK, zkCommandAckData, zkCommandPrepareData:
		if header.payloadBytes < 4 {
			return errors.New("taille du journal absente")
		}
		var sizeBytes [4]byte
		if err := client.readFull(ctx, sizeBytes[:]); err != nil {
			return err
		}
		declared := int(binary.LittleEndian.Uint32(sizeBytes[:]))
		if declared < 0 || declared > zkMaxJournalSize {
			return fmt.Errorf("journal trop volumineux annoncé par la pointeuse: %d octets", declared)
		}
		available := header.payloadBytes - 4
		decoder.setFormat(declared, recordCount)
		if available > 0 {
			consume := min(available, declared)
			if err := client.consumePayload(ctx, consume, decoder.write); err != nil {
				return err
			}
			if available > consume {
				if err := client.discardPayload(ctx, available-consume); err != nil {
					return err
				}
			}
			declared -= consume
		}
		return client.streamPreparedData(ctx, declared, decoder)
	default:
		if err := client.discardPayload(ctx, header.payloadBytes); err != nil {
			return err
		}
		return fmt.Errorf("réponse de journal inattendue: %d", header.command)
	}
}

func (client *zkDeviceClient) streamPreparedData(ctx context.Context, remaining int, decoder *zkAttendanceDecoder) error {
	defer client.commandOK(context.Background(), zkCommandFreeData)
	for remaining > 0 {
		if err := client.send(ctx, zkCommandData, nil); err != nil {
			return err
		}
		header, err := client.readHeader(ctx)
		if err != nil {
			return err
		}
		if header.command != zkCommandData && header.command != zkCommandAckData {
			_ = client.discardPayload(ctx, header.payloadBytes)
			return fmt.Errorf("bloc de journal inattendu: %d", header.command)
		}
		if header.payloadBytes == 0 {
			return errors.New("bloc de journal vide avant la fin annoncée")
		}
		consume := min(header.payloadBytes, remaining)
		if err := client.consumePayload(ctx, consume, decoder.write); err != nil {
			return err
		}
		if header.payloadBytes > consume {
			if err := client.discardPayload(ctx, header.payloadBytes-consume); err != nil {
				return err
			}
		}
		remaining -= consume
	}
	return nil
}

func (client *zkDeviceClient) send(ctx context.Context, command uint16, payload []byte) error {
	if client.conn == nil {
		return errors.New("connexion ZKTeco fermée")
	}
	if err := client.setDeadline(ctx, true); err != nil {
		return err
	}
	client.replyID++
	commandBytes := make([]byte, zkCommandHeaderSize+len(payload))
	binary.LittleEndian.PutUint16(commandBytes[0:2], command)
	binary.LittleEndian.PutUint16(commandBytes[4:6], client.sessionID)
	binary.LittleEndian.PutUint16(commandBytes[6:8], client.replyID)
	copy(commandBytes[zkCommandHeaderSize:], payload)
	binary.LittleEndian.PutUint16(commandBytes[2:4], zkChecksum(commandBytes))

	var wrapper [zkTCPHeaderSize]byte
	binary.LittleEndian.PutUint16(wrapper[0:2], zkTCPMagic1)
	binary.LittleEndian.PutUint16(wrapper[2:4], zkTCPMagic2)
	binary.LittleEndian.PutUint32(wrapper[4:8], uint32(len(commandBytes)))
	if err := writeFull(client.conn, wrapper[:]); err != nil {
		return err
	}
	return writeFull(client.conn, commandBytes)
}

func (client *zkDeviceClient) readSmall(ctx context.Context, limit int) (zkPacketHeader, []byte, error) {
	header, err := client.readHeader(ctx)
	if err != nil {
		return zkPacketHeader{}, nil, err
	}
	if header.payloadBytes > limit {
		if discardErr := client.discardPayload(ctx, header.payloadBytes); discardErr != nil {
			return zkPacketHeader{}, nil, discardErr
		}
		return zkPacketHeader{}, nil, fmt.Errorf("réponse ZKTeco trop grande: %d octets", header.payloadBytes)
	}
	payload := make([]byte, header.payloadBytes)
	if err := client.readFull(ctx, payload); err != nil {
		return zkPacketHeader{}, nil, err
	}
	return header, payload, nil
}

func (client *zkDeviceClient) readHeader(ctx context.Context) (zkPacketHeader, error) {
	if err := client.setDeadline(ctx, false); err != nil {
		return zkPacketHeader{}, err
	}
	var raw [zkTCPHeaderSize + zkCommandHeaderSize]byte
	if _, err := io.ReadFull(client.conn, raw[:]); err != nil {
		return zkPacketHeader{}, err
	}
	if binary.LittleEndian.Uint16(raw[0:2]) != zkTCPMagic1 || binary.LittleEndian.Uint16(raw[2:4]) != zkTCPMagic2 {
		return zkPacketHeader{}, errors.New("en-tête TCP ZKTeco invalide")
	}
	frameBytes := int(binary.LittleEndian.Uint32(raw[4:8]))
	if frameBytes < zkCommandHeaderSize || frameBytes > zkMaxPacketSize {
		return zkPacketHeader{}, fmt.Errorf("taille de trame ZKTeco invalide: %d", frameBytes)
	}
	header := zkPacketHeader{
		command:      binary.LittleEndian.Uint16(raw[8:10]),
		checksum:     binary.LittleEndian.Uint16(raw[10:12]),
		sessionID:    binary.LittleEndian.Uint16(raw[12:14]),
		replyID:      binary.LittleEndian.Uint16(raw[14:16]),
		payloadBytes: frameBytes - zkCommandHeaderSize,
	}
	client.replyID = header.replyID
	return header, nil
}

func (client *zkDeviceClient) consumePayload(ctx context.Context, size int, consume func([]byte) error) error {
	var buffer [32 << 10]byte
	for size > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		chunkSize := min(size, len(buffer))
		if err := client.readFull(ctx, buffer[:chunkSize]); err != nil {
			return err
		}
		if err := consume(buffer[:chunkSize]); err != nil {
			return err
		}
		size -= chunkSize
	}
	return nil
}

func (client *zkDeviceClient) discardPayload(ctx context.Context, size int) error {
	return client.consumePayload(ctx, size, func([]byte) error { return nil })
}

func (client *zkDeviceClient) readFull(ctx context.Context, target []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := client.setDeadline(ctx, false); err != nil {
		return err
	}
	_, err := io.ReadFull(client.conn, target)
	return err
}

func (client *zkDeviceClient) setDeadline(ctx context.Context, write bool) error {
	deadline := time.Now().Add(client.timeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if write {
		return client.conn.SetWriteDeadline(deadline)
	}
	return client.conn.SetReadDeadline(deadline)
}

func writeFull(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}

func zkChecksum(data []byte) uint16 {
	var sum uint32
	for len(data) > 1 {
		sum += uint32(binary.LittleEndian.Uint16(data[:2]))
		data = data[2:]
	}
	if len(data) == 1 {
		sum += uint32(data[0])
	}
	for sum > 0xffff {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

type zkAttendanceDecoder struct {
	ctx        context.Context
	since      time.Time
	yield      func([]RawPunch) error
	batch      []RawPunch
	remainder  []byte
	recordSize int
	stats      StandaloneReadStats
}

func newZKAttendanceDecoder(ctx context.Context, since time.Time, batchSize int, yield func([]RawPunch) error) *zkAttendanceDecoder {
	if batchSize <= 0 || batchSize > 4096 {
		batchSize = standaloneBatchSize
	}
	return &zkAttendanceDecoder{
		ctx: ctx, since: since, yield: yield,
		batch: make([]RawPunch, 0, batchSize), remainder: make([]byte, 0, 40),
	}
}

func (decoder *zkAttendanceDecoder) setFormat(dataBytes, recordCount int) {
	if decoder.recordSize != 0 {
		return
	}
	if recordCount > 0 && dataBytes%recordCount == 0 {
		candidate := dataBytes / recordCount
		if candidate == 8 || candidate == 16 || candidate == 40 {
			decoder.recordSize = candidate
			return
		}
	}
	for _, candidate := range []int{40, 16, 8} {
		if dataBytes >= candidate && dataBytes%candidate == 0 {
			decoder.recordSize = candidate
			return
		}
	}
}

func (decoder *zkAttendanceDecoder) write(data []byte) error {
	if decoder.recordSize == 0 {
		return errors.New("format des pointages ZKTeco non reconnu")
	}
	if len(decoder.remainder) > 0 {
		missing := decoder.recordSize - len(decoder.remainder)
		if len(data) < missing {
			decoder.remainder = append(decoder.remainder, data...)
			return nil
		}
		decoder.remainder = append(decoder.remainder, data[:missing]...)
		if err := decoder.decodeRecord(decoder.remainder); err != nil {
			return err
		}
		decoder.remainder = decoder.remainder[:0]
		data = data[missing:]
	}
	for len(data) >= decoder.recordSize {
		if err := decoder.decodeRecord(data[:decoder.recordSize]); err != nil {
			return err
		}
		data = data[decoder.recordSize:]
	}
	decoder.remainder = append(decoder.remainder, data...)
	return nil
}

func (decoder *zkAttendanceDecoder) decodeRecord(record []byte) error {
	if err := decoder.ctx.Err(); err != nil {
		return err
	}
	userID, occurredAt, state, verification, workCode, ok := decodeZKAttendanceRecord(record)
	if !ok {
		decoder.stats.Skipped++
		return nil
	}
	if occurredAt.Before(decoder.since) {
		return nil
	}
	decoder.stats.Read++
	if occurredAt.After(decoder.stats.Latest) {
		decoder.stats.Latest = occurredAt
	}
	decoder.batch = append(decoder.batch, RawPunch{
		ExternalID:   attendanceExternalID(userID, occurredAt, state, verification, workCode),
		DeviceUserID: userID,
		OccurredAt:   occurredAt,
		Direction:    directionFromState(state),
		Verification: verificationFromCode(verification),
	})
	if len(decoder.batch) == cap(decoder.batch) {
		if err := decoder.yield(decoder.batch); err != nil {
			return err
		}
		clear(decoder.batch)
		decoder.batch = decoder.batch[:0]
	}
	return nil
}

func (decoder *zkAttendanceDecoder) finish() error {
	if len(decoder.remainder) != 0 {
		return fmt.Errorf("journal ZKTeco tronqué: %d octet(s) restant(s)", len(decoder.remainder))
	}
	if len(decoder.batch) == 0 {
		return nil
	}
	err := decoder.yield(decoder.batch)
	clear(decoder.batch)
	decoder.batch = decoder.batch[:0]
	return err
}

func decodeZKAttendanceRecord(record []byte) (userID string, occurredAt time.Time, state, verification, workCode int, ok bool) {
	switch len(record) {
	case 8:
		userID = strconv.Itoa(int(binary.LittleEndian.Uint16(record[0:2])))
		state = int(record[2])
		occurredAt = decodeZKTime(binary.LittleEndian.Uint32(record[3:7]))
		verification = int(record[7])
	case 16:
		userID = strconv.FormatUint(uint64(binary.LittleEndian.Uint32(record[0:4])), 10)
		occurredAt = decodeZKTime(binary.LittleEndian.Uint32(record[4:8]))
		state = int(record[8])
		verification = int(record[9])
		workCode = int(binary.LittleEndian.Uint32(record[12:16]))
	case 40:
		userID = string(bytes.TrimRight(record[2:26], "\x00 "))
		if userID == "" {
			userID = strconv.Itoa(int(binary.LittleEndian.Uint16(record[0:2])))
		}
		state = int(record[26])
		occurredAt = decodeZKTime(binary.LittleEndian.Uint32(record[27:31]))
		verification = int(record[31])
	default:
		return "", time.Time{}, 0, 0, 0, false
	}
	if userID == "" || occurredAt.Year() < 2000 || occurredAt.Year() > 2100 {
		return "", time.Time{}, 0, 0, 0, false
	}
	return userID, occurredAt, state, verification, workCode, true
}

func decodeZKTime(value uint32) time.Time {
	second := int(value % 60)
	value /= 60
	minute := int(value % 60)
	value /= 60
	hour := int(value % 24)
	value /= 24
	day := int(value%31) + 1
	value /= 31
	month := time.Month(value%12) + 1
	year := int(value/12) + 2000
	decoded := time.Date(year, month, day, hour, minute, second, 0, time.Local)
	if decoded.Year() != year || decoded.Month() != month || decoded.Day() != day {
		return time.Time{}
	}
	return decoded
}

func attendanceExternalID(userID string, occurredAt time.Time, state, verification, workCode int) string {
	buffer := make([]byte, 0, len(userID)+48)
	buffer = append(buffer, userID...)
	buffer = append(buffer, '|')
	buffer = occurredAt.AppendFormat(buffer, time.RFC3339)
	buffer = append(buffer, '|')
	buffer = strconv.AppendInt(buffer, int64(state), 10)
	buffer = append(buffer, '|')
	buffer = strconv.AppendInt(buffer, int64(verification), 10)
	buffer = append(buffer, '|')
	buffer = strconv.AppendInt(buffer, int64(workCode), 10)
	return string(buffer)
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
	case 1, 3, 4, 6:
		return VerificationFingerprint
	case 2, 5:
		return VerificationCard
	case 8, 9, 10, 12, 13, 15:
		return VerificationFace
	default:
		return VerificationOther
	}
}
