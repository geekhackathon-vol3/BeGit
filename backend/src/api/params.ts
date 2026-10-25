const INT64_MIN = -(2n ** 63n);
const INT64_MAX = 2n ** 63n - 1n;

// ParsedID は Go の strconv.ParseInt(s, 10, 64) と同じ規則で読んだ ID。
//   valid=false        : Go でも 400 になる値（整数でない、int64 に収まらない）
//   value=null         : int64 には収まるが、JavaScript の数値で正確に表せない値。該当する行はないものとして扱う
export type ParsedID = { valid: false } | { valid: true; value: number | null };

export function parseID(raw: string): ParsedID {
  if (!/^[+-]?[0-9]+$/.test(raw)) return { valid: false };
  const big = BigInt(raw);
  if (big < INT64_MIN || big > INT64_MAX) return { valid: false };
  const value = Number(big);
  return { valid: true, value: Number.isSafeInteger(value) ? value : null };
}
