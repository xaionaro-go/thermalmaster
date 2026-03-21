package thermalmaster

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/facebookincubator/go-belt/tool/logger"
)

const readChunkSize = 16384

// ParseMarker parses a 12-byte frame marker from raw bytes.
func ParseMarker(data []byte) Marker {
	return Marker{
		Length: data[0],
		Sync:   data[1],
		Cnt1:   binary.LittleEndian.Uint32(data[2:6]),
		Cnt2:   binary.LittleEndian.Uint32(data[6:10]),
		Cnt3:   binary.LittleEndian.Uint16(data[10:12]),
	}
}

func isStartSync(b byte) bool {
	return b == SyncStartEven || b == SyncStartOdd
}

// findStartMarker scans buf[0:n] for a start marker pattern (0x0C followed
// by 0x8C or 0x8D). Returns the offset of the marker, or -1 if not found.
func findStartMarker(buf []byte, n int) int {
	// Need at least 2 bytes to identify the marker header.
	for i := 0; i+1 < n; i++ {
		if buf[i] == MarkerSize && isStartSync(buf[i+1]) {
			return i
		}
	}
	return -1
}

// ReadFrame reads a complete frame from the camera.
// Returns start_marker + pixel_data (without end marker).
//
// The USB device sends frames as continuous bulk data. Each frame consists of
// a 12-byte start marker, pixel data, and a 12-byte end marker. The markers
// are embedded within larger USB reads (typically 16384 bytes), so
// synchronization scans for the 2-byte start marker pattern within each read.
func (d *Device) ReadFrame(ctx context.Context) ([]byte, error) {
	d.mu.Lock()
	streaming := d.streaming
	d.mu.Unlock()

	if !streaming {
		return nil, fmt.Errorf("not streaming")
	}

	frameReadSize := d.config.FrameReadSize()
	buf := make([]byte, frameReadSize)
	chunk := make([]byte, readChunkSize)

	// Phase 1: find a start marker in the USB stream.
	// Scan each bulk read for the 2-byte pattern [0x0C, 0x8C/0x8D].
	// Once found, copy the marker and any trailing data into buf.
	pos := 0
	for pos == 0 {
		n, err := d.transport.BulkRead(bulkEndpointAddr, chunk)
		if err != nil {
			return nil, fmt.Errorf("reading frame data: %w", err)
		}

		off := findStartMarker(chunk, n)
		if off < 0 {
			logger.Tracef(ctx, "sync: no start marker in %d bytes", n)
			continue
		}

		// Copy from the marker to end of chunk into buf.
		toCopy := n - off
		if toCopy > frameReadSize {
			toCopy = frameReadSize
		}
		copy(buf[:toCopy], chunk[off:off+toCopy])
		pos = toCopy
	}

	// Phase 2: read remaining frame data (pixel data + end marker).
	for pos < frameReadSize {
		n, err := d.transport.BulkRead(bulkEndpointAddr, chunk)
		if err != nil {
			return nil, fmt.Errorf("reading frame data: %w", err)
		}

		toCopy := n
		if pos+toCopy > frameReadSize {
			toCopy = frameReadSize - pos
		}
		copy(buf[pos:pos+toCopy], chunk[:toCopy])
		pos += toCopy
	}

	startMarker := ParseMarker(buf[:MarkerSize])
	endMarker := ParseMarker(buf[frameReadSize-MarkerSize : frameReadSize])

	d.mu.Lock()
	defer d.mu.Unlock()

	if startMarker.Cnt1 != endMarker.Cnt1 {
		d.stats.MarkerMismatches++
	}

	if d.stats.FramesRead > 0 {
		expectedCnt3 := (d.stats.LastCnt3 + Cnt3Increment) % Cnt3Wrap
		cnt3Diff := (endMarker.Cnt3 - expectedCnt3) % Cnt3Wrap
		if cnt3Diff > Cnt3Increment/2 && cnt3Diff < Cnt3Wrap-Cnt3Increment {
			dropped := cnt3Diff / Cnt3Increment
			d.stats.FramesDropped += uint64(dropped)
		}
	}

	d.stats.FramesRead++
	d.stats.LastCnt1 = startMarker.Cnt1
	d.stats.LastCnt3 = endMarker.Cnt3

	// Return start_marker + pixel_data (without end marker).
	return buf[:frameReadSize-MarkerSize], nil
}

// ExtractThermalData extracts the temperature data (rows sensor_h+2 to
// 2*sensor_h+1) and returns a slice of raw thermal values in 1/64 Kelvin units.
func ExtractThermalData(frameData []byte, cfg ModelConfig) []RawThermalValue {
	if len(frameData) < MarkerSize+cfg.FrameSize() {
		return nil
	}

	pixelData := frameData[MarkerSize:]
	startByte := cfg.ThermalRowStart() * cfg.SensorW * 2
	endByte := cfg.ThermalRowEnd() * cfg.SensorW * 2

	if endByte > len(pixelData) {
		return nil
	}

	thermalBytes := pixelData[startByte:endByte]
	result := make([]RawThermalValue, len(thermalBytes)/2)
	for i := range result {
		result[i] = RawThermalValue(binary.LittleEndian.Uint16(thermalBytes[i*2 : i*2+2]))
	}
	return result
}

// ExtractIRBrightness extracts the IR brightness image (rows 0 to sensor_h-1)
// and returns the low byte of each 16-bit value (8-bit hardware AGC'd
// brightness).
func ExtractIRBrightness(frameData []byte, cfg ModelConfig) []uint8 {
	if len(frameData) < MarkerSize+cfg.FrameSize() {
		return nil
	}

	pixelData := frameData[MarkerSize:]
	endByte := cfg.IRRowEnd() * cfg.SensorW * 2

	if endByte > len(pixelData) {
		return nil
	}

	irBytes := pixelData[:endByte]
	result := make([]uint8, len(irBytes)/2)
	for i := range result {
		result[i] = irBytes[i*2]
	}
	return result
}

// ExtractBoth extracts both IR brightness and thermal data from a frame.
func ExtractBoth(
	frameData []byte,
	cfg ModelConfig,
) (ir []uint8, thermal []RawThermalValue) {
	return ExtractIRBrightness(frameData, cfg), ExtractThermalData(frameData, cfg)
}
