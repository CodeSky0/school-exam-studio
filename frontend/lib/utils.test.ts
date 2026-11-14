import { describe, expect, it } from "vitest";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "./utils";

describe("cn 类名合并", () => {
  it("自定义字号 token 不应吞掉文字颜色类", () => {
    const merged = cn("text-copy-13", "bg-accent text-accent-foreground", "text-copy-14");
    expect(merged).toContain("text-accent-foreground");
    expect(merged).toContain("text-copy-14");
  });

  it("主按钮所有尺寸组合都保留 accent 前景色", () => {
    for (const size of ["default", "sm", "lg"] as const) {
      const classes = buttonVariants({ variant: "default", size });
      expect(classes).toContain("text-accent-foreground");
    }
  });

  it("危险按钮所有尺寸组合都保留 accent 前景色", () => {
    for (const size of ["default", "sm", "lg"] as const) {
      const classes = buttonVariants({ variant: "destructive", size });
      expect(classes).toContain("text-accent-foreground");
    }
  });

  it("调用方显式指定的文字颜色仍可覆盖变体默认值", () => {
    const merged = cn("bg-accent text-accent-foreground", "text-error");
    expect(merged).toContain("text-error");
    expect(merged).not.toContain("text-accent-foreground");
  });
});
