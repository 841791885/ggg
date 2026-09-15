"use client";

/** 收货地址：列表 + 新增/编辑表单（默认地址切换）。 */
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { addressApi } from "@/lib/api";
import type { Address } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";

type FormState = { id?: number; recipient: string; phone: string; province: string; city: string; district: string; detail: string };
const EMPTY: FormState = { recipient: "", phone: "", province: "", city: "", district: "", detail: "" };

export default function AddressesPage() {
  const [list, setList] = useState<Address[]>([]);
  const [loading, setLoading] = useState(true);
  const [form, setForm] = useState<FormState | null>(null);
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    try {
      setList((await addressApi.list()).items ?? []);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "加载地址失败");
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => { void load(); }, [load]);

  async function save() {
    if (!form) return;
    setSaving(true);
    try {
      const body = { recipient: form.recipient, phone: form.phone, province: form.province, city: form.city, district: form.district, detail: form.detail };
      if (form.id) await addressApi.update(form.id, body);
      else await addressApi.create(body);
      toast.success("地址已保存");
      setForm(null);
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }

  if (loading) return <Skeleton className="h-40" />;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex justify-end">
        <Button onClick={() => setForm({ ...EMPTY })}>＋ 新增地址</Button>
      </div>

      {list.length === 0 ? (
        <Card><CardContent className="py-16 text-center text-sm text-muted-foreground">还没有收货地址，结算前需要至少一个</CardContent></Card>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2">
          {list.map((a) => (
            <Card key={a.id} className={a.is_default ? "border-emerald-500" : ""}>
              <CardContent className="flex items-center justify-between gap-3 py-4">
                <div className="min-w-0">
                  <p className="text-sm font-medium">
                    {a.recipient} {a.phone}
                    {a.is_default && <Badge className="ml-2 border-0">默认</Badge>}
                  </p>
                  <p className="truncate text-xs text-muted-foreground">{a.province}{a.city}{a.district}{a.detail}</p>
                </div>
                <div className="flex shrink-0 gap-1">
                  {!a.is_default && (
                    <Button size="sm" variant="outline" onClick={async () => { await addressApi.setDefault(a.id); await load(); }}>设默认</Button>
                  )}
                  <Button size="sm" variant="ghost" onClick={() => setForm({ ...a })}>编辑</Button>
                  <Button size="sm" variant="ghost" className="text-destructive" onClick={async () => { await addressApi.remove(a.id); toast.success("已删除"); await load(); }}>删除</Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {form && (
        <Card>
          <CardContent className="grid gap-3 py-4 sm:grid-cols-2">
            <div className="grid gap-1.5"><Label>收件人</Label><Input value={form.recipient} onChange={(e) => setForm({ ...form, recipient: e.target.value })} /></div>
            <div className="grid gap-1.5"><Label>手机号</Label><Input value={form.phone} onChange={(e) => setForm({ ...form, phone: e.target.value })} /></div>
            <div className="grid gap-1.5"><Label>省</Label><Input value={form.province} onChange={(e) => setForm({ ...form, province: e.target.value })} /></div>
            <div className="grid gap-1.5"><Label>市</Label><Input value={form.city} onChange={(e) => setForm({ ...form, city: e.target.value })} /></div>
            <div className="grid gap-1.5"><Label>区</Label><Input value={form.district} onChange={(e) => setForm({ ...form, district: e.target.value })} /></div>
            <div className="grid gap-1.5"><Label>详细地址</Label><Input value={form.detail} onChange={(e) => setForm({ ...form, detail: e.target.value })} /></div>
            <div className="flex gap-2 sm:col-span-2">
              <Button onClick={save} disabled={saving}>{saving ? "保存中…" : "保存"}</Button>
              <Button variant="outline" onClick={() => setForm(null)}>取消</Button>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
