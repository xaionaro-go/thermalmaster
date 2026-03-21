package thermalmaster

// Marker represents a 12-byte frame marker.
type Marker struct {
	Length uint8
	Sync   uint8
	Cnt1   uint32
	Cnt2   uint32
	Cnt3   uint16
}

// Marker sync byte values.
const (
	SyncStartEven = 0x8C
	SyncStartOdd  = 0x8D
	SyncEndEven   = 0x8E
	SyncEndOdd    = 0x8F
	Cnt3Increment = 40
	Cnt3Wrap      = 2048
)
