import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

const twMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      "font-size": [
        "text-caption-10",
        "text-label-12",
        "text-copy-13",
        "text-copy-14",
        "text-copy-15",
        "text-copy-16",
        "text-title-20",
        "text-title-24",
        "text-title-28",
        "text-display-36",
        "text-display-48",
        "text-icon-sm",
        "text-icon-md",
        "text-icon-lg",
      ],
    },
  },
});

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function roleLabel(role: "admin" | "teacher") {
  return role === "admin" ? "学校管理员" : "学科教师";
}
