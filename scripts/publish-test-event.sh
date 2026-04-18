#!/bin/bash
# テスト用のディスクアラートイベントをSNSにPublishするスクリプト
# Lambda関数が起動するかを手動で確認するときに使用する

ENDPOINT="http://localhost:4566"
REGION="ap-northeast-1"
ACCOUNT_ID="000000000000"
TOPIC_NAME="disk-high-alert"
TOPIC_ARN="arn:aws:sns:${REGION}:${ACCOUNT_ID}:${TOPIC_NAME}"

# テスト用アラートイベント（使用率92%、閾値80%）
MESSAGE=$(cat <<EOF
{
  "host": "web-01",
  "mount_point": "/var/log",
  "usage_pct": 92.0,
  "threshold": 80.0,
  "alert_type": "disk_high",
  "detected_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
EOF
)

echo "SNSにテストイベントを送信しています..."
echo "メッセージ: $MESSAGE"

aws --endpoint-url=$ENDPOINT sns publish \
  --region $REGION \
  --topic-arn $TOPIC_ARN \
  --message "$MESSAGE"

echo ""
echo "送信完了。Lambdaのログを確認するには:"
echo "  aws --endpoint-url=$ENDPOINT logs describe-log-streams \\"
echo "    --region $REGION \\"
echo "    --log-group-name /aws/lambda/slack-notifier"
