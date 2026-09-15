import type { NextConfig } from "next";

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

const nextConfig: NextConfig = {
  images: {
    // Placeholder image hosts used by the mock data. Remove these once profile
    // photos and video posters are served from Cloudflare R2.
    remotePatterns: [
      { protocol: "https", hostname: "i.pravatar.cc" },
      { protocol: "https", hostname: "picsum.photos" },
    ],
  },
  async headers() {
    return [{ source: "/(.*)", headers: securityHeaders }];
  },
};

export default nextConfig;
