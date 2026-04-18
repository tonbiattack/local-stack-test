package notify_test

import (
	"testing"

	"github.com/tonbiattack/localstack-test/internal/alert"
	"github.com/tonbiattack/localstack-test/internal/notify"
)

// TestBuildSlackMessage はSlack通知文面の生成テスト
func TestBuildSlackMessage(t *testing.T) {
	t.Run("ホスト名が文面に含まれる", func(t *testing.T) {
		event := alert.BuildAlertEvent("web-01", "/var/log", 92.0, 80.0)
		msg := notify.BuildSlackMessage(event)

		if msg.Text == "" {
			t.Error("Text should not be empty")
		}

		found := false
		for _, a := range msg.Attachments {
			for _, f := range a.Fields {
				if f.Value == "web-01" {
					found = true
				}
			}
		}
		if !found {
			t.Error("host name 'web-01' not found in attachments")
		}
	})

	t.Run("使用率がパーセント表記でフォーマットされている", func(t *testing.T) {
		event := alert.BuildAlertEvent("web-01", "/var/log", 92.0, 80.0)
		msg := notify.BuildSlackMessage(event)

		found := false
		for _, a := range msg.Attachments {
			for _, f := range a.Fields {
				if f.Value == "92.0%" {
					found = true
				}
			}
		}
		if !found {
			t.Error("usage_pct '92.0%' not found in attachments")
		}
	})

	t.Run("カラーはdangerが設定される", func(t *testing.T) {
		event := alert.BuildAlertEvent("web-01", "/var/log", 92.0, 80.0)
		msg := notify.BuildSlackMessage(event)

		if len(msg.Attachments) == 0 {
			t.Fatal("attachments should not be empty")
		}
		if msg.Attachments[0].Color != "danger" {
			t.Errorf("Color = %v, want danger", msg.Attachments[0].Color)
		}
	})
}
