import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Build a fully static client (no Node runtime needed in production).
  // The Go server serves the resulting `out/` directory directly.
  output: "export",

  // Static export doesn't optimize images at request time.
  images: {
    unoptimized: true,
  },

  // Trailing slashes make every page resolve as a directory with index.html,
  // which plays nicely with simple static file servers.
  trailingSlash: true,
};

export default nextConfig;
