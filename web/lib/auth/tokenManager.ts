/**
 * Token Manager for Kairso Web
 * Handles access token & refresh token storage, JWT expiration checking,
 * cookie synchronization, and automatic token renewal.
 */

const ACCESS_TOKEN_KEY = "kairso_access_token";
const REFRESH_TOKEN_KEY = "kairso_refresh_token";

export function getAccessToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(ACCESS_TOKEN_KEY);
}

function getRefreshToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(REFRESH_TOKEN_KEY);
}

export function setTokens(accessToken: string, refreshToken?: string): void {
  if (typeof window === "undefined") return;

  // `Secure` on HTTPS (prod) so the cookie never rides a plain-HTTP request;
  // skipped on http://localhost so local dev still works.
  const secure = typeof location !== "undefined" && location.protocol === "https:" ? "; Secure" : "";

  localStorage.setItem(ACCESS_TOKEN_KEY, accessToken);
  // Only the access-token cookie exists, and only so the Next middleware
  // route-guard can tell "is there a session". The browser's API calls read
  // the token from localStorage (see tokenManager/http.ts), not this cookie.
  document.cookie = `${ACCESS_TOKEN_KEY}=${encodeURIComponent(accessToken)}; path=/; max-age=604800; SameSite=Lax${secure}`;

  if (refreshToken) {
    // Refresh token stays in localStorage only — nothing reads it from a
    // cookie, so we don't expose the long-lived 30-day token there too.
    localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken);
  }
  // Có phiên mới còn sống — cho phép lần hết phiên KẾ TIẾP (dù đăng nhập lại qua email,
  // Google hay refresh) được báo lại, không bị cờ chống-lặp của lần trước chặn mất.
  sessionExpiredNotified = false;
}

export function clearTokens(): void {
  if (typeof window === "undefined") return;

  localStorage.removeItem(ACCESS_TOKEN_KEY);
  localStorage.removeItem(REFRESH_TOKEN_KEY);

  document.cookie = `${ACCESS_TOKEN_KEY}=; path=/; max-age=0; SameSite=Lax`;
  document.cookie = `${REFRESH_TOKEN_KEY}=; path=/; max-age=0; SameSite=Lax`;
}

/**
 * Checks whether a JWT is expired or expiring within `offsetSeconds`
 */
export function isTokenExpired(token: string, offsetSeconds: number = 30): boolean {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return true;
    const payload = JSON.parse(atob(parts[1]));
    if (!payload.exp) return false;
    const expiryMs = payload.exp * 1000;
    return Date.now() + offsetSeconds * 1000 >= expiryMs;
  } catch {
    return true;
  }
}

/**
 * Phát khi phiên đăng nhập chắc chắn đã hết (không còn refresh token, hoặc backend
 * từ chối refresh token bằng 401) — phân biệt với lỗi mạng/5xx tạm thời, để mất mạng
 * chập chờn không đá người dùng ra ngoài. `AuthContext` lắng nghe sự kiện này để dọn
 * state và đưa người dùng về `/login` bằng MỘT thông báo rõ ràng, thay vì để mỗi
 * request đang chạy tự báo lỗi "thiếu header" của riêng nó.
 */
const SESSION_EXPIRED_EVENT = "kairso:session-expired";
let sessionExpiredNotified = false;

function notifySessionExpired(): void {
  if (typeof window === "undefined" || sessionExpiredNotified) return;
  sessionExpiredNotified = true;
  window.dispatchEvent(new Event(SESSION_EXPIRED_EVENT));
}

export function onSessionExpired(cb: () => void): () => void {
  if (typeof window === "undefined") return () => {};
  window.addEventListener(SESSION_EXPIRED_EVENT, cb);
  return () => window.removeEventListener(SESSION_EXPIRED_EVENT, cb);
}

/**
 * Renews the access token using the refresh token via backend Go /v1/auth/refresh.
 * Trả `null` khi refresh không thành công — nhưng chỉ xoá token và báo "hết phiên"
 * khi biết chắc refresh token không còn dùng được (vắng mặt, hoặc bị từ chối 401);
 * lỗi mạng/timeout/5xx tạm thời giữ nguyên token để lần gọi sau thử lại.
 */
let refreshPromise: Promise<string | null> | null = null;

export async function refreshAccessToken(): Promise<string | null> {
  const refreshToken = getRefreshToken();
  if (!refreshToken) {
    clearTokens();
    notifySessionExpired();
    return null;
  }

  // Prevent multiple parallel refresh requests
  if (refreshPromise) {
    return refreshPromise;
  }

  refreshPromise = (async () => {
    try {
      const response = await fetch("/api/proxy/v1/auth/refresh", {
        method: "POST",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ refreshToken }),
      });

      if (!response.ok) {
        // 401/403: backend đã xác nhận refresh token không còn hợp lệ — hết phiên thật.
        // Còn lại (mất mạng, 5xx, proxy lỗi tạm thời) không kết luận được gì, giữ token.
        if (response.status === 401 || response.status === 403) {
          clearTokens();
          notifySessionExpired();
        }
        return null;
      }

      const data = await response.json();
      if (data.accessToken) {
        setTokens(data.accessToken, data.refreshToken || refreshToken);
        sessionExpiredNotified = false;
        return data.accessToken as string;
      }

      clearTokens();
      notifySessionExpired();
      return null;
    } catch (err) {
      // fetch ném lỗi (mất mạng, offline...) — tạm thời, không phải phiên hết hạn.
      console.error("Token refresh failed:", err);
      return null;
    } finally {
      refreshPromise = null;
    }
  })();

  return refreshPromise;
}
