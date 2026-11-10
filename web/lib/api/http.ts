import { getAccessToken, isTokenExpired, refreshAccessToken } from "../auth/tokenManager";

/** Lỗi từ backend: `{ "error": "..." }` kèm HTTP status. */
export class ApiError extends Error {
  status: number;
  data: unknown;

  constructor(message: string, status: number, data?: unknown) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.data = data;
  }
}

export interface RequestOptions {
  signal?: AbortSignal;
  headers?: Record<string, string>;
}

type Query = object;

interface InternalOptions extends RequestOptions {
  body?: unknown;
  query?: Query;
  _retry?: boolean;
}

/** Định danh tab này. Backend dùng để không phát lại realtime event về đúng tab vừa gây ra nó. */
export const connId: string = (() => {
  const bytes = new Uint8Array(16);
  if (typeof crypto !== "undefined") crypto.getRandomValues(bytes);
  return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
})();

function buildQuery(query?: Query): string {
  if (!query) return "";
  const sp = new URLSearchParams();
  for (const [k, v] of Object.entries(query)) {
    if (v === undefined || v === null || v === "") continue;
    sp.append(k, Array.isArray(v) ? v.join(",") : String(v));
  }
  const s = sp.toString();
  return s ? `?${s}` : "";
}

export async function request<T>(
  method: string,
  path: string,
  { body, query, headers: custom, signal, _retry }: InternalOptions = {},
): Promise<T> {
  const url = `/api/proxy/${path.replace(/^\//, "")}${buildQuery(query)}`;
  const isAuthCall = path.includes("/auth/") || path.startsWith("/public/");

  let token = getAccessToken();
  // Không có token và biết chắc không phải lỗi gõ nhầm (đã đăng nhập trước đó rồi mất) →
  // đây là request đến chậm sau khi phiên đã hết; ném lỗi cục bộ ngay thay vì gửi lên
  // server để nhận "thiếu header Authorization" — tránh dội hàng loạt request thừa mỗi
  // khi nhiều truy vấn cùng phát hiện phiên hết cùng lúc (xem tokenManager.onSessionExpired).
  if (!token && !isAuthCall) {
    throw new ApiError("Phiên đăng nhập đã hết hạn, vui lòng đăng nhập lại", 401);
  }
  if (token && !isAuthCall && isTokenExpired(token)) {
    // Refresh thất bại có thể vì phiên thật sự đã hết (tokenManager đã tự lo — xem
    // onSessionExpired) HOẶC chỉ vì mất mạng tạm thời; ở đây chưa biết là trường hợp
    // nào nên KHÔNG được dùng status 401 (AuthContext coi 401 là "chắc chắn hết phiên"
    // và sẽ đăng xuất oan nếu chỉ là lỗi mạng) — status 0 đánh dấu "không phải lỗi
    // xác thực thật từ server", chỉ để hiện thông báo, không kích hoạt đăng xuất.
    token = await refreshAccessToken();
    if (!token) throw new ApiError("Không thể làm mới phiên đăng nhập, kiểm tra kết nối mạng và thử lại", 0);
  }

  const headers: Record<string, string> = {
    Accept: "application/json",
    "X-Conn-Id": connId,
    ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
    ...custom,
  };
  if (token && !path.startsWith("/public/")) headers.Authorization = `Bearer ${token}`;

  const res = await fetch(url, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
    signal,
  });

  if (res.status === 401 && !_retry && !isAuthCall) {
    if (await refreshAccessToken()) {
      return request<T>(method, path, { body, query, headers: custom, signal, _retry: true });
    }
  }

  if (!res.ok) {
    const raw = await res.text();
    let data: unknown = raw;
    try {
      data = JSON.parse(raw);
    } catch {
      /* body không phải JSON */
    }
    const errField =
      data && typeof data === "object" ? (data as { error?: unknown }).error : undefined;
    const msg =
      (typeof errField === "string" && errField) ||
      (typeof data === "string" && data) ||
      `HTTP ${res.status}`;
    throw new ApiError(msg, res.status, data);
  }

  if (res.status === 204) return undefined as T;
  const text = await res.text();
  return (text ? JSON.parse(text) : undefined) as T;
}

/**
 * Như `request` nhưng trả về Blob — cho các endpoint trả file nhị phân (ví dụ
 * xuất Excel KPI). Dùng chung logic token/refresh; lỗi vẫn là `{error}` JSON.
 */
export async function requestBlob(
  method: string,
  path: string,
  { query, signal }: { query?: Query; signal?: AbortSignal } = {},
): Promise<Blob> {
  const url = `/api/proxy/${path.replace(/^\//, "")}${buildQuery(query)}`;

  let token = getAccessToken();
  if (!token) throw new ApiError("Phiên đăng nhập đã hết hạn, vui lòng đăng nhập lại", 401);
  if (isTokenExpired(token)) {
    token = await refreshAccessToken();
    if (!token) throw new ApiError("Không thể làm mới phiên đăng nhập, kiểm tra kết nối mạng và thử lại", 0);
  }

  const res = await fetch(url, {
    method,
    headers: { "X-Conn-Id": connId, Authorization: `Bearer ${token}` },
    signal,
  });

  if (!res.ok) {
    const raw = await res.text();
    let data: unknown = raw;
    try {
      data = JSON.parse(raw);
    } catch {
      /* body không phải JSON */
    }
    const errField = data && typeof data === "object" ? (data as { error?: unknown }).error : undefined;
    const msg = (typeof errField === "string" && errField) || (typeof data === "string" && data) || `HTTP ${res.status}`;
    throw new ApiError(msg, res.status, data);
  }

  return res.blob();
}
