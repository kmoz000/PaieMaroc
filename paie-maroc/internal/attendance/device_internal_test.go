package attendance

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

func TestDecodeZKAttendanceRecordLayouts(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.October, 10, 8, 31, 42, 0, time.Local)
	tests := []struct {
		name         string
		record       []byte
		wantUser     string
		wantState    int
		wantVerify   int
		wantWorkCode int
	}{
		{name: "eight bytes", record: attendanceRecord8(42, at, 0, 1), wantUser: "42", wantVerify: 1},
		{name: "sixteen bytes", record: attendanceRecord16(123456, at, 1, 2, 77), wantUser: "123456", wantState: 1, wantVerify: 2, wantWorkCode: 77},
		{name: "forty bytes", record: attendanceRecord40(42, "000000042", at, 4, 15), wantUser: "000000042", wantState: 4, wantVerify: 15},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			user, occurredAt, state, verification, workCode, ok := decodeZKAttendanceRecord(test.record)
			if !ok {
				t.Fatal("record was rejected")
			}
			if user != test.wantUser || !occurredAt.Equal(at) || state != test.wantState || verification != test.wantVerify || workCode != test.wantWorkCode {
				t.Fatalf("decoded = user %q, time %v, state %d, verification %d, workCode %d", user, occurredAt, state, verification, workCode)
			}
		})
	}
}

func TestZKAttendanceDecoderStreamsBoundedBatches(t *testing.T) {
	t.Parallel()
	const records = 5001
	start := time.Date(2026, time.October, 10, 8, 0, 0, 0, time.Local)
	data := make([]byte, 0, records*40)
	for index := range records {
		data = append(data, attendanceRecord40(uint16(index+1), fmt.Sprintf("%09d", index+1), start.Add(time.Duration(index)*time.Second), 0, 1)...)
	}

	var received, maxBatch int
	decoder := newZKAttendanceDecoder(context.Background(), start.Add(100*time.Second), standaloneBatchSize, func(batch []RawPunch) error {
		received += len(batch)
		maxBatch = max(maxBatch, len(batch))
		return nil
	})
	decoder.setFormat(len(data), records)
	for offset := 0; offset < len(data); {
		end := min(offset+137, len(data))
		if err := decoder.write(data[offset:end]); err != nil {
			t.Fatal(err)
		}
		offset = end
	}
	if err := decoder.finish(); err != nil {
		t.Fatal(err)
	}
	if received != records-100 || decoder.stats.Read != records-100 {
		t.Fatalf("received=%d stats=%+v", received, decoder.stats)
	}
	if maxBatch != standaloneBatchSize {
		t.Fatalf("maximum batch = %d, want %d", maxBatch, standaloneBatchSize)
	}
	if !decoder.stats.Latest.Equal(start.Add((records - 1) * time.Second)) {
		t.Fatalf("latest = %v", decoder.stats.Latest)
	}
}

func TestZKAttendanceDecoderRejectsTruncatedJournal(t *testing.T) {
	t.Parallel()
	decoder := newZKAttendanceDecoder(context.Background(), time.Time{}, 4, func([]RawPunch) error { return nil })
	decoder.setFormat(80, 2)
	if err := decoder.write(make([]byte, 39)); err != nil {
		t.Fatal(err)
	}
	if err := decoder.finish(); err == nil {
		t.Fatal("truncated journal was accepted")
	}
}

func TestStreamStandaloneAgainstTCPDevice(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	at := time.Date(2026, time.October, 10, 8, 30, 0, 0, time.Local)
	journal := append(attendanceRecord40(1, "000000001", at, 0, 15), attendanceRecord40(2, "000000002", at.Add(time.Minute), 1, 1)...)
	serverErr := make(chan error, 1)
	go serveTestZKDevice(listener, journal, serverErr)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var punches []RawPunch
	port := listener.Addr().(*net.TCPAddr).Port
	stats, err := StreamStandalone(ctx, Device{
		Name: "Test", Model: "MB10-VL", Protocol: ProtocolStandalone,
		Host: "127.0.0.1", Port: port, Enabled: true,
	}, time.Time{}, func(batch []RawPunch) error {
		punches = append(punches, batch...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
	if stats.Read != 2 || stats.Skipped != 0 || len(punches) != 2 {
		t.Fatalf("stats=%+v punches=%+v", stats, punches)
	}
	if punches[0].DeviceUserID != "000000001" || punches[1].Direction != DirectionExit {
		t.Fatalf("punches=%+v", punches)
	}
}

func TestStreamStandalonePreparedTransfer(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	at := time.Date(2026, time.October, 10, 9, 0, 0, 0, time.Local)
	journal := attendanceRecord40(7, "000000007", at, 0, 2)
	serverErr := make(chan error, 1)
	go serveTestZKPreparedDevice(listener, journal, serverErr)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	port := listener.Addr().(*net.TCPAddr).Port
	stats, err := StreamStandalone(ctx, Device{
		Name: "Test", Model: "MB10-VL", Protocol: ProtocolStandalone,
		Host: "127.0.0.1", Port: port, Enabled: true,
	}, time.Time{}, func(batch []RawPunch) error {
		if len(batch) != 1 || batch[0].DeviceUserID != "000000007" {
			return fmt.Errorf("lot inattendu: %+v", batch)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
	if stats.Read != 1 || !stats.Latest.Equal(at) {
		t.Fatalf("stats=%+v", stats)
	}
}

func BenchmarkZKAttendanceDecoder100K(b *testing.B) {
	const records = 100_000
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.Local)
	data := make([]byte, 0, records*40)
	for index := range records {
		data = append(data, attendanceRecord40(uint16(index), fmt.Sprintf("%09d", index), start.Add(time.Duration(index)*time.Second), 0, 1)...)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		decoder := newZKAttendanceDecoder(context.Background(), time.Time{}, standaloneBatchSize, func([]RawPunch) error { return nil })
		decoder.setFormat(len(data), records)
		if err := decoder.write(data); err != nil {
			b.Fatal(err)
		}
		if err := decoder.finish(); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(standaloneBatchSize, "max_batch_records")
}

func serveTestZKDevice(listener net.Listener, journal []byte, result chan<- error) {
	connection, err := listener.Accept()
	if err != nil {
		result <- err
		return
	}
	defer connection.Close()
	commands := []uint16{zkCommandConnect, zkCommandGetFreeSizes, zkCommandDisable, zkCommandAttendance, zkCommandEnable, zkCommandExit}
	for _, want := range commands {
		command, replyID, err := readTestZKPacket(connection)
		if err != nil {
			result <- err
			return
		}
		if command != want {
			result <- fmt.Errorf("commande reçue %d, attendue %d", command, want)
			return
		}
		if command == zkCommandExit {
			result <- nil
			return
		}
		var payload []byte
		responseCommand := zkCommandAckOK
		switch command {
		case zkCommandGetFreeSizes:
			payload = make([]byte, 20)
			binary.LittleEndian.PutUint32(payload[16:20], uint32(len(journal)/40))
		case zkCommandAttendance:
			responseCommand = zkCommandData
			payload = journal
		}
		if err := writeTestZKPacket(connection, responseCommand, 7, replyID, payload); err != nil {
			result <- err
			return
		}
	}
	result <- errorsForImpossibleTestPath()
}

func serveTestZKPreparedDevice(listener net.Listener, journal []byte, result chan<- error) {
	connection, err := listener.Accept()
	if err != nil {
		result <- err
		return
	}
	defer connection.Close()
	commands := []uint16{zkCommandConnect, zkCommandGetFreeSizes, zkCommandDisable, zkCommandAttendance, zkCommandData, zkCommandFreeData, zkCommandEnable, zkCommandExit}
	for _, want := range commands {
		command, replyID, err := readTestZKPacket(connection)
		if err != nil {
			result <- err
			return
		}
		if command != want {
			result <- fmt.Errorf("commande reçue %d, attendue %d", command, want)
			return
		}
		if command == zkCommandExit {
			result <- nil
			return
		}
		responseCommand := zkCommandAckOK
		var payload []byte
		switch command {
		case zkCommandGetFreeSizes:
			payload = make([]byte, 20)
			binary.LittleEndian.PutUint32(payload[16:20], 1)
		case zkCommandAttendance:
			responseCommand = zkCommandPrepareData
			payload = make([]byte, 4)
			binary.LittleEndian.PutUint32(payload, uint32(len(journal)))
		case zkCommandData:
			responseCommand = zkCommandData
			payload = journal
		}
		if err := writeTestZKPacket(connection, responseCommand, 7, replyID, payload); err != nil {
			result <- err
			return
		}
	}
	result <- errorsForImpossibleTestPath()
}

func readTestZKPacket(reader io.Reader) (command, replyID uint16, err error) {
	var wrapper [zkTCPHeaderSize]byte
	if _, err = io.ReadFull(reader, wrapper[:]); err != nil {
		return 0, 0, err
	}
	size := int(binary.LittleEndian.Uint32(wrapper[4:8]))
	frame := make([]byte, size)
	if _, err = io.ReadFull(reader, frame); err != nil {
		return 0, 0, err
	}
	return binary.LittleEndian.Uint16(frame[0:2]), binary.LittleEndian.Uint16(frame[6:8]), nil
}

func writeTestZKPacket(writer io.Writer, command, sessionID, replyID uint16, payload []byte) error {
	frame := make([]byte, zkCommandHeaderSize+len(payload))
	binary.LittleEndian.PutUint16(frame[0:2], command)
	binary.LittleEndian.PutUint16(frame[4:6], sessionID)
	binary.LittleEndian.PutUint16(frame[6:8], replyID)
	copy(frame[8:], payload)
	binary.LittleEndian.PutUint16(frame[2:4], zkChecksum(frame))
	var wrapper [zkTCPHeaderSize]byte
	binary.LittleEndian.PutUint16(wrapper[0:2], zkTCPMagic1)
	binary.LittleEndian.PutUint16(wrapper[2:4], zkTCPMagic2)
	binary.LittleEndian.PutUint32(wrapper[4:8], uint32(len(frame)))
	if err := writeFull(writer, wrapper[:]); err != nil {
		return err
	}
	return writeFull(writer, frame)
}

func attendanceRecord8(userID uint16, at time.Time, state, verification byte) []byte {
	record := make([]byte, 8)
	binary.LittleEndian.PutUint16(record[0:2], userID)
	record[2] = state
	binary.LittleEndian.PutUint32(record[3:7], encodeZKTime(at))
	record[7] = verification
	return record
}

func attendanceRecord16(userID uint32, at time.Time, state, verification byte, workCode uint32) []byte {
	record := make([]byte, 16)
	binary.LittleEndian.PutUint32(record[0:4], userID)
	binary.LittleEndian.PutUint32(record[4:8], encodeZKTime(at))
	record[8], record[9] = state, verification
	binary.LittleEndian.PutUint32(record[12:16], workCode)
	return record
}

func attendanceRecord40(uid uint16, userID string, at time.Time, state, verification byte) []byte {
	record := make([]byte, 40)
	binary.LittleEndian.PutUint16(record[0:2], uid)
	copy(record[2:26], userID)
	record[26] = state
	binary.LittleEndian.PutUint32(record[27:31], encodeZKTime(at))
	record[31] = verification
	return record
}

func encodeZKTime(at time.Time) uint32 {
	value := uint32(at.Year() - 2000)
	value = value*12 + uint32(at.Month()-1)
	value = value*31 + uint32(at.Day()-1)
	value = value*24 + uint32(at.Hour())
	value = value*60 + uint32(at.Minute())
	return value*60 + uint32(at.Second())
}

func errorsForImpossibleTestPath() error {
	return fmt.Errorf("serveur de test terminé sans commande de sortie")
}
