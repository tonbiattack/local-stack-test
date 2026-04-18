// e2eテストはLocalStack + WireMockを使った全フローの結合テスト
// 実行前にLocalStackとWireMockが起動していること（docker compose up localstack wiremock）
package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/tonbiattack/localstack-test/internal/alert"
)

const (
	// WireMockのデフォルトエンドポイント
	wiremockEndpoint = "http://localhost:8080"
)

// wiremockHealthCheck はWireMockが起動しているか確認する
func wiremockHealthCheck(t *testing.T) {
	t.Helper()
	resp, err := http.Get(wiremockEndpoint + "/__admin/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Skip("WireMockが起動していません（docker compose up wiremock が必要）")
	}
}

// TestDiskAPIStub はWireMockが外部APIのスタブとして機能することを確認する
func TestDiskAPIStub(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("INTEGRATION_TEST=true の場合のみ実行します")
	}
	wiremockHealthCheck(t)

	t.Run("正常系：ディスク使用率を返す", func(t *testing.T) {
		resp, err := http.Get(wiremockEndpoint + "/api/hosts/web-01/disk")
		if err != nil {
			t.Fatalf("WireMockへのリクエストに失敗しました: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
		}

		if result["host"] != "web-01" {
			t.Errorf("host = %v, want web-01", result["host"])
		}
		if result["usage_pct"] != 92.0 {
			t.Errorf("usage_pct = %v, want 92.0", result["usage_pct"])
		}
		t.Logf("正常系レスポンス確認: %v", result)
	})

	t.Run("異常系：接続エラーを再現する", func(t *testing.T) {
		_, err := http.Get(wiremockEndpoint + "/api/hosts/web-error/disk")
		if err == nil {
			t.Error("接続エラーが期待されましたが、リクエストが成功しました")
		}
		t.Logf("期待通り接続エラーが発生しました: %v", err)
	})
}

// TestFullFlow はSNS → Lambda → Slack Webhook の全フローを確認する
// WireMockがSlack Webhookのスタブとしても機能する
func TestFullFlow(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("INTEGRATION_TEST=true の場合のみ実行します")
	}
	wiremockHealthCheck(t)

	// WireMockにSlack Webhookのスタブを動的に登録する
	setupSlackWebhookStub(t)

	cfg := localstackConfig(t)
	snsClient := sns.NewFromConfig(cfg)
	logsClient := cloudwatchlogs.NewFromConfig(cfg)
	ctx := context.Background()

	topicName := "disk-high-alert"

	// SNSトピックを取得（deploy済みのトピックを使う）
	topicArn := topicARN(topicName)

	t.Run("閾値超過イベントをPublishするとLambdaが起動する", func(t *testing.T) {
		event := alert.BuildAlertEvent("web-01", "/var/log", 92.0, 80.0)
		payload, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("イベントのJSON変換に失敗しました: %v", err)
		}

		_, err = snsClient.Publish(ctx, &sns.PublishInput{
			TopicArn: aws.String(topicArn),
			Message:  aws.String(string(payload)),
		})
		if err != nil {
			t.Fatalf("SNS Publishに失敗しました: %v", err)
		}

		// Lambdaの起動を待つ（非同期のため）
		time.Sleep(3 * time.Second)

		// CloudWatch LogsでLambda起動を確認する
		logGroupName := fmt.Sprintf("/aws/lambda/slack-notifier")
		logsOut, err := logsClient.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: aws.String(logGroupName),
			OrderBy:      "LastEventTime",
			Descending:   aws.Bool(true),
			Limit:        aws.Int32(1),
		})
		if err != nil {
			t.Fatalf("CloudWatch Logsの取得に失敗しました: %v", err)
		}

		if len(logsOut.LogStreams) == 0 {
			t.Error("Lambdaのログストリームがありません。Lambdaが起動していない可能性があります")
		} else {
			t.Logf("Lambda起動を確認: %s", *logsOut.LogStreams[0].LogStreamName)
		}
	})

	t.Run("WireMockがSlack Webhookのリクエストを受信している", func(t *testing.T) {
		// WireMockのリクエスト受信履歴を確認する
		received := verifySlackWebhookCalled(t)
		if !received {
			t.Error("Slack Webhookへのリクエストが記録されていません")
		}
	})
}

// setupSlackWebhookStub はWireMockにSlack Webhookのスタブを動的に登録する
func setupSlackWebhookStub(t *testing.T) {
	t.Helper()

	stubDef := `{
		"request": {
			"method": "POST",
			"url": "/slack/webhook"
		},
		"response": {
			"status": 200,
			"body": "ok",
			"headers": { "Content-Type": "text/plain" }
		}
	}`

	resp, err := http.Post(
		wiremockEndpoint+"/__admin/mappings",
		"application/json",
		strings.NewReader(stubDef),
	)
	if err != nil {
		t.Fatalf("WireMockへのスタブ登録に失敗しました: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("スタブ登録のレスポンスが異常です: %d", resp.StatusCode)
	}
	t.Log("Slack Webhookスタブを登録しました")
}

// verifySlackWebhookCalled はWireMockがSlack Webhookへのリクエストを受信したか確認する
func verifySlackWebhookCalled(t *testing.T) bool {
	t.Helper()

	resp, err := http.Get(wiremockEndpoint + "/__admin/requests")
	if err != nil {
		t.Fatalf("WireMockのリクエスト履歴取得に失敗しました: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return strings.Contains(string(body), "/slack/webhook")
}
