"use client";

import { AnimatePresence, motion } from "framer-motion";
import { cn } from "@/lib/utils";

type StatusMessageTone = "success" | "info" | "warning" | "error";

interface StatusMessageProps {
  message?: string | null;
  tone?: StatusMessageTone;
  className?: string;
}

const toneStyles: Record<StatusMessageTone, string> = {
  success: "border-success/25 bg-success-soft text-success",
  info: "border-info/25 bg-info-soft text-info",
  warning: "border-warning/30 bg-warning-soft text-warning",
  error: "border-error-soft-strong bg-error-soft text-error",
};

export function StatusMessage({ message, tone = "info", className }: StatusMessageProps) {
  return (
    <AnimatePresence mode="wait">
      {message ? (
        <motion.p
          key={`${tone}:${message}`}
          role={tone === "error" ? "alert" : "status"}
          initial={{ opacity: 0, y: -4 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -4 }}
          transition={{ duration: 0.18 }}
          className={cn("rounded-lg border px-4 py-3 text-copy-13 leading-6", toneStyles[tone], className)}
        >
          {message}
        </motion.p>
      ) : null}
    </AnimatePresence>
  );
}
