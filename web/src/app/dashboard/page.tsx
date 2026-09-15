"use client";

/** 概览：聚合"现在有什么事等我做"，每项可点击直达处理页。 */
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { orderApi, productApi, taskApi } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

interface Todo { icon: string; text: string; href: string }

export default function DashboardPage() {
  const router = useRouter();
  const { isAdmin } = useAuth();
  const [todos, setTodos] = useState<Todo[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!isAdmin) { setLoading(false); return; }
    (async () => {
      const found: Todo[] = [];
      // 每项独立 try：某个接口失败不影响其余待办的展示。
      try {
        const orders = await orderApi.adminList("paid");
        if (orders.total > 0) found.push({ icon: "🚚", text: `${orders.total} 笔已支付订单等待发货`, href: "/orders" });
      } catch {}
      try {
        const products = await productApi.adminList(1, 1);
        if (products.total > 0) found.push({ icon: "📦", text: `共 ${products.total} 个商品，可去管理`, href: "/products" });
      } catch {}
      try {
        const tasks = await taskApi.list("failed");
        if (tasks.total > 0) found.push({ icon: "⚠️", text: `${tasks.total} 个后台任务失败待人工重试`, href: "/tasks" });
      } catch {}
      setTodos(found);
      setLoading(false);
    })();
  }, [isAdmin]);

  if (!isAdmin) return <Card><CardContent className="py-16 text-center text-sm text-muted-foreground">仅运营角色可查看</CardContent></Card>;
  if (loading) return <Skeleton className="h-40" />;

  return (
    <Card>
      <CardHeader><CardTitle className="text-base">待处理事项</CardTitle></CardHeader>
      <CardContent className="grid gap-2">
        {todos.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted-foreground">暂无待办，一切正常 ✓</p>
        ) : todos.map((t) => (
          <button
            key={t.href + t.text}
            onClick={() => router.push(t.href)}
            className="flex items-center gap-3 rounded-md border px-4 py-3 text-left text-sm transition-colors hover:border-primary"
          >
            <span>{t.icon}</span>
            <span className="flex-1">{t.text}</span>
            <span className="text-xs font-medium text-primary">去处理 →</span>
          </button>
        ))}
      </CardContent>
    </Card>
  );
}
