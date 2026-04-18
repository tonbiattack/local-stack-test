package alert_test

import (
	"testing"

	"github.com/tonbiattack/localstack-test/internal/alert"
)

// TestShouldAlert は閾値判定ロジックのテスト
func TestShouldAlert(t *testing.T) {
	tests := []struct {
		name      string
		usagePct  float64
		threshold float64
		want      bool
	}{
		{"使用率が閾値を超えている場合はアラート", 92.0, 80.0, true},
		{"使用率が閾値未満の場合はスキップ", 70.0, 80.0, false},
		{"使用率が閾値と等しい場合はスキップ", 80.0, 80.0, false},
		{"使用率が0の場合はスキップ", 0.0, 80.0, false},
		{"閾値が0の場合は常にアラート（0より大きければ）", 0.1, 0.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alert.ShouldAlert(tt.usagePct, tt.threshold)
			if got != tt.want {
				t.Errorf("ShouldAlert(%v, %v) = %v, want %v", tt.usagePct, tt.threshold, got, tt.want)
			}
		})
	}
}

// TestBuildAlertEvent はSNS送信用イベントの生成テスト
func TestBuildAlertEvent(t *testing.T) {
	t.Run("イベントの各フィールドが正しく設定される", func(t *testing.T) {
		event := alert.BuildAlertEvent("web-01", "/var/log", 92.0, 80.0)

		if event.Host != "web-01" {
			t.Errorf("Host = %v, want web-01", event.Host)
		}
		if event.MountPoint != "/var/log" {
			t.Errorf("MountPoint = %v, want /var/log", event.MountPoint)
		}
		if event.UsagePct != 92.0 {
			t.Errorf("UsagePct = %v, want 92.0", event.UsagePct)
		}
		if event.Threshold != 80.0 {
			t.Errorf("Threshold = %v, want 80.0", event.Threshold)
		}
		if event.AlertType != "disk_high" {
			t.Errorf("AlertType = %v, want disk_high", event.AlertType)
		}
		if event.DetectedAt.IsZero() {
			t.Error("DetectedAt should not be zero")
		}
	})
}
