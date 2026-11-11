"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ClipboardIcon, Cross2Icon, DashboardIcon, ExitIcon, FileTextIcon, GearIcon, HamburgerMenuIcon, PersonIcon } from "@radix-ui/react-icons";
import { logout } from "@/lib/auth";
import type { CurrentUser } from "@/lib/schemas";
import { cn, roleLabel } from "@/lib/utils";
import { Brand } from "@/components/brand";
import { Button } from "@/components/ui/button";

const navigation = [
  { href: "/admin/dashboard", label: "学校总览", icon: DashboardIcon, roles: ["admin"] },
  { href: "/admin/teachers", label: "教师管理", icon: PersonIcon, roles: ["admin"] },
  { href: "/admin/questions", label: "全校题库", icon: FileTextIcon, roles: ["admin"] },
  { href: "/admin/settings", label: "学校设置", icon: GearIcon, roles: ["admin"] },
  { href: "/account", label: "个人中心", icon: PersonIcon, roles: ["admin"] },
  { href: "/teacher/dashboard", label: "个人总览", icon: DashboardIcon, roles: ["teacher"] },
  { href: "/teacher/questions", label: "教师题库", icon: FileTextIcon, roles: ["teacher"] },
  { href: "/teacher/papers", label: "组卷中心", icon: ClipboardIcon, roles: ["teacher"] },
  { href: "/teacher/profile", label: "个人中心", icon: PersonIcon, roles: ["teacher"] },
];

export function AppShell({ user, children }: { user: CurrentUser; children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [isOpen, setIsOpen] = useState(false);
  const logoutMutation = useMutation({
    mutationFn: logout,
    onSuccess: () => {
      queryClient.clear();
      router.replace("/login");
      router.refresh();
    },
  });

  useEffect(() => {
    function closeOnEscape(event: KeyboardEvent) {
      if (event.key === "Escape") setIsOpen(false);
    }
    document.addEventListener("keydown", closeOnEscape);
    return () => document.removeEventListener("keydown", closeOnEscape);
  }, []);

  return (
    <div className="min-h-dvh bg-background">
      {isOpen ? <button aria-label="关闭导航" className="fixed inset-0 z-30 bg-slate-950/35 lg:hidden" onClick={() => setIsOpen(false)} /> : null}
      <aside className={cn("app-shell-sidebar invisible fixed inset-y-0 left-0 z-40 flex w-60 -translate-x-full flex-col border-r border-slate-800 bg-slate-950 px-4 py-5 text-white transition-transform duration-300 ease-out lg:visible lg:translate-x-0", isOpen && "visible translate-x-0")}>
        <div className="flex h-11 items-center justify-between px-2">
          <Brand inverse />
          <Button variant="ghost" size="icon" className="text-slate-300 hover:bg-white/10 hover:text-white lg:hidden" aria-label="关闭导航" onClick={() => setIsOpen(false)}><Cross2Icon /></Button>
        </div>
        <div className="mt-8 px-3 text-xs font-medium tracking-[0.14em] text-slate-500">{user.role === "admin" ? "管理员工作区" : "教师工作区"}</div>
        <nav aria-label="主导航" className="mt-3 grid gap-1">
          {navigation.filter((item) => item.roles.includes(user.role)).map((item) => {
            const Icon = item.icon;
            const active = item.href === "/" ? pathname === item.href : pathname.startsWith(item.href);
            return (
              <Link key={item.href} href={item.href} onClick={() => setIsOpen(false)} aria-current={active ? "page" : undefined} className={cn("flex h-11 items-center gap-3 rounded-lg px-3 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400", active ? "bg-primary text-white" : "text-slate-300 hover:bg-white/8 hover:text-white")}>
                <Icon className="size-4" />{item.label}
              </Link>
            );
          })}
        </nav>
        <div className="mt-auto border-t border-white/10 px-3 pt-5">
          <p className="truncate text-sm font-medium text-slate-100">{user.name}</p>
          <p className="mt-1 text-xs text-slate-400">{roleLabel(user.role)}</p>
          <p className="mt-4 text-[11px] leading-4 text-slate-500">学校 ID</p>
          <p className="mt-1 break-all font-mono text-[11px] leading-4 text-slate-400">{user.school_id}</p>
        </div>
      </aside>
      <div className="app-shell-content min-h-dvh lg:pl-60">
        <header className="app-shell-header sticky top-0 z-20 flex h-16 items-center justify-between border-b border-border bg-white/95 px-4 backdrop-blur sm:px-6 lg:px-8">
          <div className="flex items-center gap-3 lg:hidden">
            <Button variant="ghost" size="icon" aria-label="打开导航" aria-expanded={isOpen} onClick={() => setIsOpen(true)}><HamburgerMenuIcon /></Button>
            <Brand />
          </div>
          <div className="hidden lg:block">
            <p className="text-sm font-medium text-slate-800">{roleLabel(user.role)}</p>
            <p className="mt-0.5 text-xs text-slate-500">{user.email}</p>
          </div>
          <div className="flex items-center gap-3">
            <span className="hidden text-sm text-slate-600 sm:inline">{user.name}</span>
            <Button variant="outline" size="sm" disabled={logoutMutation.isPending} onClick={() => logoutMutation.mutate()}><ExitIcon />{logoutMutation.isPending ? "正在退出…" : "退出"}</Button>
          </div>
        </header>
        {logoutMutation.isError ? <p role="alert" className="border-b border-red-200 bg-red-50 px-6 py-2 text-sm text-red-700">{logoutMutation.error.message}</p> : null}
        <main className="app-shell-main p-5 sm:p-6 lg:p-8">{children}</main>
      </div>
    </div>
  );
}
