package models

// SpeedTestResult represents a single speed test result
type SpeedTestResult struct {
	IP         string
	Status     string // 待测试, 检测数据中心, 测试中, 已完成, 无效, 跳过
	Latency    string // ms
	Speed      string // Mbps
	DataCenter string
	PeakSpeed  float64 // Mbps
}

// TestConfig holds the test configuration
type TestConfig struct {
	ExpectedServers int
	UseTLS          bool
	IPType          string // ipv4 or ipv6
	Bandwidth       float64
	Timeout         int
	DownloadTime    int
	FilePath        string
	SelectedDC      string
}

// TestStats holds testing statistics
type TestStats struct {
	Total        int
	Completed    int
	Qualified    int
	CurrentIP    string
	CurrentSpeed string
}
