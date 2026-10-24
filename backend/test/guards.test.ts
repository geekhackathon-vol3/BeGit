import { describe, expect, it } from "vitest";
import { isInternalPath, stripInternalHeaders } from "../src/guards";

describe("isInternalPath（#131）", () => {
  it.each([
    "/internal",
    "/internal/cron",
    "/Internal/cron",
    "//internal/cron",
    "/%69nternal/cron",
    "/internal%2Fcron",
    "/%E0%A4%A",
  ])("%s は内部扱い", (path) => {
    expect(isInternalPath(path)).toBe(true);
  });

  it.each(["/groups/1/posts/2/confirm", "/internals", "/me"])("%s は内部扱いではない", (path) => {
    expect(isInternalPath(path)).toBe(false);
  });
});

describe("stripInternalHeaders（#131）", () => {
  it("内部用ヘッダーを取り除き、ほかは残す", () => {
    const headers = stripInternalHeaders(new Headers({
      Authorization: "Bearer dev_user001",
      "X-Internal-Cron-Secret": "x",
      "x-internal-dev-mode": "true",
      "X-Cron-Secret": "x",
      "X-Notification-Queue-Secret": "x",
    }));
    expect([...headers.keys()]).toEqual(["authorization"]);
  });
});
