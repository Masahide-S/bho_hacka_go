package monitor

// FullSnapshot はDB保存用に全データをまとめた構造体です
type FullSnapshot struct {
	System    SystemResources
	Processes []ProcessInfo
}
