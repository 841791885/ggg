"use client";

/**
 * 全站骨架：未登录显示登录页，已登录显示侧栏 + 内容区。
 *
 * 这一层替代了上一版手工的 showApp/logout 切 DOM——用条件渲染表达，
 * 不存在"忘了加 hidden 类导致两个视图同时出现"这类问题。
 */
import { useAuth } from "@/lib/auth-context";
import { Sidebar } from "./sidebar";
import { LoginForm } from "./login-form";

export function AppShell({ children }: { children: React.ReactNode }) {
  const { auth, ready } = useAuth();

  // 首帧（还没读 localStorage）不渲染任何一态，避免登录页一闪而过。
  if (!ready) {
    return <div className="flex min-h-screen items-center justify-center text-sm text-muted-foreground">加载中…</div>;
  }

  if (!auth) return <LoginForm />;

  return (
    <div className="flex min-h-screen">
      <Sidebar />
      <main className="min-w-0 flex-1 px-8 py-6">{children}</main>
    </div>
  );
}