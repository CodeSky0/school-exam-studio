import { describe, expect, it } from "vitest";
import {
  currentUserSchema,
  emailSchema,
  initSchema,
  loginSchema,
  nameSchema,
  passwordSchema,
  registerSchema,
  schoolCodeSchema,
  subjectSchema,
} from "./schemas";

describe("表单字段校验", () => {
  it("只接受八位数字学校代码", () => {
    expect(schoolCodeSchema.parse("01234567")).toBe("01234567");
    expect(schoolCodeSchema.safeParse("1234567").success).toBe(false);
    expect(schoolCodeSchema.safeParse("123456789").success).toBe(false);
    expect(schoolCodeSchema.safeParse("1234567a").success).toBe(false);
  });

  it("修剪姓名并限制二至五十个字符", () => {
    expect(nameSchema.parse("  李老师  ")).toBe("李老师");
    expect(nameSchema.safeParse("李").success).toBe(false);
    expect(nameSchema.safeParse("李".repeat(51)).success).toBe(false);
  });

  it("修剪学科并限制一至三十个字符", () => {
    expect(subjectSchema.parse(" 数学 ")).toBe("数学");
    expect(subjectSchema.safeParse("   ").success).toBe(false);
    expect(subjectSchema.safeParse("数".repeat(31)).success).toBe(false);
  });

  it("规范化有效邮箱并拒绝无效邮箱", () => {
    expect(emailSchema.parse(" Teacher@Example.COM ")).toBe("teacher@example.com");
    expect(emailSchema.safeParse("teacher@invalid").success).toBe(false);
  });

  it("接受十至七十二位密码的边界值", () => {
    expect(passwordSchema.safeParse("1".repeat(10)).success).toBe(true);
    expect(passwordSchema.safeParse("1".repeat(72)).success).toBe(true);
    expect(passwordSchema.safeParse("1".repeat(9)).success).toBe(false);
    expect(passwordSchema.safeParse("1".repeat(73)).success).toBe(false);
  });

  it("拒绝 UTF-8 编码超过七十二字节的密码", () => {
    expect(passwordSchema.safeParse("密".repeat(24)).success).toBe(true);
    expect(passwordSchema.safeParse("密".repeat(25)).success).toBe(false);
  });
});

describe("认证表单与响应校验", () => {
  it("解析完整初始化表单", () => {
    const result = initSchema.parse({
      schoolName: "  实验中学  ",
      adminName: "  陈老师 ",
      email: "ADMIN@EXAMPLE.COM",
      password: "StrongPass123!",
    });
    expect(result).toEqual({
      schoolName: "实验中学",
      adminName: "陈老师",
      email: "admin@example.com",
      password: "StrongPass123!",
    });
  });

  it("解析完整注册表单", () => {
    const result = registerSchema.parse({
      schoolCode: "12345678",
      name: "周老师",
      subject: "物理",
      email: "TEACHER@EXAMPLE.COM",
      password: "StrongPass123!",
    });
    expect(result.email).toBe("teacher@example.com");
  });

  it("登录表单同样执行密码长度约束", () => {
    expect(loginSchema.safeParse({ email: "admin@example.com", password: "short" }).success).toBe(false);
  });

  it("拒绝缺少学校标识的当前用户响应", () => {
    expect(currentUserSchema.safeParse({
      id: "e2e01f6c-b5b2-4d0a-9f4f-cfd5e518a7a8",
      name: "陈老师",
      email: "admin@example.com",
      role: "admin",
      subject: "",
    }).success).toBe(false);
  });
});
