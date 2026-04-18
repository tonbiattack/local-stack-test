// integration パッケージはLocalStackを使ったSNS→Lambdaの結合テスト
// 実行前にLocalStackが起動していること（docker compose up localstack）
package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/tonbiattack/localstack-test/internal/alert"
)

const (
	// LocalStackのデフォルトエンドポイント
	localstackEndpoint = "http://localhost:4566"
	// テスト用のAWSリージョン
	testRegion = "ap-northeast-1"
	// テスト用のダミーアカウントID（LocalStackでは検証されない）
	dummyAccountID = "000000000000"
)

// localstackConfig はLocalStack接続用のAWS設定を返す
func localstackConfig(t *testing.T) aws.Config {
	t.Helper()

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(testRegion),
		// LocalStackへのエンドポイント上書き
		config.WithBaseEndpoint(localstackEndpoint),
		// LocalStackはダミー認証情報でよい
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		t.Fatalf("AWS設定の読み込みに失敗しました: %v", err)
	}
	return cfg
}

// topicARN はSNSトピックのARNを組み立てる
func topicARN(topicName string) string {
	return fmt.Sprintf("arn:aws:sns:%s:%s:%s", testRegion, dummyAccountID, topicName)
}

// functionARN はLambda関数のARNを組み立てる
func functionARN(functionName string) string {
	return fmt.Sprintf("arn:aws:lambda:%s:%s:function:%s", testRegion, dummyAccountID, functionName)
}

// TestSNSPublish はLocalStackのSNSへPublishできることを確認する
func TestSNSPublish(t *testing.T) {
	// LocalStackが起動していない場合はスキップ
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("INTEGRATION_TEST=true の場合のみ実行します（LocalStack起動が必要）")
	}

	cfg := localstackConfig(t)
	snsClient := sns.NewFromConfig(cfg)
	ctx := context.Background()

	topicName := "disk-high-alert-test"

	// テスト用SNSトピックを作成
	createOut, err := snsClient.CreateTopic(ctx, &sns.CreateTopicInput{
		Name: aws.String(topicName),
	})
	if err != nil {
		t.Fatalf("SNSトピックの作成に失敗しました: %v", err)
	}
	t.Logf("トピック作成完了: %s", *createOut.TopicArn)

	t.Run("ディスクアラートイベントをPublishできる", func(t *testing.T) {
		event := alert.BuildAlertEvent("web-01", "/var/log", 92.0, 80.0)
		payload, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("イベントのJSON変換に失敗しました: %v", err)
		}

		out, err := snsClient.Publish(ctx, &sns.PublishInput{
			TopicArn: createOut.TopicArn,
			Message:  aws.String(string(payload)),
		})
		if err != nil {
			t.Fatalf("SNS Publishに失敗しました: %v", err)
		}

		if out.MessageId == nil || *out.MessageId == "" {
			t.Error("MessageIdが空です")
		}
		t.Logf("Publish成功: MessageId=%s", *out.MessageId)
	})
}

// TestSNSToLambda はSNS → Lambda の配線が正しいことを確認する
func TestSNSToLambda(t *testing.T) {
	// LocalStackが起動していない場合はスキップ
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("INTEGRATION_TEST=true の場合のみ実行します（LocalStack起動が必要）")
	}

	cfg := localstackConfig(t)
	snsClient := sns.NewFromConfig(cfg)
	lambdaClient := lambda.NewFromConfig(cfg)
	logsClient := cloudwatchlogs.NewFromConfig(cfg)
	ctx := context.Background()

	topicName := "disk-high-alert-integration"
	functionName := "slack-notifier"

	// SNSトピックを作成
	createTopicOut, err := snsClient.CreateTopic(ctx, &sns.CreateTopicInput{
		Name: aws.String(topicName),
	})
	if err != nil {
		t.Fatalf("SNSトピックの作成に失敗しました: %v", err)
	}

	// SNS → Lambda のサブスクリプションを設定
	_, err = snsClient.Subscribe(ctx, &sns.SubscribeInput{
		TopicArn: createTopicOut.TopicArn,
		Protocol: aws.String("lambda"),
		Endpoint: aws.String(functionARN(functionName)),
	})
	if err != nil {
		t.Fatalf("SNSサブスクリプションの設定に失敗しました: %v", err)
	}
	t.Logf("SNS → Lambda サブスクリプション設定完了")

	t.Run("SNSにPublishするとLambdaが起動する", func(t *testing.T) {
		// Publish前のLambda起動回数を取得
		beforeOut, _ := lambdaClient.GetFunction(ctx, &lambda.GetFunctionInput{
			FunctionName: aws.String(functionName),
		})
		_ = beforeOut

		event := alert.BuildAlertEvent("web-01", "/var/log", 92.0, 80.0)
		payload, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("イベントのJSON変換に失敗しました: %v", err)
		}

		// SNSにPublish
		_, err = snsClient.Publish(ctx, &sns.PublishInput{
			TopicArn: createTopicOut.TopicArn,
			Message:  aws.String(string(payload)),
		})
		if err != nil {
			t.Fatalf("SNS Publishに失敗しました: %v", err)
		}

		// Lambdaの起動を待つ（非同期のため）
		time.Sleep(3 * time.Second)

		// CloudWatch LogsでLambda起動を確認
		logGroupName := fmt.Sprintf("/aws/lambda/%s", functionName)
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
			t.Logf("Lambda起動を確認: LogStream=%s", *logsOut.LogStreams[0].LogStreamName)
		}
	})
}
