// alertパッケージはディスク使用率の閾値判定とアラートイベント生成を担当する
package alert

import "time"

// AlertEvent はSNSに送信するディスクアラートのイベントデータ
type AlertEvent struct {
	Host       string    `json:"host"`
	MountPoint string    `json:"mount_point"`
	UsagePct   float64   `json:"usage_pct"`
	Threshold  float64   `json:"threshold"`
	AlertType  string    `json:"alert_type"`
	DetectedAt time.Time `json:"detected_at"`
}

// ShouldAlert はディスク使用率が閾値を超えているかを判定する
// 使用率が閾値より大きい場合にtrueを返す（等しい場合はfalse）
func ShouldAlert(usagePct, threshold float64) bool {
	return usagePct > threshold
}

// BuildAlertEvent はSNS送信用のアラートイベントを生成する
func BuildAlertEvent(host, mountPoint string, usagePct, threshold float64) AlertEvent {
	return AlertEvent{
		Host:       host,
		MountPoint: mountPoint,
		UsagePct:   usagePct,
		Threshold:  threshold,
		AlertType:  "disk_high",
		DetectedAt: time.Now(),
	}
}
