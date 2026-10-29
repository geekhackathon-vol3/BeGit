import { afterEach, describe, expect, it, vi } from "vitest";
import type { NotificationQueueMessage } from "../src/api/types";
import { deliverNotifications } from "../src/queue";

const BASE = "https://begit.example";
const HEADERS = { "X-Internal-Dev-Mode": "", "X-Notification-Queue-Secret": "s3cret" };

interface FakeMessage {
  body: NotificationQueueMessage;
  acked: boolean;
  retried: boolean;
  retryDelay?: number;
  ack(): void;
  retry(options?: { delaySeconds?: number }): void;
}

function batchOf(...bodies: NotificationQueueMessage[]): { batch: MessageBatch<NotificationQueueMessage>; messages: FakeMessage[] } {
  const messages = bodies.map((body): FakeMessage => ({
    body,
    acked: false,
    retried: false,
    ack() { this.acked = true; },
    retry(options) { this.retried = true; this.retryDelay = options?.delaySeconds; },
  }));
  return { batch: { messages } as unknown as MessageBatch<NotificationQueueMessage>, messages };
}

// fakeContainer は受けたリクエストを記録し、respond の結果を返す
function fakeContainer(respond: (request: Request) => Response | Promise<Response>) {
  const requests: Request[] = [];
  return {
    requests,
    fetch: async (request: Request) => {
      requests.push(request.clone());
      return respond(request);
    },
  };
}

const reaction: NotificationQueueMessage = { type: "reaction", group_id: 12, post_id: 890, actor_user_id: 2, author_user_id: 99 };
const beGitTime: NotificationQueueMessage = { type: "begit_time", group_id: 12, notification_id: 345, sprint_id: 7 };

describe("deliverNotifications（#134）", () => {
  afterEach(() => vi.restoreAllMocks());

  it("job_id は今までどおり外部通知の配信に渡す", async () => {
    const container = fakeContainer(() => new Response(null, { status: 204 }));
    const { batch, messages } = batchOf({ job_id: 42 });
    await deliverNotifications(batch, container, BASE, HEADERS);

    expect(container.requests).toHaveLength(1);
    const [request] = container.requests;
    expect(request.method).toBe("POST");
    expect(request.url).toBe(`${BASE}/internal/notification-deliveries/42`);
    expect(request.headers.get("X-Notification-Queue-Secret")).toBe("s3cret");
    expect(await request.text()).toBe("");
    expect(messages[0].acked).toBe(true);
  });

  it.each([reaction, beGitTime])("type=$type は FCM の受け口にメッセージをそのまま渡す", async (body) => {
    vi.spyOn(console, "log").mockImplementation(() => {});
    const container = fakeContainer(() => new Response(null, { status: 204 }));
    const { batch, messages } = batchOf(body);
    await deliverNotifications(batch, container, BASE, HEADERS);

    const [request] = container.requests;
    expect(request.url).toBe(`${BASE}/internal/push-notifications`);
    expect(request.headers.get("X-Notification-Queue-Secret")).toBe("s3cret");
    expect(request.headers.get("Content-Type")).toBe("application/json");
    expect(await request.json()).toEqual(body);
    expect(messages[0].acked).toBe(true);
  });

  it.each([
    { status: 400, acked: true, retried: false },
    { status: 422, acked: true, retried: false },
    { status: 404, acked: true, retried: false },
    { status: 503, acked: false, retried: true },
    { status: 500, acked: false, retried: true },
  ])("応答 $status は 4xx なら捨て、5xx なら間隔を空けて再送する（job_id と type で同じ）", async ({ status, acked, retried }) => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    const container = fakeContainer(() => new Response("x", { status }));
    const { batch, messages } = batchOf({ job_id: 1 }, reaction);
    await deliverNotifications(batch, container, BASE, HEADERS);

    for (const message of messages) {
      expect({ acked: message.acked, retried: message.retried }).toEqual({ acked, retried });
      if (retried) expect(message.retryDelay).toBeGreaterThan(0);
    }
  });

  it("コンテナに届かなければ再送し、残りのメッセージも処理する", async () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    vi.spyOn(console, "log").mockImplementation(() => {});
    let calls = 0;
    const container = fakeContainer(() => {
      calls++;
      if (calls === 1) throw new Error("container unavailable");
      return new Response(null, { status: 204 });
    });
    const { batch, messages } = batchOf(reaction, beGitTime);
    await deliverNotifications(batch, container, BASE, HEADERS);

    expect(messages[0]).toMatchObject({ acked: false, retried: true });
    expect(messages[1]).toMatchObject({ acked: true, retried: false });
  });
});
