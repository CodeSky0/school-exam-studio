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
    <div className="min-h-dvh bg-paper">
      {isOpen ? <button aria-label="关闭导航" className="fixed inset-0 z-30 bg-neutral-10/25 backdrop-blur-sm lg:hidden" onClick={() => setIsOpen(false)} /> : null}
      <aside className={cn("app-shell-sidebar invisible fixed inset-y-0 left-0 z-40 flex w-60 -translate-x-full flex-col border-r border-border bg-neutral-2/85 px-4 py-6 backdrop-blur-xl transition-transform duration-300 ease-out lg:visible lg:translate-x-0", isOpen && "visible translate-x-0")}>
        <div className="flex h-10 items-center justify-between px-2">
          <Brand />
          <Button variant="ghost" size="icon" className="lg:hidden" aria-label="关闭导航" onClick={() => setIsOpen(false)}><Cross2Icon /></Button>
        </div>
        <div className="mt-8 px-3 text-caption-10 font-medium uppercase tracking-[0.18em] text-neutral-6">{user.role === "admin" ? "管理员工作区" : "教师工作区"}</div>
        <nav aria-label="主导航" className="mt-3 grid gap-1">
          {navigation.filter((item) => item.roles.includes(user.role)).map((item) => {
            const Icon = item.icon;
            const active = item.href === "/" ? pathname === item.href : pathname.startsWith(item.href);
            return (
              <Link key={item.href} href={item.href} onClick={() => setIsOpen(false)} aria-current={active ? "page" : undefined} className={cn("relative flex h-10 items-center gap-3 rounded-md px-3 text-copy-13 font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/35", active ? "bg-paper text-accent-strong ring-1 ring-border" : "text-neutral-7 hover:bg-neutral-3/70 hover:text-neutral-10")}>
                {active ? <span className="absolute inset-y-2 left-0 w-0.5 rounded-full bg-accent" aria-hidden="true" /> : null}
                <Icon className="size-4" />{item.label}
              </Link>
            );
          })}
        </nav>
        <div className="mt-auto border-t border-border px-3 pt-5">
          <p className="truncate text-copy-13 font-medium text-neutral-9">{user.name}</p>
          <p className="mt-1 text-label-12 text-neutral-7">{roleLabel(user.role)}</p>
          <p className="mt-5 text-caption-10 uppercase tracking-[0.16em] text-neutral-6">学校 ID</p>
          <p className="mt-1 break-all font-mono text-label-12 leading-5 text-neutral-7">{user.school_id}</p>
        </div>
      </aside>
      <div className="app-shell-content min-h-dvh lg:pl-60">
        <header className="app-shell-header sticky top-0 z-20 flex h-16 items-center justify-between border-b border-border bg-paper/80 px-4 backdrop-blur-sm sm:px-6 lg:px-8">
          <div className="flex items-center gap-3 lg:hidden">
            <Button variant="ghost" size="icon" aria-label="打开导航" aria-expanded={isOpen} onClick={() => setIsOpen(true)}><HamburgerMenuIcon /></Button>
            <Brand />
          </div>
          <div className="hidden lg:block">
            <p className="text-copy-13 font-medium text-neutral-8">{roleLabel(user.role)}</p>
            <p className="mt-0.5 text-label-12 text-neutral-6">{user.email}</p>
          </div>
          <div className="flex items-center gap-3">
            <span className="hidden text-copy-13 text-neutral-7 sm:inline">{user.name}</span>
            <Button variant="outline" size="sm" disabled={logoutMutation.isPending} onClick={() => logoutMutation.mutate()}><ExitIcon />{logoutMutation.isPending ? "正在退出…" : "退出"}</Button>
          </div>
        </header>
        {logoutMutation.isError ? <p role="alert" className="border-b border-error-soft-strong bg-error-soft px-6 py-2 text-copy-13 text-error">{logoutMutation.error.message}</p> : null}
        <main className="app-shell-main mx-auto w-full max-w-[1560px] p-5 sm:p-8 lg:p-10">{children}</main>
      </div>
    </div>
  );
}