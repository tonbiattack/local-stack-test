#!/bin/bash
# Lambda関数をLocalStackにデプロイするスクリプト
# 初回セットアップや関数コードの更新時に使用する

set -e

ENDPOINT="http://localhost:4566"
REGION="ap-northeast-1"
ACCOUNT_ID="000000000000"
FUNCTION_NAME="slack-notifier"
TOPIC_NAME="disk-high-alert"

echo "=== Lambda デプロイ開始 ==="

# Lambda関数のzipを作成する
echo "zipを作成しています..."
zip -j /tmp/slack-notifier.zip lambda/index.js
echo "zip作成完了: /tmp/slack-notifier.zip"

# Lambda関数が存在するか確認する
FUNCTION_EXISTS=$(aws --endpoint-url=$ENDPOINT lambda get-function \
  --region $REGION \
  --function-name $FUNCTION_NAME \
  --query 'Configuration.FunctionName' \
  --output text 2>/dev/null || echo "")

if [ -z "$FUNCTION_EXISTS" ]; then
  # 存在しない場合は新規作成する
  echo "Lambda関数を新規作成しています..."
  aws --endpoint-url=$ENDPOINT lambda create-function \
    --region $REGION \
    --function-name $FUNCTION_NAME \
    --runtime nodejs18.x \
    --handler index.handler \
    --zip-file fileb:///tmp/slack-notifier.zip \
    --role "arn:aws:iam::${ACCOUNT_ID}:role/lambda-role" \
    --environment "Variables={SLACK_WEBHOOK_URL=http://localhost:8080/slack/webhook}"
  echo "Lambda関数作成完了"
else
  # 存在する場合はコードを更新する
  echo "Lambda関数のコードを更新しています..."
  aws --endpoint-url=$ENDPOINT lambda update-function-code \
    --region $REGION \
    --function-name $FUNCTION_NAME \
    --zip-file fileb:///tmp/slack-notifier.zip
  echo "Lambda関数更新完了"
fi

# SNSトピックを作成する（存在する場合はARNを取得する）
echo "SNSトピックを確認しています..."
TOPIC_ARN=$(aws --endpoint-url=$ENDPOINT sns create-topic \
  --region $REGION \
  --name $TOPIC_NAME \
  --query TopicArn \
  --output text)
echo "SNSトピックARN: $TOPIC_ARN"

# SNS → Lambda のサブスクリプションを設定する
FUNCTION_ARN="arn:aws:lambda:${REGION}:${ACCOUNT_ID}:function:${FUNCTION_NAME}"
echo "SNS → Lambda サブスクリプションを設定しています..."
aws --endpoint-url=$ENDPOINT sns subscribe \
  --region $REGION \
  --topic-arn $TOPIC_ARN \
  --protocol lambda \
  --notification-endpoint $FUNCTION_ARN
echo "サブスクリプション設定完了"

echo "=== デプロイ完了 ==="
echo ""
echo "テストメッセージを送信するには:"
echo "  bash scripts/publish-test-event.sh"
