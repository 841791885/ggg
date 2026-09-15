"use client";

/** 消息通知：支付/发货/售后事件由后端 worker 通过 Outbox 投递到这里。 */
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { notificationApi } from "@/lib/api";
import type { Notification } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

export default function NotificationsPage() {
  const [list, setList] = useState<Notification[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      setList((await notificationApi.list()).list ?? []);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "加载通知失败");
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => { void load(); }, [load]);

  if (loading) return <Skeleton className="h-40" />;

  return (
    <div className="flex flex-col gap-3">
      <div className="flex justify-end">
        <Button variant="outline" onClick={async () => { await notificationApi.markAllRead(); await load(); }}>全部已读</Button>
      </div>
      {list.length === 0 ? (
        <Card><CardContent className="py-16 text-center text-sm text-muted-foreground">
          暂无消息<br /><span className="text-xs">支付成功、发货等事件会由后端 worker 投递到这里（Outbox 模式）</span>
        </CardContent></Card>
      ) : (
        list.map((n) => (
          <Card key={n.id} className={cn(!n.read && "border-l-4 border-l-primary")}>
            <CardContent className="py-3">
              <p className="text-sm font-medium">{n.title}</p>
              <p className="text-xs text-muted-foreground">{new Date(n.created_at).toLocaleString("zh-CN")}</p>
              <p className="mt-1 text-sm">{n.content}</p>
            </CardContent>
          </Card>
        ))
      )}
    </div>
  );
}
