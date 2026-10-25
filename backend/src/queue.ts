import type { NotificationQueueMessage } from "./api/types";

// RETRY_DELAY_SECONDS は再送までの間隔。間隔なしだと、FCM やコンテナが数十秒止まっただけで
// max_retries（wrangler.toml）を使い切って DLQ に入るため、少し空ける
const RETRY_DELAY_SECONDS = 10;

// QueueContainer は queue() が通知を渡す先（Go の裏方のコンテナ。テストでは差し替える）
export interface QueueContainer {
  fetch(request: Request): Promise<Response>;
}

// deliverNotifications は NOTIFICATION_QUEUE のメッセージを Go の裏方のコンテナに渡す。
//   { job_id }: 外部通知の配信  → POST /internal/notification-deliveries/:jobId
//   { type }  : FCM の通知（#134）→ POST /internal/push-notifications（本文はメッセージそのまま。検証は Go が行う）
// 応答の扱いはどちらも同じ：2xx は完了、4xx は再送しても変わらないので捨てる、5xx と通信の失敗は間隔を空けて再送する。
// headers には内部用ヘッダーと X-Notification-Queue-Secret を入れて渡す。
export async function deliverNotifications(
  batch: MessageBatch<NotificationQueueMessage>,
  container: QueueContainer,
  baseURL: string,
  headers: Record<string, string>,
): Promise<void> {
  for (const message of batch.messages) {
    const body = message.body;
    const isJob = "job_id" in body;
    const label = isJob ? `job_id=${body.job_id}` : `type=${body.type}`;
    const request = isJob
      ? new Request(`${baseURL}/internal/notification-deliveries/${body.job_id}`, { method: "POST", headers })
      : new Request(`${baseURL}/internal/push-notifications`, {
        method: "POST",
        headers: { ...headers, "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });

    try {
      const response = await container.fetch(request);
      if (response.ok) {
        if (!isJob) console.log(`[notification-queue] push done ${label} status=${response.status}`);
        message.ack();
        continue;
      }
      const responseBody = (await response.text().catch(() => "")).slice(0, 500);
      console.error(`[notification-queue] delivery failed ${label} status=${response.status} body=${responseBody}`);
      if (response.status >= 400 && response.status < 500) {
        message.ack();
      } else {
        message.retry({ delaySeconds: RETRY_DELAY_SECONDS });
      }
    } catch (error) {
      console.error(`[notification-queue] delivery request failed ${label}`, error);
      message.retry({ delaySeconds: RETRY_DELAY_SECONDS });
    }
  }
}
