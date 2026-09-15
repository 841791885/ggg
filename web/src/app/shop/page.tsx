"use client";

/**
 * 逛商城：在售商品卡片流，展开规格直接加购。
 *
 * 数据来源 GET /api/v1/products（后端强制 status=on_sale），
 * 规格来自 GET /api/v1/products/:id/skus（后端强制 status=active）。
 */
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { cartApi, productApi, yuan } from "@/lib/api";
import type { Product, SKU } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";

export default function ShopPage() {
  const [products, setProducts] = useState<Product[]>([]);
  const [skusByProduct, setSkusByProduct] = useState<Record<number, SKU[]>>({});
  const [loading, setLoading] = useState(true);
  const [addingId, setAddingId] = useState<number | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const { list } = await productApi.listOnSale();
        if (cancelled) return;
        setProducts(list);
        // 并发拉每个商品的规格；某个失败不影响整体展示。
        const entries = await Promise.all(
          list.map(async (p) => {
            try {
              const res = await productApi.listPublicSKUs(p.id);
              return [p.id, res.list] as const;
            } catch {
              return [p.id, [] as SKU[]] as const;
            }
          }),
        );
        if (!cancelled) setSkusByProduct(Object.fromEntries(entries));
      } catch (err) {
        toast.error(err instanceof Error ? err.message : "加载商品失败");
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  async function handleAdd(sku: SKU) {
    setAddingId(sku.id);
    try {
      await cartApi.add(sku.id, 1);
      toast.success(`已加入购物车：${Object.values(sku.specs).join("·") || sku.code}`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "加购失败");
    } finally {
      setAddingId(null);
    }
  }

  if (loading) {
    return (
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {[1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-48" />
        ))}
      </div>
    );
  }

  if (products.length === 0) {
    return (
      <Card>
        <CardContent className="py-16 text-center text-sm text-muted-foreground">
          暂无可售商品
          <br />
          <span className="text-xs">运营角色可在「商品管理」创建商品并上架</span>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {products.map((product) => {
        const skus = skusByProduct[product.id] ?? [];
        return (
          <Card key={product.id} className="flex flex-col">
            <CardHeader>
              <CardTitle className="text-base">{product.name}</CardTitle>
              {product.description && <CardDescription>{product.description}</CardDescription>}
            </CardHeader>
            <CardContent className="flex flex-1 flex-col gap-2">
              {skus.length === 0 ? (
                <p className="text-xs text-muted-foreground">无可售规格</p>
              ) : (
                skus.map((sku) => (
                  <div key={sku.id} className="flex items-center justify-between gap-3 border-t pt-2 first:border-0 first:pt-0">
                    <div className="min-w-0">
                      <p className="text-sm font-semibold">{yuan(sku.price_cent)}</p>
                      <p className="truncate text-xs text-muted-foreground">
                        {Object.values(sku.specs).join("·") || sku.code}
                        {" · "}
                        {sku.stock > 0 ? `库存${sku.stock}` : "已售罄"}
                      </p>
                    </div>
                    <Button
                      size="sm"
                      disabled={sku.stock <= 0 || addingId === sku.id}
                      onClick={() => handleAdd(sku)}
                    >
                      {sku.stock <= 0 ? "无货" : addingId === sku.id ? "加入中…" : "加入购物车"}
                    </Button>
                  </div>
                ))
              )}
            </CardContent>
          </Card>
        );
      })}
    </div>
  );
}