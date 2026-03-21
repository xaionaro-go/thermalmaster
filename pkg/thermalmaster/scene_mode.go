package thermalmaster

// SceneMode represents an image processing scene mode.
type SceneMode uint8

const (
	SceneNormal   SceneMode = 0
	SceneCity     SceneMode = 1
	SceneJungle   SceneMode = 2
	SceneBird     SceneMode = 3
	SceneNormal50 SceneMode = 4 // 50Hz variant
	SceneCity50   SceneMode = 5 // 50Hz variant
	SceneJungle50 SceneMode = 6 // 50Hz variant
	SceneBird50   SceneMode = 7 // 50Hz variant
	SceneRainFog  SceneMode = 8
)
