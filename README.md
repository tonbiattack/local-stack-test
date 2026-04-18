# local-stack-test

LocalStack を使って AWS SNS → Lambda の配線をローカルでテストするサンプルプロジェクトです。

Go でテストファーストに実装しています。

## LocalStack とは

LocalStack は AWS サービスをローカルで再現するツールです。
AWS CLI や SDK からは本物の AWS と同じように操作でき、エンドポイントを `http://localhost:4566` に向けるだけで切り替えられます。

```bash
# 本物の AWS への操作
aws sns create-topic --name my-topic

# LocalStack への操作（--endpoint-url を追加するだけ）
aws --endpoint-url=http://localhost:4566 sns create-topic --name my-topic
```

SNS・Lambda・SQS・Parameter Store・Secrets Manager などを、実際の AWS にデプロイせずにローカルで動作確認できます。
開発サイクルが速くなり、AWS へのデプロイは動くことが確認できてからで済みます。

詳しくは [LocalStack でAWSサービスをローカルで動かす](https://qiita.com/tonbi_attack/items/localstack) で整理しています。

## 構成

```
k8s CronJob（毎時0分）
  → 外部APIからディスク使用率取得
  → Parameter Store の閾値と比較
  → 使用率 > 閾値の場合 → SNS Publish
                           → Lambda 起動
                             → Slack Webhook 通知
```

このプロジェクトでは SNS → Lambda の部分をLocalStackで検証します。

## ディレクトリ構成

```
local-stack-test/
├── internal/
│   ├── alert/         # 閾値判定・イベント生成ロジック（外部依存なし）
│   └── notify/        # Slack通知メッセージ生成ロジック（外部依存なし）
├── integration/       # LocalStackを使った結合テスト
├── lambda/            # Lambda関数のNode.jsコード
├── scripts/           # セットアップ・デプロイスクリプト
├── docker-compose.yml
└── Makefile
```

## 前提条件

- Go 1.21+
- Docker / Docker Compose
- AWS CLI（LocalStack接続用）

AWS CLI の認証情報はダミーでよいです。

```bash
aws configure
# AWS Access Key ID: test
# AWS Secret Access Key: test
# Default region name: ap-northeast-1
# Default output format: json
```

## 事前準備（初回セットアップ）

プロジェクトを動かすには以下のツールが必要です。順番にインストールしてください。

### 1. 必要なパッケージのインストール

```bash
# make（タスクランナー）
sudo apt install make

# zip（Lambdaパッケージの作成に使用）
sudo apt install zip
```

### 2. AWS CLI のインストール

```bash
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o /tmp/awscliv2.zip
unzip /tmp/awscliv2.zip -d /tmp/awscli
sudo /tmp/awscli/aws/install
```

### 3. AWS CLI の設定（ダミー認証情報でOK）

LocalStack はダミーの認証情報でも動作します。`Default output format` は必ず `json` にしてください。

```bash
aws configure
# AWS Access Key ID [None]: test
# AWS Secret Access Key [None]: test
# Default region name [None]: ap-northeast-1
# Default output format [None]: json   ← json と入力すること（空白や他の値は不可）
```

### 4. Docker / Docker Compose のインストール確認

```bash
docker --version
docker compose version
```

インストールされていない場合は [Docker 公式ドキュメント](https://docs.docker.com/engine/install/ubuntu/) を参照してください。

## テストの実行

### 単体テスト（LocalStack不要）

外部依存のないロジックのテストです。

```bash
make test-unit
# または
go test ./internal/... -v
```

### 結合テスト（LocalStack必要）

SNS → Lambda の配線テストです。

```bash
# LocalStackを起動する
make up

# Lambda関数をデプロイする
make deploy

# 結合テストを実行する
make test-integration
```

### 手動でのテスト

```bash
# テストイベントをPublishする
make publish-test

# Lambdaのログを確認する
make logs
```

## CLI でのリソース確認

LocalStack 起動中に以下のコマンドでリソースの状態を確認できます。
`--endpoint-url=http://localhost:4566` を付けることで LocalStack に向けて操作します。

### LocalStack の起動確認

```bash
curl http://localhost:4566/_localstack/health
```

### SNS

```bash
# トピック一覧
aws --endpoint-url=http://localhost:4566 --region ap-northeast-1 sns list-topics

# サブスクリプション一覧
aws --endpoint-url=http://localhost:4566 --region ap-northeast-1 sns list-subscriptions
```

### Lambda

```bash
# 関数一覧
aws --endpoint-url=http://localhost:4566 --region ap-northeast-1 lambda list-functions

# 関数の詳細
aws --endpoint-url=http://localhost:4566 --region ap-northeast-1 lambda get-function --function-name slack-notifier
```

### CloudWatch Logs（Lambdaのログ）

```bash
# ロググループ一覧
aws --endpoint-url=http://localhost:4566 --region ap-northeast-1 logs describe-log-groups

# 最新のログストリーム一覧
aws --endpoint-url=http://localhost:4566 --region ap-northeast-1 logs describe-log-streams \
  --log-group-name /aws/lambda/slack-notifier

# ログの中身を確認（<log-stream-name> は上のコマンドで取得したもの）
aws --endpoint-url=http://localhost:4566 --region ap-northeast-1 logs get-log-events \
  --log-group-name /aws/lambda/slack-notifier \
  --log-stream-name "<log-stream-name>"
```

## テストの段階について

全部つないで一度に動かすと、失敗したときに原因の範囲が絞れません。

例えば「Slack に通知が来ない」という現象が起きたとき、原因の候補は次のとおりです。

- CronJob の閾値判定がおかしい
- 外部 API からのディスク使用率取得が失敗している
- SNS Publish が失敗している
- Lambda が SNS トリガーを受け取っていない
- Lambda が Webhook を叩いていない
- Webhook のリクエスト形式が間違っている

段階を分けると「この段階まで通っている」という確信が積み上がり、問題の範囲を絞り込めます。

| テスト種別 | 確認すること | 必要なもの |
|---|---|---|
| 単体テスト | 閾値判定・イベント生成・文面生成のロジック | なし |
| 結合テスト | SNS → Lambda の配線 | LocalStack |
| E2E テスト | 全フロー・外部 API 異常系 | LocalStack + WireMock |

### 段階1：単体テスト（外部依存なし）

ロジックが正しいかを確認します。Docker も LocalStack も不要なので CI で高速に実行できます。



### 段階2：Lambda 結合テスト（LocalStack）

SNS → Lambda の配線が正しいかを確認します。

この段階で確認することは次のとおりです。

- SNS の Publish で Lambda が起動するか
- Lambda が SNS メッセージをデシリアライズできるか
- Slack Webhook へのリクエストが発行されるか

Lambda 単体の結合テストが通っていれば、Lambda と SNS の連携は問題ないと判断できます。
E2E テストでの失敗が Lambda 側の問題でないと絞り込めます。

```bash
make up
make deploy
make test-integration
```

### 段階3：E2E テスト（LocalStack + WireMock）

CronJob も含めた全フローを確認します。外部 API は WireMock で差し替えます。

WireMock を使う理由は2つあります。接続エラーなどの異常系を安定して再現するためと、実際の外部 API へのアクセスを発生させずテストを外部サービスの状態に左右されないようにするためです。

スタブ定義は `wiremock/mappings/` に置いています。

```bash
# LocalStack と WireMock を起動する
make up-all
make deploy

# 手動でテストイベントを発行して全フローを確認する
make publish-test
make logs
```

なお、LocalStack は AWS 本番の完全代替ではないため、本番相当の最終確認は別途必要です。

この段階的なアプローチの詳細は [LocalStack と WireMock で段階的な結合テストをする](https://qiita.com/tonbi_attack/items/localstack-wiremock) で整理しています。
