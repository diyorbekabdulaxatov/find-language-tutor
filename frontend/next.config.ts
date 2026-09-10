import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  images: {
    // Placeholder image hosts used by the mock data. Remove these once profile
    // photos and video posters are served from Cloudflare R2.
    remotePatterns: [
      { protocol: "https", hostname: "i.pravatar.cc" },
      { protocol: "https", hostname: "picsum.photos" },
    ],
  },
};

export default nextConfig;