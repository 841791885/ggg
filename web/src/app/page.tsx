import { redirect } from "next/navigation";

/** 首页直接进商城；未登录时 AppShell 会拦下显示登录页。 */
export default function Home() {
  redirect("/shop");
}
