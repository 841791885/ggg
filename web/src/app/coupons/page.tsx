"use client";

/** 优惠券：可领取的模板 + 我的券包（含 A4 的"每人限N张"与 seq 展示）。 */
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { couponApi, yuan } from "@/lib/api";
import type { CouponTemplate, UserCoupon } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";

export default function CouponsPage() {
  const [templates, setTemplates] = useState<CouponTemplate[]>([]);
  const [mine, setMine] = useState<UserCoupon[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      const [t, m] = await Promise.all([couponApi.listTemplates(), couponApi.listMine()]);
      setTemplates(t.list ?? []);
      setMine(m.items ?? []);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "加载优惠券失败");
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => { void load(); }, [load]);

  async function claim(t: CouponTemplate) {
    try {
      await couponApi.claim(t.id);
      toast.success(`已领取：${t.name}`);
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "领取失败");
    }
  }

  if (loading) return <Skeleton className="h-56" />;

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader><CardTitle className="text-base">可领取的券</CardTitle></CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>名称</TableHead><TableHead>门槛/面额</TableHead><TableHead>剩余</TableHead><TableHead>有效期</TableHead><TableHead className="text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {templates.map((t) => {
                const held = mine.filter((c) => c.template_id === t.id).length;
                const full = held >= t.per_user_limit;
                const soldOut = t.remaining <= 0;
                return (
                  <TableRow key={t.id}>
                    <TableCell>
                      <span className="font-medium">{t.name}</span>
                      {t.per_user_limit > 1 && (
                        <span className="ml-2 text-xs text-muted-foreground">每人限{t.per_user_limit}张 · 已领{held}</span>
                      )}
                    </TableCell>
                    <TableCell>满{yuan(t.threshold_cent)}减{yuan(t.discount_cent)}</TableCell>
                    <TableCell>{t.remaining}/{t.total_count}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {new Date(t.starts_at).toLocaleDateString("zh-CN")} ~ {new Date(t.ends_at).toLocaleDateString("zh-CN")}
                    </TableCell>
                    <TableCell className="text-right">
                      {full ? <Badge variant="secondary">已领满</Badge>
                        : soldOut ? <Badge variant="secondary">已抢光</Badge>
                        : <Button size="sm" onClick={() => claim(t)}>领取</Button>}
                    </TableCell>
                  </TableRow>
                );
              })}
              {templates.length === 0 && (
                <TableRow><TableCell colSpan={5} className="py-10 text-center text-sm text-muted-foreground">暂无可领的券</TableCell></TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle className="text-base">我的券包</CardTitle></CardHeader>
        <CardContent className="grid gap-2">
          {mine.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">还没有领取优惠券</p>
          ) : (
            mine.map((c) => (
              <div key={c.id} className="flex items-center justify-between rounded-md border px-3 py-2">
                <div>
                  <p className="text-sm font-medium">{c.template_name || `券 #${c.template_id}`}</p>
                  <p className="text-xs text-muted-foreground">
                    满{yuan(c.threshold_cent)}减{yuan(c.discount_cent)}
                    {c.seq > 1 && ` · 第${c.seq}张`}
                  </p>
                </div>
                <Badge variant={c.status === "unused" ? "default" : "secondary"}>
                  {{ unused: "未使用", used: "已使用", expired: "已过期" }[c.status]}
                </Badge>
              </div>
            ))
          )}
        </CardContent>
      </Card>
    </div>
  );
}
