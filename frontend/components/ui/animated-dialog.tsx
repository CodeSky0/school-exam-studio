"use client";

import { useEffect, useRef, type ReactNode } from "react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";

interface AnimatedDialogProps {
  open: boolean;
  labelledBy: string;
  describedBy?: string;
  onClose: () => void;
  isPending?: boolean;
  className?: string;
  children: ReactNode;
}

let bodyScrollLockCount = 0;
let previousBodyOverflow = "";

export function acquireBodyScrollLock() {
  if (bodyScrollLockCount === 0) {
    previousBodyOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
  }
  bodyScrollLockCount += 1;
  let released = false;

  return () => {
    if (released) return;
    released = true;
    bodyScrollLockCount = Math.max(0, bodyScrollLockCount - 1);
    if (bodyScrollLockCount === 0) {
      document.body.style.overflow = previousBodyOverflow;
    }
  };
}

function AnimatedDialogSurface({ labelledBy, describedBy, onClose, isPending = false, className, children }: Omit<AnimatedDialogProps, "open">) {
  const dialogRef = useRef<HTMLDialogElement>(null);
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
    <motion.dialog
      ref={dialogRef}
      aria-labelledby={labelledBy}
      aria-describedby={describedBy}
      initial={shouldReduceMotion ? { opacity: 0 } : { opacity: 0, y: 12, scale: 0.98 }}
      animate={shouldReduceMotion ? { opacity: 1 } : { opacity: 1, y: 0, scale: 1 }}
      exit={shouldReduceMotion ? { opacity: 0 } : { opacity: 0, y: 8, scale: 0.98 }}
      transition={{ duration: shouldReduceMotion ? 0 : 0.2, ease: [0.16, 1, 0.3, 1] }}
      onCancel={(event) => {
        event.preventDefault();
        if (!isPending) onClose();
      }}
      className={className}
    >
      {children}
    </motion.dialog>
  );
}

export function AnimatedDialog({ open, isPending = false, ...props }: AnimatedDialogProps) {
  return (
    <AnimatePresence>
      {open ? <AnimatedDialogSurface key="animated-dialog" isPending={isPending} {...props} /> : null}
    </AnimatePresence>
  );
}
