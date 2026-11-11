import { describe, expect, it } from "vitest";
import { fromCents, sumScores, toCents } from "./score";

describe("整数百分分值", () => {
  it("把严格的两位小数字符串转换为整数百分", () => {
    expect(toCents("0.10")).toBe(10);
    expect(toCents("12.34")).toBe(1234);
    expect(toCents("999999.99")).toBe(99_999_999);
  });

  it.each(["0.00", "00.10", "01.00", "0", "1", "1.2", "1.234", "-1.00", " 1.00", "1.00 ", "1000000.00", "NaN"])(
    "拒绝非法或非正数分值 %s",
    (value) => {
      expect(() => toCents(value)).toThrow();
    },
  );

  it("把整数百分格式化为两位小数", () => {
    expect(fromCents(0)).toBe("0.00");
    expect(fromCents(10)).toBe("0.10");
    expect(fromCents(1234)).toBe("12.34");
  });

  it.each([-1, 1.5, Number.NaN, 100_000_000])("拒绝非法整数值 %s", (value) => {
    expect(() => fromCents(value)).toThrow();
  });

  it("避免二进制浮点求和误差", () => {
    expect(sumScores(["0.10", "0.20"])).toBe("0.30");
    expect(sumScores(Array.from({ length: 100 }, () => "0.01"))).toBe("1.00");
    expect(sumScores([])).toBe("0.00");
  });

  it("求和时拒绝任何非法分值", () => {
    expect(() => sumScores(["0.10", "0.00"])).toThrow();
    expect(() => sumScores(["0.10", "0.2"])).toThrow();
  });
});
