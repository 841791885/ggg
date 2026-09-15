"use client";

/**
 * 结算页：核对清单（服务端重算）→ 选地址 → 提交订单。
 *
 * 幂等键的生命周期在这里体现：进入本页生成一次，提交时复用——
 * 网络抖动重试时后端会识别为同一次购买意图，不会重复下单。
 * （详见后端 services/order_service.go 的 Create 注释）
 */
import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { addressApi, cartApi, newIdempotencyKey, orderApi, yuan } from "@/lib/api";
import type { Address, CartPreview } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";

export default function CheckoutPage() {
  const router = useRouter();
  const [preview, setPreview] = useState<CartPreview | null>(null);
  const [addresses, setAddresses] = useState<Address[]>([]);
  const [addressId, setAddressId] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  // 幂等键：本次"购买意图"的生命周期内固定不变（进入页面时生成一次）。
  const idempotencyKey = useRef(newIdempotencyKey());

  useEffect(() => {
    (async () => {
      try {
        const [p, a] = await Promise.all([cartApi.preview(), addressApi.list()]);
        setPreview(p);
        const list = a.items ?? [];
        setAddresses(list);
        const preferred = list.find((x) => x.is_default) ?? list[0];
        if (preferred) setAddressId(String(preferred.id));
      } catch (err) {
        toast.error(err instanceof Error ? err.message : "加载结算信息失败");
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  const submit = useCallback(async () => {
    if (!addressId) {
      toast.error("请先添加收货地址");
      router.push("/addresses");
      return;
    }
    setSubmitting(true);
    try {
      const order = await orderApi.create(Number(addressId), idempotencyKey.current);
      toast.success(`订单已创建：${order.order_no.slice(-8)}`);
      // 下单后清理已购明细（后端暂无"按选中项清空"接口，逐条删除等价实现）。
      for (const item of preview?.items.filter((i) => i.purchasable) ?? []) {
        cartApi.remove(item.item_id).catch(() => {});
      }
      router.push("/orders");
    } catch (err) {
      // 失败时保留页面与幂等键：用户重试仍复用同一 key，不会重复下单。
      toast.error(err instanceof Error ? err.message : "下单失败");
    } finally {
      setSubmitting(false);
    }
  }, [addressId, preview, router]);

  if (loading) return <Skeleton className="h-64" />;

  const purchasable = preview?.items.filter((i) => i.purchasable) ?? [];

  if (purchasable.length === 0) {
    return (
      <Card>
        <CardContent className="py-16 text-center text-sm text-muted-foreground">
          没有可购买的商品
          <br />
          <span className="text-xs">{preview?.items[0]?.reason ?? "请先在购物车勾选商品"}</span>
          <br />
          <Button variant="link" onClick={() => router.push("/cart")}>
            回购物车
          </Button>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">核对清单</CardTitle>
        <p className="text-xs text-muted-foreground">金额由服务端重算，不可购项不会进入订单</p>
      </CardHeader>
      <CardContent className="flex flex-col gap-1">
        {preview?.items.map((item) => (
          <div
            key={item.item_id}
            className={`flex items-center justify-between border-b py-3 text-sm last:border-0 ${item.purchasable ? "" : "opacity-50"}`}
          >
            <div>
              <span className="font-medium">{item.product_name}</span>
              <span className="ml-2 text-xs text-muted-foreground">
                {item.sku_code} ×{item.quantity}
              </span>
            </div>
            <span>{item.purchasable ? yuan(item.subtotal_cent) : <span className="text-destructive"> {item.reason}</span>}</span>
          </div>
        ))}

        <div className="flex items-baseline justify-end gap-2 py-4">
          <span className="text-sm">应付合计</span>
          <span className="text-2xl font-bold">{yuan(preview?.total_cent ?? 0)}</span>
          <span className="text-xs text-muted-foreground">（{purchasable.length} 件可购）</span>
        </div>

        <div className="flex items-center gap-3 border-t pt-4">
          <Label className="shrink-0">收货地址</Label>
          {addresses.length > 0 ? (
            <Select value={addressId} onValueChange={(v) => setAddressId(v ?? "")}>
              <SelectTrigger className="flex-1">
                {/* 显式渲染选中项文本：默认的 <SelectValue /> 在某些版本下只显示 value（数字 id），
                    这里手动查出对应地址，保证用户看到的是"收件人 + 电话 + 详细地址"。 */}
                <SelectValue placeholder="选择地址">
                  {addresses.find((x) => String(x.id) === addressId)
                    ? (() => { const sel = addresses.find((x) => String(x.id) === addressId)!;
                        return `${sel.recipient} ${sel.phone} · ${sel.province}${sel.city}${sel.district}${sel.detail}`; })()
                    : "选择地址"}
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                {addresses.map((a) => (
                  <SelectItem key={a.id} value={String(a.id)}>
                    {a.recipient} {a.phone} · {a.province}{a.city}{a.district}{a.detail}
                    {a.is_default ? "【默认】" : ""}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          ) : (
            <span className="flex-1 text-sm text-destructive">暂无收货地址</span>
          )}
          <Button variant="link" onClick={() => router.push("/addresses")}>
            管理地址
          </Button>
        </div>

        <Button className="mt-4 w-full" size="lg" disabled={submitting || !addressId} onClick={submit}>
          {submitting ? "提交中…" : "提交订单"}
        </Button>
        <p className="mt-2 text-center text-xs text-muted-foreground">
          提交后订单保留 30 分钟支付窗口，超时系统会自动关单并归还库存
        </p>
      </CardContent>
    </Card>
  );
}