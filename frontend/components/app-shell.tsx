"use client";

import { useCallback, useRef, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ExitIcon, HamburgerMenuIcon } from "@radix-ui/react-icons";
import { logout } from "@/lib/auth";
import type { CurrentUser } from "@/lib/schemas";
import { roleLabel } from "@/lib/utils";
import { Brand } from "@/components/brand";
import { MobileNavigation, NavigationContent } from "@/components/mobile-navigation";
import { Button } from "@/components/ui/button";

export function AppShell({ user, children }: { user: CurrentUser; children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const queryClient = useQueryClient();
  const menuButtonRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  const closeNavigation = useCallback(() => setIsOpen(false), []);
  const logoutMutation = useMutation({
    mutationFn: logout,
    onSuccess: () => {
      queryClient.clear();
      router.replace("/login");
      router.refresh();
    },
  });

  return (
    <div className="min-h-dvh bg-paper">
      <a href="#main-content" className="skip-link">跳到主内容</a>
      <MobileNavigation open={isOpen} user={user} pathname={pathname} onClose={closeNavigation} triggerRef={menuButtonRef} />
      <aside className="app-shell-sidebar fixed inset-y-0 left-0 z-40 hidden w-60 flex-col border-r border-border bg-neutral-2/85 px-4 py-6 backdrop-blur-xl lg:flex">
        <NavigationContent user={user} pathname={pathname} />
      </aside>
      <div className="app-shell-content min-h-dvh lg:pl-60">
        <header className="app-shell-header sticky top-0 z-20 flex h-16 items-center justify-between border-b border-border bg-paper/80 px-4 backdrop-blur-sm sm:px-6 lg:px-8">
          <div className="flex items-center gap-3 lg:hidden">
            <Button ref={menuButtonRef} id="mobile-navigation-trigger" variant="ghost" size="icon" aria-label="打开导航" aria-expanded={isOpen} aria-controls="mobile-navigation" onClick={() => setIsOpen(true)}><HamburgerMenuIcon /></Button>
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
        <main id="main-content" tabIndex={-1} className="app-shell-main mx-auto w-full max-w-[1560px] p-5 sm:p-8 lg:p-10">{children}</main>
      </div>
    </div>
  );
}
