import type { NextConfig } from "next";

// 低内存构建开关：在 1 GB 内存的受限环境中，配合
// `NODE_OPTIONS=--max-old-space-size=256 next build --webpack --experimental-build-mode=compile`
// 与随后的 `--experimental-build-mode=generate` 两阶段构建使用。
const lowMemoryBuild = process.env.LOW_MEMORY_BUILD === "1";

const nextConfig: NextConfig = {
  poweredByHeader: false,
  reactStrictMode: true,
  experimental: lowMemoryBuild
    ? {
        cpus: 1,
        workerThreads: false,
        memoryBasedWorkersCount: true,
        webpackMemoryOptimizations: true,
        parallelServerCompiles: false,
      }
    : undefined,
};

export default nextConfig;
