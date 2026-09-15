import type { NextConfig } from "next";

// Next 与 Go 后端是两条独立进程：Next(:3000) 负责页面，Go(:8080) 负责 API。
// 用 rewrite 把 /api/* 转发给后端，好处是【浏览器视角同源】：
// 不用配 CORS、不产生预检请求，Cookie 与 Authorization 头都能正常往返。
// 部署时这层转发由 Nginx 接管（见 PRD-011），开发期先交给 Next 自己。
const nextConfig: NextConfig = {
  reactCompiler: true,
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${process.env.API_BASE_URL ?? "http://127.0.0.1:8080"}/api/:path*`,
      },
    ];
  },
};

export default nextConfig;