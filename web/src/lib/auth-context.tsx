"use client";

/**
 * 登录态上下文。
 *
 * 为什么用 Context 而不是像上一版那样直接读写 localStorage：
 * 每个页面都要知道"我是谁、是不是运营"，散落各处会在切换账号时不同步。
 * Context 让登录/登出成为单一入口，所有消费组件自动重渲染。
 *
 * 注意这里是"客户端状态"（token 存在浏览器），不是服务端会话——
 * 后端用 JWT 无状态鉴权，本 Context 只是浏览器侧的镜像。
 */
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { authApi, getAuth, setAuth } from "./api";
import type { AuthState, UserRole } from "./types";

interface AuthContextValue {
  auth: AuthState | null;
  isAdmin: boolean;
  /** 是否已完成首次 localStorage 读取（避免 SSR/CSR 首帧闪烁）。 */
  ready: boolean;
  login: (login: string, password: string, role: UserRole) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [auth, setAuthState] = useState<AuthState | null>(null);
  const [ready, setReady] = useState(false);

  // 首帧后再读 localStorage：SSR 阶段没有 window，直接读会报错。
  useEffect(() => {
    setAuthState(getAuth());
    setReady(true);
  }, []);

  const login = useCallback(async (loginName: string, password: string, role: UserRole) => {
    const data = await authApi.login(loginName, password);
    const next: AuthState = { token: data.access_token, login: loginName, role };
    setAuth(next);
    setAuthState(next);
  }, []);

  const logout = useCallback(() => {
    setAuth(null);
    setAuthState(null);
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({ auth, isAdmin: auth?.role === "admin", ready, login, logout }),
    [auth, ready, login, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth 必须在 <AuthProvider> 内使用");
  return ctx;
}