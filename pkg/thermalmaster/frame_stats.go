package thermalmaster

// FrameStats tracks frame statistics.
type FrameStats struct {
	FramesRead       uint64
	FramesDropped    uint64
	MarkerMismatches uint64
	LastCnt1         uint32
	LastCnt3         uint16
}
