import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function roleLabel(role: "admin" | "teacher") {
  return role === "admin" ? "学校管理员" : "学科教师";
}
