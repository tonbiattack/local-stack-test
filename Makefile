.PHONY: test test-unit test-integration test-e2e up up-all down deploy publish-test logs

# 単体テストを実行する（LocalStack不要）
test-unit:
	go test ./internal/... -v

# 結合テストを実行する（LocalStack必要）
test-integration:
	INTEGRATION_TEST=true go test ./integration/... -run "TestSNS" -v -timeout 30s

# E2Eテストを実行する（LocalStack + WireMock必要）
test-e2e:
	INTEGRATION_TEST=true go test ./integration/... -run "TestDiskAPI|TestFullFlow" -v -timeout 60s

# 全テストを実行する
test: test-unit test-integration test-e2e

# LocalStackのみ起動する
up:
	docker compose up -d localstack

# LocalStackとWireMockを起動する（E2Eテスト用）
up-all:
	docker compose up -d localstack wiremock

# 全サービスを停止する
down:
	docker compose down

# Lambda関数をLocalStackにデプロイする
deploy:
	bash scripts/deploy-lambda.sh

# テストイベントをSNSにPublishする
publish-test:
	bash scripts/publish-test-event.sh

# Lambda関数のログを確認する
logs:
	aws --endpoint-url=http://localhost:4566 logs describe-log-streams \
		--region ap-northeast-1 \
		--log-group-name /aws/lambda/slack-notifier \
		--order-by LastEventTime \
		--descending \
		--query 'logStreams[0].logStreamName' \
		--output text | xargs -I{} \
		aws --endpoint-url=http://localhost:4566 logs get-log-events \
		--region ap-northeast-1 \
		--log-group-name /aws/lambda/slack-notifier \
		--log-stream-name {}
