"use client";

import { useEffect, useRef, type RefObject } from "react";
import Link from "next/link";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { ClipboardIcon, Cross2Icon, DashboardIcon, FileTextIcon, GearIcon, MagicWandIcon, PersonIcon } from "@radix-ui/react-icons";
import type { CurrentUser } from "@/lib/schemas";
import { cn, roleLabel } from "@/lib/utils";
import { Brand } from "@/components/brand";
import { Button } from "@/components/ui/button";
import { acquireBodyScrollLock } from "@/components/ui/animated-dialog";

const navigation = [
  { href: "/admin/dashboard", label: "学校总览", icon: DashboardIcon, roles: ["admin"] },
  { href: "/admin/ai", label: "AI 工作台", icon: MagicWandIcon, roles: ["admin"] },
  { href: "/admin/teachers", label: "教师管理", icon: PersonIcon, roles: ["admin"] },
  { href: "/admin/questions", label: "全校题库", icon: FileTextIcon, roles: ["admin"] },
  { href: "/admin/settings", label: "学校设置", icon: GearIcon, roles: ["admin"] },
  { href: "/account", label: "个人中心", icon: PersonIcon, roles: ["admin"] },
  { href: "/teacher/dashboard", label: "个人总览", icon: DashboardIcon, roles: ["teacher"] },
  { href: "/teacher/ai", label: "AI 工作台", icon: MagicWandIcon, roles: ["teacher"] },
  { href: "/teacher/questions", label: "教师题库", icon: FileTextIcon, roles: ["teacher"] },
  { href: "/teacher/papers", label: "组卷中心", icon: ClipboardIcon, roles: ["teacher"] },
  { href: "/teacher/profile", label: "个人中心", icon: PersonIcon, roles: ["teacher"] },
];

interface NavigationContentProps {
  user: CurrentUser;
  pathname: string;
  onNavigate?: () => void;
  closeButtonRef?: RefObject<HTMLButtonElement | null>;
}

export function NavigationContent({ user, pathname, onNavigate, closeButtonRef }: NavigationContentProps) {
  return (
    <>
      <div className="flex h-10 items-center justify-between px-2">
        <Brand />
        {onNavigate ? <Button ref={closeButtonRef} type="button" variant="ghost" size="icon" className="lg:hidden" aria-label="关闭导航" onClick={onNavigate}><Cross2Icon /></Button> : null}
      </div>
      <div className="mt-8 px-3 text-caption-10 font-medium uppercase tracking-[0.18em] text-neutral-6">{user.role === "admin" ? "管理员工作区" : "教师工作区"}</div>
      <nav aria-label="主导航" className="mt-3 grid gap-1">
        {navigation.filter((item) => item.roles.includes(user.role)).map((item) => {
          const Icon = item.icon;
          const active = item.href === "/" ? pathname === item.href : pathname.startsWith(item.href);
          return (
            <Link key={item.href} href={item.href} onClick={onNavigate} aria-current={active ? "page" : undefined} className={cn("relative flex h-10 items-center gap-3 rounded-md px-3 text-copy-13 font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/35", active ? "bg-paper text-accent-strong ring-1 ring-border" : "text-neutral-7 hover:bg-neutral-3/70 hover:text-neutral-10")}>
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
    </>
  );
}

interface MobileNavigationProps {
  open: boolean;
  user: CurrentUser;
  pathname: string;
  onClose: () => void;
  triggerRef: RefObject<HTMLButtonElement | null>;
}

function MobileNavigationDialog({ user, pathname, onClose }: Omit<MobileNavigationProps, "open" | "triggerRef">) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const shouldReduceMotion = useReducedMotion();

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (!dialog.open) {
      if (typeof dialog.showModal === "function") {
        dialog.showModal();
      } else {
        dialog.setAttribute("open", "");
      }
    }
    const releaseBodyScrollLock = acquireBodyScrollLock();
    closeButtonRef.current?.focus();

    return () => {
      if (dialog.open) {
        if (typeof dialog.close === "function") {
          dialog.close();
        } else {
          dialog.removeAttribute("open");
        }
      }
      releaseBodyScrollLock();
    };
  }, []);

  return (
    <dialog
      ref={dialogRef}
      id="mobile-navigation"
      aria-label="主导航"
      onCancel={(event) => {
        event.preventDefault();
        onClose();
      }}
      className="fixed inset-0 m-0 h-dvh max-h-none w-full max-w-none overflow-hidden border-0 bg-transparent p-0 text-neutral-9 backdrop:bg-transparent"
    >
      <motion.button
        type="button"
        tabIndex={-1}
        aria-label="关闭导航"
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        exit={{ opacity: 0 }}
        transition={{ duration: shouldReduceMotion ? 0 : 0.2 }}
        className="absolute inset-0 bg-neutral-10/25 backdrop-blur-sm"
        onClick={onClose}
      />
      <motion.aside
        initial={shouldReduceMotion ? { opacity: 0 } : { x: "-100%" }}
        animate={shouldReduceMotion ? { opacity: 1 } : { x: 0 }}
        exit={shouldReduceMotion ? { opacity: 0 } : { x: "-100%" }}
        transition={{ duration: shouldReduceMotion ? 0 : 0.24, ease: [0.16, 1, 0.3, 1] }}
        className="app-shell-sidebar absolute inset-y-0 left-0 z-10 flex w-60 flex-col border-r border-border bg-neutral-2/85 px-4 py-6 backdrop-blur-xl"
      >
        <NavigationContent user={user} pathname={pathname} onNavigate={onClose} closeButtonRef={closeButtonRef} />
      </motion.aside>
    </dialog>
  );
}

export function MobileNavigation({ open, user, pathname, onClose, triggerRef }: MobileNavigationProps) {
  useEffect(() => {
    if (typeof window.matchMedia !== "function") return;
    const desktopQuery = window.matchMedia("(min-width: 1024px)");
    const closeAtDesktop = (event: MediaQueryListEvent) => {
      if (event.matches) onClose();
    };
    desktopQuery.addEventListener("change", closeAtDesktop);
    if (open && desktopQuery.matches) onClose();
    return () => desktopQuery.removeEventListener("change", closeAtDesktop);
  }, [onClose, open]);

  return (
    <AnimatePresence onExitComplete={() => {
      if (!open) triggerRef.current?.focus();
    }}>
      {open ? <MobileNavigationDialog key="mobile-navigation" user={user} pathname={pathname} onClose={onClose} /> : null}
    </AnimatePresence>
  );
}
