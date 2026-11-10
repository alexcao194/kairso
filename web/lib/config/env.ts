/**
 * Cấu hình môi trường. Thứ tự ưu tiên cho các giá trị CLIENT (per-customer khi
 * self-host): window.__KAIRSO_ENV__ (nạp lúc chạy từ /env-config.js do container
 * sinh) → NEXT_PUBLIC_* (bake lúc build) → mặc định.
 *
 * Nhờ vậy một image web dựng sẵn chạy được ở mọi host mà không phải build lại:
 * API đi same-origin qua /api/proxy, WS same-origin (xem RealtimeProvider), chỉ
 * Firebase/WS-override cần nạp lúc chạy.
 */

type RuntimeEnv = {
  firebaseApiKey?: string;
  firebaseAuthDomain?: string;
  firebaseProjectId?: string;
  firebaseStorageBucket?: string;
  firebaseMessagingSenderId?: string;
  firebaseAppId?: string;
  wsUrl?: string;
};

export function runtimeEnv(): RuntimeEnv {
  if (typeof window !== "undefined") {
    const e = (window as unknown as { __KAIRSO_ENV__?: RuntimeEnv }).__KAIRSO_ENV__;
    if (e && typeof e === "object") return e;
  }
  return {};
}

export const Configs = {
  baseUrl: process.env.NEXT_PUBLIC_BASE_URL || "https://api.kairso.com/api",
  apiUrl: process.env.NEXT_PUBLIC_API_URL || "https://api.kairso.com/api",
  // Rỗng = nối WebSocket same-origin (/ws trên chính host phục vụ web). Đặt
  // NEXT_PUBLIC_WS_URL hoặc window.__KAIRSO_ENV__.wsUrl để trỏ host khác.
  wsUrl: process.env.NEXT_PUBLIC_WS_URL || "",
  googleClientId:
    process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID ||
    "626351558175-md9sie4jo4duhjqvk3smhi14dohpp5c8.apps.googleusercontent.com",
  supportEmail: process.env.NEXT_PUBLIC_SUPPORT_EMAIL || "support@kairso.com",
  termsOfServiceUrl:
    process.env.NEXT_PUBLIC_TERMS_OF_SERVICE_URL || "https://home.kairso.com/tos",
  privacyPolicyUrl:
    process.env.NEXT_PUBLIC_PRIVACY_POLICY_URL || "https://home.kairso.com/privacy",
  isProduction: process.env.NODE_ENV === "production",
  isDevelopment: process.env.NODE_ENV !== "production",
} as const;

/** Host WS hiệu lực: ưu tiên override (runtime → build), rỗng = same-origin. */
export function resolveWsUrl(accessToken: string, connId: string): string {
  const override = runtimeEnv().wsUrl || Configs.wsUrl;
  const q = `?token=${encodeURIComponent(accessToken)}&connId=${encodeURIComponent(connId)}`;
  if (override) {
    const u = new URL(override);
    const proto = u.protocol === "https:" || u.protocol === "wss:" ? "wss" : "ws";
    return `${proto}://${u.host}/ws${q}`;
  }
  const proto = window.location.protocol === "https:" ? "wss" : "ws";
  return `${proto}://${window.location.host}/ws${q}`;
}
