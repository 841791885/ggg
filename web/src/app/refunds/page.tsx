"use client";

/** 退款售后：我的退款单 +（运营）审核台。 */
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { refundApi, yuan } from "@/lib/api";
import type { Refund } from "@/lib/types";
import { useAuth } from "@/lib/auth-context";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";

const STATUS_LABEL: Record<string, string> = { pending: "待审核", approved: "已同意", rejected: "已驳回", refunded: "已退款" };

export default function RefundsPage() {
  const { isAdmin } = useAuth();
  const [mine, setMine] = useState<Refund[]>([]);
  const [pending, setPending] = useState<Refund[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      const m = await refundApi.listMine();
      setMine(m.items ?? []);
      if (isAdmin) {
        const all = await refundApi.adminList("pending");
        setPending(all.list ?? []);
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "加载退款失败");
    } finally {
      setLoading(false);
    }
  }, [isAdmin]);
  useEffect(() => { void load(); }, [load]);

  if (loading) return <Skeleton className="h-40" />;

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader><CardTitle className="text-base">我的退款单</CardTitle></CardHeader>
        <CardContent className="grid gap-2">
          {mine.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">
              暂无退款申请<br /><span className="text-xs">对「已完成」订单的商品行发起申请</span>
            </p>
          ) : mine.map((r) => (
            <div key={r.id} className="flex items-center justify-between rounded-md border px-3 py-2">
              <div>
                <p className="text-sm font-medium">退款 #{r.id}</p>
                <p className="text-xs text-muted-foreground">
                  订单项 {r.order_item_id} · {yuan(r.amount_cent)} · {new Date(r.created_at).toLocaleString("zh-CN")}
                </p>
              </div>
              <Badge variant={r.status === "pending" ? "secondary" : "default"}>{STATUS_LABEL[r.status] ?? r.status}</Badge>
            </div>
          ))}
        </CardContent>
      </Card>

      {isAdmin && (
        <Card>
          <CardHeader><CardTitle className="text-base">运营审核台</CardTitle></CardHeader>
          <CardContent className="grid gap-2">
            {pending.length === 0 ? (
              <p className="py-8 text-center text-sm text-muted-foreground">没有待处理的退款申请</p>
            ) : pending.map((r) => (
              <div key={r.id} className="flex items-center justify-between rounded-md border px-3 py-2">
                <div>
                  <p className="text-sm font-medium">待审 #{r.id}</p>
                  <p className="text-xs text-muted-foreground">{yuan(r.amount_cent)} · {r.reason}</p>
                </div>
                <div className="flex gap-2">
                  <Button size="sm" onClick={async () => { await refundApi.approve(r.id); toast.success("已同意"); await load(); }}>同意</Button>
                  <Button size="sm" variant="outline" onClick={async () => { await refundApi.reject(r.id); toast.success("已驳回"); await load(); }}>驳回</Button>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
