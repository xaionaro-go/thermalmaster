package thermalmaster

// CRC-16/CCITT constants.
const (
	crcPolynomial = 0x1021 // CRC-CCITT polynomial (x^16 + x^12 + x^5 + 1)
	crcInit       = 0x0000 // Initial CRC value
	crcHighBit    = 0x8000 // MSB mask for 16-bit CRC
)

// CRC16CCITT computes CRC16-CCITT checksum (polynomial 0x1021, init 0x0000).
func CRC16CCITT(data []byte) uint16 {
	crc := uint16(crcInit)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&crcHighBit != 0 {
				crc = (crc << 1) ^ crcPolynomial
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
