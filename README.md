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

| テスト種別 | 対象 | 必要なもの |
|---|---|---|
| 単体テスト | 閾値判定・イベント生成・文面生成のロジック | なし |
| 結合テスト | SNS → Lambda の配線 | LocalStack |

ロジックのテストを外部依存なしで先に固めることで、結合テストで失敗したときの原因を絞り込みやすくなります。
