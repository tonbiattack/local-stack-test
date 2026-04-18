// slack-notifier Lambda関数
// SNSからディスクアラートイベントを受け取り、Slack Webhookへ通知する

const https = require("https");
const http = require("http");
const { URL } = require("url");

/**
 * Slack通知メッセージを組み立てる
 * @param {Object} event - ディスクアラートイベント
 * @returns {Object} Slack Webhookへ送信するメッセージ
 */
function buildSlackMessage(event) {
  return {
    text: `【ディスクアラート】${event.host} のディスク使用率が閾値を超えました`,
    attachments: [
      {
        color: "danger",
        fields: [
          { title: "ホスト", value: event.host, short: true },
          { title: "マウントポイント", value: event.mount_point, short: true },
          { title: "使用率", value: `${event.usage_pct}%`, short: true },
          { title: "閾値", value: `${event.threshold}%`, short: true },
        ],
      },
    ],
  };
}

/**
 * Slack WebhookへHTTP POSTする
 * @param {string} webhookUrl - Slack Webhook URL
 * @param {Object} message - 送信するメッセージ
 * @returns {Promise<void>}
 */
function sendToSlack(webhookUrl, message) {
  return new Promise((resolve, reject) => {
    const payload = JSON.stringify(message);
    const url = new URL(webhookUrl);

    const options = {
      hostname: url.hostname,
      port: url.port || (url.protocol === "https:" ? 443 : 80),
      path: url.pathname,
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Content-Length": Buffer.byteLength(payload),
      },
    };

    const client = url.protocol === "https:" ? https : http;
    const req = client.request(options, (res) => {
      let body = "";
      res.on("data", (chunk) => (body += chunk));
      res.on("end", () => {
        if (res.statusCode === 200) {
          resolve();
        } else {
          reject(new Error(`Slack returned ${res.statusCode}: ${body}`));
        }
      });
    });

    req.on("error", reject);
    req.write(payload);
    req.end();
  });
}

/**
 * Lambdaハンドラー
 * SNSイベントを受け取り、Slack Webhookへ通知する
 */
exports.handler = async (event) => {
  const webhookUrl = process.env.SLACK_WEBHOOK_URL;

  if (!webhookUrl) {
    console.error("SLACK_WEBHOOK_URL が設定されていません");
    throw new Error("SLACK_WEBHOOK_URL is required");
  }

  for (const record of event.Records) {
    const alertEvent = JSON.parse(record.Sns.Message);
    console.log("受信したアラートイベント:", JSON.stringify(alertEvent));

    const message = buildSlackMessage(alertEvent);
    await sendToSlack(webhookUrl, message);
    console.log("Slack通知送信完了:", alertEvent.host);
  }
};

// テスト用にエクスポート
module.exports.buildSlackMessage = buildSlackMessage;
