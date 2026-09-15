"use client";

/**
 * 购物车：勾选、改数量、删除，底部合计去结算。
 *
 * 商品名/单价/库存都是后端聚合好的（CartItem 的那些展示字段），
 * 上一版这里全是 undefined，根因是接口只返回 sku_id——现在契约在 types.ts 里写死了。
 */
import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { cartApi, yuan } from "@/lib/api";
import type { CartItem } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";

export default function CartPage() {
  const router = useRouter();
  const [items, setItems] = useState<CartItem[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      const cart = await cartApi.get();
      setItems(cart.items ?? []);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "加载购物车失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function changeQuantity(item: CartItem, next: number) {
    if (next < 1) return;
    try {
      await cartApi.updateQuantity(item.id, next);
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "修改数量失败");
    }
  }

  async function toggleSelected(item: CartItem, checked: boolean) {
    // 乐观更新：先改 UI 再发请求，失败时重新拉取纠正。
    setItems((prev) => prev.map((i) => (i.id === item.id ? { ...i, selected: checked } : i)));
    try {
      await cartApi.setSelected(item.id, checked);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "更新选中状态失败");
      await load();
    }
  }

  async function remove(item: CartItem) {
    try {
      await cartApi.remove(item.id);
      toast.success("已移除");
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "删除失败");
    }
  }

  const chosen = items.filter((i) => i.selected);
  const totalCent = chosen.reduce((sum, i) => sum + i.price_cent * i.quantity, 0);

  if (loading) return <div className="grid gap-3">{[1, 2].map((i) => <Skeleton key={i} className="h-20" />)}</div>;

  if (items.length === 0) {
    return (
      <Card>
        <CardContent className="py-16 text-center text-sm text-muted-foreground">
          购物车是空的
          <br />
          <Button variant="link" onClick={() => router.push("/shop")}>
            去逛商城 →
          </Button>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="grid gap-3">
        {items.map((item) => (
          <Card key={item.id}>
            <CardContent className="flex items-center gap-4 py-4">
              <Checkbox checked={item.selected} onCheckedChange={(v) => toggleSelected(item, v === true)} />
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium">{item.product_name || `SKU #${item.sku_id}`}</p>
                <p className="text-xs text-muted-foreground">
                  {item.sku_code} · {yuan(item.price_cent)}
                </p>
                {!item.purchasable && item.reason && (
                  <Badge variant="destructive" className="mt-1 border-0">
                    {item.reason}
                  </Badge>
                )}
              </div>
              <div className="flex items-center">
                <Button variant="outline" size="icon" className="size-8 rounded-r-none" onClick={() => changeQuantity(item, item.quantity - 1)}>
                  −
                </Button>
                <div className="flex h-8 w-14 items-center justify-center border-y text-sm">{item.quantity}</div>
                <Button variant="outline" size="icon" className="size-8 rounded-l-none" onClick={() => changeQuantity(item, item.quantity + 1)}>
                  ＋
                </Button>
              </div>
              <p className="w-20 text-right text-sm font-semibold">{yuan(item.price_cent * item.quantity)}</p>
              <Button variant="ghost" size="sm" className="text-destructive" onClick={() => remove(item)}>
                删除
              </Button>
            </CardContent>
          </Card>
        ))}
      </div>

      <Card className="sticky bottom-4">
        <CardContent className="flex items-center justify-between py-4">
          <span className="text-sm">
            已选 <strong>{chosen.length}</strong> 件 · 合计 <strong className="text-lg">{yuan(totalCent)}</strong>
          </span>
          <Button disabled={chosen.length === 0} onClick={() => router.push("/checkout")}>
            去结算 →
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}