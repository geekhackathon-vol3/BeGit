// readBodyUpTo はボディを max バイトまで読む。超えたら残りを読まずに null を返す（Content-Length がない場合に備える）
export async function readBodyUpTo(request: Request, max: number): Promise<Uint8Array | null> {
  if (!request.body) return new Uint8Array();
  const reader = request.body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    total += value.byteLength;
    if (total > max) {
      // 読むのをやめるだけなので、取り消しの失敗は無視する（413 を 500 にしない）
      await reader.cancel().catch(() => {});
      return null;
    }
    chunks.push(value);
  }
  const bytes = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return bytes;
}

// limitBytes は、流れたバイト数が max を超えたら失敗するストリーム（ボディを溜めずに上限をかける）
export function limitBytes(max: number): TransformStream<Uint8Array, Uint8Array> {
  let total = 0;
  return new TransformStream({
    transform(chunk, controller) {
      total += chunk.byteLength;
      if (total > max) controller.error(new Error(`body exceeds ${max} bytes`));
      else controller.enqueue(chunk);
    },
  });
}

// readCloneUpTo は request.clone() の本文を max バイトまで読む。超えたら null を返す。
// clone は元の本文と tee() でつながっていて、片方の取り消しはもう片方も取り消されるまで終わらないので、取り消しは待たない
// （同じファイルの readBodyUpTo は取り消しを待つため使わない）
export async function readCloneUpTo(request: Request, max: number): Promise<Uint8Array | null> {
  const clone = request.clone();
  if (!clone.body) return new Uint8Array();
  const reader = clone.body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    total += value.byteLength;
    if (total > max) {
      reader.cancel().catch(() => {});
      return null;
    }
    chunks.push(value);
  }
  const bytes = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return bytes;
}
