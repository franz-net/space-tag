import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  async rewrites() {
    return [
      {
        source: "/api/ws",
        destination: "http://localhost:8080/ws",
      },
    ];
  },
};

export default nextConfig;
