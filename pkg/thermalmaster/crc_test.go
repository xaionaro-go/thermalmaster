package thermalmaster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCRC16CCITT(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want uint16
	}{
		{
			// CRC-16/XMODEM standard check value.
			name: "standard_check_value",
			data: []byte("123456789"),
			want: 0x31C3,
		},
		{
			// Verified against captured read_name command from the camera protocol.
			// cmdType=0x0101, param=0x0081, register=0x0001, respLen=0x001E
			name: "read_name_payload",
			data: []byte{
				0x01, 0x01, 0x81, 0x00,
				0x01, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x1E, 0x00, 0x00, 0x00,
			},
			want: 0x904F,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CRC16CCITT(tc.data)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCRC16CCITT_Empty(t *testing.T) {
	assert.Equal(t, uint16(0x0000), CRC16CCITT(nil))
	assert.Equal(t, uint16(0x0000), CRC16CCITT([]byte{}))
}
