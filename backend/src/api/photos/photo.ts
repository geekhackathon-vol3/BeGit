import type { PhotoStorage } from "../r2";

// photos テーブルの行と、Go の handler.PhotoJSON と同じ形の応答。

export interface PhotoRow {
  id: number;
  post_id: number;
  r2_key: string;
  photo_type: string;
}

export interface PhotoJSON {
  id: number;
  photo_type: string;
  url: string;
}

// toPhotoJSONs は写真に署名付き URL を付ける（Go の buildFeedPhotos・アップロードの応答と同じ）
export function toPhotoJSONs(storage: PhotoStorage, rows: PhotoRow[]): Promise<PhotoJSON[]> {
  return Promise.all(rows.map(async (row) => ({ id: row.id, photo_type: row.photo_type, url: await storage.presignGetURL(row.r2_key) })));
}
