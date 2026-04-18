# E2Eテストが動かなかった原因と修正

E2Eテスト（LocalStack + WireMock）を動かす際に発生した問題と、その修正内容を記録します。

## 問題1：Lambda が WireMock の Slack Webhook へ接続できない

### 症状

`make test-e2e` を実行すると、Lambda は起動するが Slack Webhook へのリクエストが WireMock に届かない。

### 原因

2つの原因が重なっていました。

**原因A：Webhook URL に `http://` を使っているのに `https` モジュールで接続していた**

`lambda/index.js` の `sendToSlack` 関数は `https.request` を固定で使っていました。
WireMock のエンドポイントは `http://` のため、プロトコル不一致で接続が失敗していました。

```javascript
// 修正前：https固定
const req = https.request(options, (res) => { ... });

// 修正後：URLのプロトコルに応じて切り替える
const client = url.protocol === "https:" ? https : http;
const req = client.request(options, (res) => { ... });
```

あわせてポート番号の明示も追加しました。

```javascript
port: url.port || (url.protocol === "https:" ? 443 : 80),
```

**原因B：Webhook URL に `localhost` を指定していたため Docker コンテナ内から届かなかった**

`deploy-lambda.sh` の環境変数に `http://localhost:8080/slack/webhook` を設定していました。
Lambda は LocalStack のコンテナ内で動作するため、`localhost` はコンテナ自身を指してしまい WireMock に届きません。

Docker Compose のネットワーク内では、サービス名（`wiremock`）でコンテナ間通信ができます。

```bash
# 修正前
--environment "Variables={SLACK_WEBHOOK_URL=http://localhost:8080/slack/webhook}"

# 修正後
--environment "Variables={SLACK_WEBHOOK_URL=http://wiremock:8080/slack/webhook}"
```

### 修正ファイル

| ファイル | 修正内容 |
|---|---|
| `lambda/index.js` | `http`/`https` をURLのプロトコルに応じて切り替えるよう修正 |
| `scripts/deploy-lambda.sh` | Webhook URL を `localhost` から `wiremock`（サービス名）に変更 |

## 問題2：Lambda の更新時に環境変数が反映されない

### 症状

`make deploy` を2回目以降に実行したとき、Lambda のコードは更新されるが環境変数（`SLACK_WEBHOOK_URL`）が古い値のままになる。

### 原因

`deploy-lambda.sh` の更新処理（`update-function-code`）はコードのみ更新し、環境変数を更新していませんでした。

### 修正

更新処理に `update-function-configuration` を追加しました。

```bash
aws --endpoint-url=$ENDPOINT lambda update-function-configuration \
  --region $REGION \
  --function-name $FUNCTION_NAME \
  --environment "Variables={SLACK_WEBHOOK_URL=http://wiremock:8080/slack/webhook}"
```

## E2Eテストを動かす手順

修正後の正しい手順は次のとおりです。

```bash
# LocalStack と WireMock を起動する
make up-all

# Lambda をデプロイする（初回・更新ともに同じコマンド）
make deploy

# E2E テストを実行する
make test-e2e
```

## 教訓

- Docker コンテナ間の通信には `localhost` ではなくサービス名を使う
- Lambda から外部 HTTP エンドポイントへ接続する場合、プロトコル（http/https）を意識する
- デプロイスクリプトはコードだけでなく環境変数の更新も含める
