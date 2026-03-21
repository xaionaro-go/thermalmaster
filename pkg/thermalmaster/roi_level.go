package thermalmaster

// ROILevel represents a region-of-interest AGC level.
type ROILevel uint8

const (
	ROIDisable ROILevel = 0
	ROIThird   ROILevel = 1
	ROIHalf    ROILevel = 2
)
