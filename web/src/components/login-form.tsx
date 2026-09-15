"use client";

/**
 * 登录页：身份用下拉框选择（学习项目里账号是固定的几个）。
 *
 * 表单用受控组件 + useState，提交后调 authContext.login。
 * 与上一版的差别：不需要手动 DOM 操作，loading/错误状态由 React 管理。
 */
import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useAuth } from "@/lib/auth-context";
import type { UserRole } from "@/lib/types";

/** 学习项目预设账号；密码统一 12345678（见后端 migrations 的种子数据）。 */
const ACCOUNTS: { value: string; label: string; role: UserRole }[] = [
  { value: "admin", label: "运营管理员 admin", role: "admin" },
  { value: "buyer_test_01", label: "普通买家 buyer_test_01", role: "customer" },
  { value: "racer_a", label: "买家A · 抢购测试 racer_a", role: "customer" },
  { value: "racer_b", label: "买家B · 抢购测试 racer_b", role: "customer" },
];

export function LoginForm() {
  const { login } = useAuth();
  const [account, setAccount] = useState("admin");
  const [password, setPassword] = useState("12345678");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError("");
    try {
      const target = ACCOUNTS.find((a) => a.value === account);
      await login(account, password, target?.role ?? "customer");
      toast.success(`欢迎回来，${account}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "登录失败");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-muted/30 p-6">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <div className="mb-2 flex size-10 items-center justify-center rounded-lg bg-primary font-bold text-primary-foreground">
            G
          </div>
          <CardTitle>GoMall 商城</CardTitle>
          <CardDescription>选择身份登录，体验买家与运营两种视角</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="grid gap-4">
            <div className="grid gap-2">
              <Label htmlFor="account">登录身份</Label>
              <Select value={account} onValueChange={(v: string | null) => setAccount(v ?? "admin")}>
                <SelectTrigger id="account">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {ACCOUNTS.map((a) => (
                    <SelectItem key={a.value} value={a.value}>
                      {a.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-2">
              <Label htmlFor="password">密码</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
                required
              />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
            <Button type="submit" disabled={loading}>
              {loading ? "登录中…" : "登录"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}