import type { Metadata } from "next";
import { Geist } from "next/font/google";
import { AuthProvider } from "@/lib/auth-context";
import { Toaster } from "@/components/ui/sonner";
import { AppShell } from "@/components/app-shell";
import "./globals.css";

const geist = Geist({ variable: "--font-geist-sans", subsets: ["latin"] });

export const metadata: Metadata = {
  title: "GoMall 商城",
  description: "Go 后端学习项目的前端界面",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN" className={`${geist.variable} h-full antialiased`}>
      <body className="min-h-full">
        {/* AuthProvider 提供全站登录态；AppShell 提供侧栏+顶栏骨架。
            两者都是客户端组件——后端是独立 REST API，SSR 阶段拿不到浏览器里的 JWT。 */}
        <AuthProvider>
          <AppShell>{children}</AppShell>
          <Toaster position="top-center" richColors />
        </AuthProvider>
      </body>
    </html>
  );
}