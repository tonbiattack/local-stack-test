// notifyパッケージはSlack通知メッセージの生成を担当する
package notify

import (
	"fmt"

	"github.com/tonbiattack/localstack-test/internal/alert"
)

// SlackMessage はSlack Webhookへ送信するメッセージ構造体
type SlackMessage struct {
	Text        string       `json:"text"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Attachment はSlackメッセージの添付情報
type Attachment struct {
	Color  string  `json:"color"`
	Fields []Field `json:"fields"`
}

// Field はAttachment内の各フィールド
type Field struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// BuildSlackMessage はアラートイベントからSlack通知メッセージを生成する
func BuildSlackMessage(event alert.AlertEvent) SlackMessage {
	return SlackMessage{
		Text: fmt.Sprintf("【ディスクアラート】%s のディスク使用率が閾値を超えました", event.Host),
		Attachments: []Attachment{
			{
				Color: "danger",
				Fields: []Field{
					{Title: "ホスト", Value: event.Host, Short: true},
					{Title: "マウントポイント", Value: event.MountPoint, Short: true},
					{Title: "使用率", Value: fmt.Sprintf("%.1f%%", event.UsagePct), Short: true},
					{Title: "閾値", Value: fmt.Sprintf("%.1f%%", event.Threshold), Short: true},
				},
			},
		},
	}
}
