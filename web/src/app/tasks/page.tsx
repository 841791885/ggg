"use client";

/** 后台任务：worker 队列视图（超时关单 / 站内信投递），失败可人工重试。 */
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { taskApi } from "@/lib/api";
import type { BackgroundTask } from "@/lib/types";
import { useAuth } from "@/lib/auth-context";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { TaskStatusBadge } from "@/components/status-badge";

export default function TasksPage() {
  const { isAdmin } = useAuth();
  const [list, setList] = useState<BackgroundTask[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    if (!isAdmin) { setLoading(false); return; }
    try {
      setList((await taskApi.list()).list ?? []);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "加载任务失败");
    } finally {
      setLoading(false);
    }
  }, [isAdmin]);
  useEffect(() => { void load(); }, [load]);

  if (!isAdmin) return <Card><CardContent className="py-16 text-center text-sm text-muted-foreground">仅运营角色可查看</CardContent></Card>;
  if (loading) return <Skeleton className="h-40" />;

  return (
    <div className="flex flex-col gap-3">
      <p className="text-xs text-muted-foreground">
        worker 每 5 秒扫描一次到期任务，失败按指数退避自动重试；达到上限后定格待人工处理
      </p>
      {list.length === 0 ? (
        <Card><CardContent className="py-16 text-center text-sm text-muted-foreground">队列为空</CardContent></Card>
      ) : (
        list.map((t) => (
          <Card key={t.id}>
            <CardContent className="flex items-center justify-between gap-3 py-3">
              <div className="min-w-0">
                <p className="text-sm font-medium">
                  {t.task_type} <span className="text-xs text-muted-foreground">#{t.id}</span>
                </p>
                <p className="text-xs text-muted-foreground">
                  尝试 {t.attempts}/{t.max_attempts} · 下次 {new Date(t.next_run_at).toLocaleString("zh-CN")}
                </p>
                {t.last_error && <p className="truncate text-xs text-destructive">{t.last_error}</p>}
              </div>
              <div className="flex shrink-0 items-center gap-2">
                <TaskStatusBadge status={t.status} />
                {t.status === "failed" && (
                  <Button size="sm" variant="outline" onClick={async () => { await taskApi.retry(t.id); toast.success("已重新入队"); await load(); }}>重试</Button>
                )}
              </div>
            </CardContent>
          </Card>
        ))
      )}
    </div>
  );
}
