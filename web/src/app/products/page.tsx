"use client";

/**
 * 商品管理（运营）：列表 + 上架/下架 + SKU 抽屉。
 * SKU 的规格用动态键值对编辑，对应后端 specs 字段（1~5 项）。
 */
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { productApi, yuan } from "@/lib/api";
import type { Product, SKU } from "@/lib/types";
import { useAuth } from "@/lib/auth-context";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { ProductStatusBadge } from "@/components/status-badge";

export default function ProductsPage() {
  const { isAdmin } = useAuth();
  const [list, setList] = useState<Product[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [skuProduct, setSkuProduct] = useState<Product | null>(null);
  const [skus, setSkus] = useState<SKU[]>([]);
  const [skuCode, setSkuCode] = useState("");
  const [skuPrice, setSkuPrice] = useState("");
  const [skuStock, setSkuStock] = useState("");
  // 规格行：每行一个 {key, value}，提交时合成对象。
  const [specs, setSpecs] = useState<{ key: string; value: string }[]>([{ key: "", value: "" }]);

  const load = useCallback(async () => {
    if (!isAdmin) { setLoading(false); return; }
    try {
      setList((await productApi.adminList(1, 50)).list ?? []);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "加载商品失败");
    } finally {
      setLoading(false);
    }
  }, [isAdmin]);
  useEffect(() => { void load(); }, [load]);

  const loadSkus = useCallback(async (p: Product) => {
    try {
      setSkus((await productApi.listSKUs(p.id)).list ?? []);
    } catch { setSkus([]); }
  }, []);

  async function createProduct() {
    if (!name.trim()) { toast.error("请填写商品名称"); return; }
    try {
      await productApi.create(name, description);
      toast.success("商品已创建（草稿状态，记得上架）");
      setCreating(false); setName(""); setDescription("");
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "创建失败");
    }
  }

  async function toggleStatus(p: Product) {
    const next = p.status === "on_sale" ? "off_sale" : "on_sale";
    try {
      await productApi.updateStatus(p.id, next);
      toast.success(next === "on_sale" ? "已上架，买家可见" : "已下架");
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "操作失败");
    }
  }

  async function createSku() {
    if (!skuProduct) return;
    const specMap: Record<string, string> = {};
    specs.forEach((s) => { if (s.key.trim()) specMap[s.key.trim()] = s.value.trim(); });
    if (Object.keys(specMap).length === 0) { toast.error("至少填写一项规格"); return; }
    try {
      await productApi.createSKU(skuProduct.id, {
        code: skuCode.trim().toUpperCase(),
        specs: specMap,
        price_cent: Math.round(Number(skuPrice) * 100),
        stock: Number(skuStock),
      });
      toast.success("SKU 已创建");
      setSkuCode(""); setSkuPrice(""); setSkuStock(""); setSpecs([{ key: "", value: "" }]);
      await loadSkus(skuProduct);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "创建 SKU 失败");
    }
  }

  if (!isAdmin) return <Card><CardContent className="py-16 text-center text-sm text-muted-foreground">仅运营角色可管理商品</CardContent></Card>;
  if (loading) return <Skeleton className="h-40" />;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex justify-end">
        <Button onClick={() => setCreating((v) => !v)}>＋ 新建商品</Button>
      </div>

      {creating && (
        <Card>
          <CardContent className="grid gap-3 py-4">
            <div className="grid gap-1.5"><Label>商品名称</Label><Input value={name} onChange={(e) => setName(e.target.value)} /></div>
            <div className="grid gap-1.5"><Label>商品描述</Label><Input value={description} onChange={(e) => setDescription(e.target.value)} /></div>
            <div className="flex gap-2">
              <Button onClick={createProduct}>创建（草稿）</Button>
              <Button variant="outline" onClick={() => setCreating(false)}>取消</Button>
            </div>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow><TableHead>商品</TableHead><TableHead>状态</TableHead><TableHead className="text-right">操作</TableHead></TableRow>
            </TableHeader>
            <TableBody>
              {list.map((p) => (
                <TableRow key={p.id}>
                  <TableCell>
                    <p className="font-medium">{p.name}</p>
                    <p className="text-xs text-muted-foreground">{p.description}</p>
                  </TableCell>
                  <TableCell><ProductStatusBadge status={p.status} /></TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-1">
                      <Button size="sm" variant={p.status === "on_sale" ? "outline" : "default"} onClick={() => toggleStatus(p)}>
                        {p.status === "on_sale" ? "下架" : "上架"}
                      </Button>
                      <Button size="sm" variant="outline" onClick={() => { setSkuProduct(p); void loadSkus(p); }}>SKU/库存</Button>
                      <Button size="sm" variant="ghost" className="text-destructive" onClick={async () => {
                        try { await productApi.remove(p.id); toast.success("已删除"); await load(); }
                        catch (err) { toast.error(err instanceof Error ? err.message : "删除失败"); }
                      }}>删除</Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
              {list.length === 0 && <TableRow><TableCell colSpan={3} className="py-10 text-center text-sm text-muted-foreground">还没有商品</TableCell></TableRow>}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Dialog open={!!skuProduct} onOpenChange={(o) => !o && setSkuProduct(null)}>
        <DialogContent className="max-h-[80vh] overflow-y-auto sm:max-w-lg">
          <DialogHeader><DialogTitle>{skuProduct?.name} · 规格与库存</DialogTitle></DialogHeader>

          <div className="grid gap-2">
            {skus.map((s) => (
              <div key={s.id} className="flex items-center justify-between rounded-md border px-3 py-2 text-sm">
                <div>
                  <p className="font-medium">{s.code}</p>
                  <p className="text-xs text-muted-foreground">
                    {Object.entries(s.specs).map(([k, v]) => `${k}:${v}`).join(" · ")} · {yuan(s.price_cent)} · 库存 {s.stock}
                  </p>
                </div>
                <Badge variant={s.status === "active" ? "default" : "secondary"}>{s.status === "active" ? "启用" : "停用"}</Badge>
              </div>
            ))}
            {skus.length === 0 && <p className="py-4 text-center text-sm text-muted-foreground">暂无 SKU</p>}
          </div>

          <div className="grid gap-3 border-t pt-4">
            <p className="text-sm font-medium">新增 SKU</p>
            <div className="grid grid-cols-3 gap-2">
              <div className="grid gap-1.5"><Label className="text-xs">编码</Label><Input value={skuCode} onChange={(e) => setSkuCode(e.target.value)} placeholder="SKU-001" /></div>
              <div className="grid gap-1.5"><Label className="text-xs">价格（元）</Label><Input type="number" step="0.01" value={skuPrice} onChange={(e) => setSkuPrice(e.target.value)} /></div>
              <div className="grid gap-1.5"><Label className="text-xs">库存</Label><Input type="number" value={skuStock} onChange={(e) => setSkuStock(e.target.value)} /></div>
            </div>
            <div className="grid gap-2">
              <Label className="text-xs">规格（1~5 项）</Label>
              {specs.map((s, i) => (
                <div key={i} className="flex gap-2">
                  <Input placeholder="规格名，如 颜色" value={s.key} onChange={(e) => setSpecs(specs.map((x, j) => j === i ? { ...x, key: e.target.value } : x))} />
                  <Input placeholder="规格值，如 黑色" value={s.value} onChange={(e) => setSpecs(specs.map((x, j) => j === i ? { ...x, value: e.target.value } : x))} />
                  <Button variant="ghost" size="icon" disabled={specs.length <= 1}
                    onClick={() => setSpecs(specs.filter((_, j) => j !== i))}>×</Button>
                </div>
              ))}
              <Button variant="outline" size="sm" disabled={specs.length >= 5}
                onClick={() => setSpecs([...specs, { key: "", value: "" }])}>＋ 添加规格</Button>
            </div>
            <Button onClick={createSku}>创建 SKU</Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
