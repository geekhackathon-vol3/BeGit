// errorResponse は Go の handler.respondError と同じ形（{"error": "..."}）でエラーを返す。
export function errorResponse(status: number, message: string): Response {
  return Response.json({ error: message }, { status });
}
