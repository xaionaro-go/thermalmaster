package thermalmaster

import (
	"encoding/binary"
	"fmt"
)

// parseSingleByteResponse extracts a single byte from a command response.
// Most VDCMD Get commands return exactly 1 byte.
func parseSingleByteResponse(resp []byte) (uint8, error) {
	if len(resp) < 1 {
		return 0, fmt.Errorf("response too short: got %d bytes, need at least 1", len(resp))
	}
	return resp[0], nil
}

// parseUint16Response extracts a little-endian uint16 from the first 2 bytes
// of a command response.
func parseUint16Response(resp []byte) (uint16, error) {
	if len(resp) < 2 {
		return 0, fmt.Errorf("response too short: got %d bytes, need at least 2", len(resp))
	}
	return binary.LittleEndian.Uint16(resp[:2]), nil
}

// parseInt32Response extracts a little-endian int32 from the first 4 bytes
// of a command response.
func parseInt32Response(resp []byte) (int32, error) {
	if len(resp) < 4 {
		return 0, fmt.Errorf("response too short: got %d bytes, need at least 4", len(resp))
	}
	return int32(binary.LittleEndian.Uint32(resp[:4])), nil
}
