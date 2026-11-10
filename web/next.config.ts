import type { NextConfig } from "next";

/**
 * Content-Security-Policy. Shipped as Report-Only for now: enforcing it would
 * risk breaking the Firebase/Google sign-in flow (popups, iframes, token
 * endpoints) if an origin is missed, so we gather violations first, then
 * promote to an enforcing `Content-Security-Policy` once the login flow is
 * confirmed clean. `connect-src` lists the backend API + WebSocket and the
 * Firebase/Google auth endpoints; `frame-src` the sign-in frames.
 */
const csp = [
  "default-src 'self'",
  // Next.js injects inline bootstrap scripts + we run a small inline theme
  // script in layout.tsx; keep 'unsafe-inline'/'unsafe-eval' until a nonce
  // setup replaces them.
  "script-src 'self' 'unsafe-inline' 'unsafe-eval'",
  "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
  "font-src 'self' https://fonts.gstatic.com data:",
  "img-src 'self' data: blob: https:",
  "connect-src 'self' https://api.kairso.com wss://api.kairso.com https://*.googleapis.com https://*.firebaseio.com https://identitytoolkit.googleapis.com https://securetoken.googleapis.com",
  "frame-src 'self' https://*.firebaseapp.com https://accounts.google.com https://*.google.com",
  "worker-src 'self' blob:",
  "object-src 'none'",
  "base-uri 'self'",
  "form-action 'self'",
  "frame-ancestors 'self'",
].join("; ");

const securityHeaders = [
  // Pin HTTPS for this host for a year. No includeSubDomains/preload yet —
  // avoids forcing HTTPS onto sibling subdomains we don't control here.
  { key: "Strict-Transport-Security", value: "max-age=31536000" },
  // Clickjacking: the app never needs to be framed by another origin.
  { key: "X-Frame-Options", value: "SAMEORIGIN" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  // Don't leak full URLs (task ids live in query strings) to other origins.
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  // geolocation is used to set the company coordinates in attendance settings.
  { key: "Permissions-Policy", value: "camera=(), microphone=(), payment=(), usb=(), geolocation=(self)" },
  { key: "Content-Security-Policy-Report-Only", value: csp },
];

const nextConfig: NextConfig = {
  output: "standalone",
  // Don't advertise the framework/version.
  poweredByHeader: false,
  devIndicators: {
    position: "bottom-right",
  },
  async headers() {
    return [{ source: "/:path*", headers: securityHeaders }];
  },
};

export default nextConfig;
