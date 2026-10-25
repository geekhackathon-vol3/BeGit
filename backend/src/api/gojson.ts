// Go の encoding/json（gin の ShouldBindJSON）で構造体に読むときと同じ結果になるかを、JSON.parse の結果から判定する。
// 判定できない入力（Go と読み方がずれうるもの）は null を返し、呼び出し元はコンテナへ転送して Go に任せる。

export type GoFieldType = "string" | "*string" | "*int64";

// decodeGoObject は、text を Go で fields の構造体に読んだときの値を返す。
//   null を返す : Go がエラーにする入力、または Go と同じに読めると言い切れない入力
//   値          : fields の名前（小文字）→ 値。キーがない、または null で読まれないものは undefined
// 言い切れないものは次のとおり。
//   - JSON.parse が読めない（Go は後ろに続くゴミを無視するなど、読み方が違う）
//   - キーが重複している（JSON.parse は前の値を捨てるが、Go は型の誤りを覚えている）
//   - キーに ASCII 以外の文字がある（Go は Unicode の大文字小文字も同じとみなす）
//   - 整数の項目が、JavaScript の数値で正確に表せない
export function decodeGoObject(text: string, fields: Record<string, GoFieldType>): Record<string, unknown> | null {
  // 数値の元の表記を、それを持つオブジェクトごとに覚える（整数の項目は Go と同じく表記で判定する。1.0 や 1e3 は Go ではエラー）
  const numberSources = new Map<object, Map<string, string>>();
  let parsed: unknown;
  try {
    parsed = JSON.parse(text, function (this: object, key: string, value: unknown, context?: { source?: string }) {
      if (typeof value === "number") {
        let sources = numberSources.get(this);
        if (!sources) numberSources.set(this, (sources = new Map()));
        sources.set(key, context?.source ?? "");
      }
      return value;
    });
  } catch {
    return null;
  }
  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) return null;
  if (hasDuplicateKeys(text)) return null;

  const out: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(parsed)) {
    if (!/^[\x20-\x7e]*$/.test(key)) return null;
    const name = key.toLowerCase();
    const type = fields[name];
    if (!type) continue;
    if (value === null) {
      // Go は null をポインターなら nil にし、そうでなければ読み飛ばす
      if (type.startsWith("*")) out[name] = null;
      continue;
    }
    if (type === "*int64") {
      const source = numberSources.get(parsed)?.get(key);
      if (typeof value !== "number" || source === undefined || !/^-?\d+$/.test(source)) return null;
      if (!Number.isSafeInteger(value)) return null;
      out[name] = value;
    } else {
      if (typeof value !== "string") return null;
      out[name] = value;
    }
  }
  return out;
}

// hasDuplicateKeys は、入れ子も含めて、小文字にすると同じになるキーが2回以上あるかを返す。
// 入れ子のキーも数えるので、本当は重複していない入力でも true になりうる（そのときは転送するだけなので安全側）
function hasDuplicateKeys(text: string): boolean {
  const seen = new Set<string>();
  // 文字列の外には " が現れないので、文字列を前から順に拾い、直後に : が続くものをキーとみなす
  const token = /"(?:[^"\\]|\\.)*"(\s*:)?/g;
  for (const match of text.matchAll(token)) {
    if (!match[1]) continue;
    const key = (JSON.parse(match[0].slice(0, match[0].length - match[1].length)) as string).toLowerCase();
    if (seen.has(key)) return true;
    seen.add(key);
  }
  return false;
}
