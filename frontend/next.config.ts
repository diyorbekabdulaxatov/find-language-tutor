import type { NextConfig } from "next";
import createNextIntlPlugin from "next-intl/plugin";

// Browser-hardening headers for every page. No CSP yet: Next's inline
// bootstrap scripts need a nonce-based policy, which is its own piece of work.
// HSTS is only meaningful (and only safe) over HTTPS, so it is production-only.
const securityHeaders = [
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
  ...(process.env.NODE_ENV === "production"
    ? [{ key: "Strict-Transport-Security", value: "max-age=63072000; includeSubDomains" }]
    : []),
];

// Uploaded teacher photos are served by the API's public media route, so the
// image optimizer must be allowed to fetch from that origin.
const apiUrl = new URL(process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080");

const isLocalApi = ["localhost", "127.0.0.1", "::1"].includes(apiUrl.hostname);

const nextConfig: NextConfig = {
  images: {
    // Next 16 refuses to optimize images from private IPs (SSRF guard). Local
    // dev serves them from localhost:8080, so allow it only in that case —
    // never on a real deployment.
    dangerouslyAllowLocalIP: isLocalApi,
    remotePatterns: [
      {
        protocol: apiUrl.protocol.replace(":", "") as "http" | "https",
        hostname: apiUrl.hostname,
        port: apiUrl.port,
        pathname: "/v1/teachers/**",
      },
      // Placeholder image hosts used by the seed's demo teachers.
      { protocol: "https", hostname: "i.pravatar.cc" },
      { protocol: "https", hostname: "picsum.photos" },
    ],
  },
  async headers() {
    return [{ source: "/(.*)", headers: securityHeaders }];
  },
};

const withNextIntl = createNextIntlPlugin("./src/i18n/request.ts");

export default withNextIntl(nextConfig);
