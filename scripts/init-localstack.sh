#!/bin/bash
# LocalStack起動時に実行されるセットアップスクリプト
# SNSトピックとLambda関数を自動作成する

set -e

ENDPOINT="http://localhost:4566"
REGION="ap-northeast-1"
ACCOUNT_ID="000000000000"

echo "=== LocalStack セットアップ開始 ==="

# Lambda関数のzipを作成する
echo "Lambda関数のzipを作成しています..."
cd /etc/localstack/init/ready.d
zip -j /tmp/slack-notifier.zip /var/lib/localstack/lambda/index.js 2>/dev/null || true

# Lambda関数を作成する
echo "Lambda関数を作成しています..."
aws --endpoint-url=$ENDPOINT lambda create-function \
  --region $REGION \
  --function-name slack-notifier \
  --runtime nodejs18.x \
  --handler index.handler \
  --zip-file fileb:///tmp/slack-notifier.zip \
  --role "arn:aws:iam::${ACCOUNT_ID}:role/lambda-role" \
  --environment "Variables={SLACK_WEBHOOK_URL=http://wiremock:8080/slack/webhook}" \
  2>/dev/null && echo "Lambda関数作成完了" || echo "Lambda関数はすでに存在します"

# SNSトピックを作成する
echo "SNSトピックを作成しています..."
TOPIC_ARN=$(aws --endpoint-url=$ENDPOINT sns create-topic \
  --region $REGION \
  --name disk-high-alert \
  --query TopicArn \
  --output text)
echo "SNSトピック作成完了: $TOPIC_ARN"

# SNS → Lambda のサブスクリプションを設定する
echo "SNS → Lambda サブスクリプションを設定しています..."
FUNCTION_ARN="arn:aws:lambda:${REGION}:${ACCOUNT_ID}:function:slack-notifier"
aws --endpoint-url=$ENDPOINT sns subscribe \
  --region $REGION \
  --topic-arn $TOPIC_ARN \
  --protocol lambda \
  --notification-endpoint $FUNCTION_ARN
echo "サブスクリプション設定完了"

echo "=== LocalStack セットアップ完了 ==="
