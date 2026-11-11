import { z } from "zod";

export const schoolCodeSchema = z.string().regex(/^\d{8}$/, "学校代码须为 8 位数字");
export const nameSchema = z.string().trim().min(2, "姓名须为 2 至 50 个字符").max(50, "姓名须为 2 至 50 个字符");
export const subjectSchema = z.string().trim().min(1, "请输入学科").max(30, "学科不能超过 30 个字符");
export const emailSchema = z.string().trim().toLowerCase().pipe(z.email("请输入有效邮箱"));
export const passwordSchema = z.string().min(10, "密码须为 10 至 72 个字符").max(72, "密码须为 10 至 72 个字符");

export const initSchema = z.object({
  schoolName: z.string().trim().min(2, "学校名称须为 2 至 200 个字符").max(200, "学校名称须为 2 至 200 个字符"),
  adminName: nameSchema,
  email: emailSchema,
  password: passwordSchema,
});

export const registerSchema = z.object({
  schoolCode: schoolCodeSchema,
  name: nameSchema,
  subject: subjectSchema,
  email: emailSchema,
  password: passwordSchema,
});

export const loginSchema = z.object({
  email: emailSchema,
  password: passwordSchema,
});

export const schoolNameSchema = z.string().trim().min(2, "学校名称须为 2 至 200 个字符").max(200, "学校名称须为 2 至 200 个字符");

export const changePasswordSchema = z.object({
  old_password: passwordSchema,
  new_password: passwordSchema,
  confirm_password: z.string().min(1, "请再次输入新密码"),
}).refine((values) => values.old_password !== values.new_password, {
  path: ["new_password"],
  message: "新密码不能与原密码相同",
}).refine((values) => values.new_password === values.confirm_password, {
  path: ["confirm_password"],
  message: "两次输入的新密码不一致",
});

export const initStatusSchema = z.object({
  initialized: z.boolean(),
});

export const initResponseSchema = z.object({
  school_code: schoolCodeSchema,
});

export const currentUserSchema = z.object({
  id: z.string().uuid(),
  school_id: z.string().uuid(),
  name: z.string().min(1),
  subject: z.string(),
  email: z.email(),
  role: z.enum(["admin", "teacher"]),
  status: z.enum(["active", "disabled"]).optional(),
  created_at: z.string().optional(),
  updated_at: z.string().optional(),
});

export const userResponseSchema = z.object({ user: currentUserSchema });
export const logoutResponseSchema = z.object({ status: z.literal("ok") });

export type InitFormValues = z.infer<typeof initSchema>;
export type RegisterFormValues = z.infer<typeof registerSchema>;
export type LoginFormValues = z.infer<typeof loginSchema>;
export type CurrentUser = z.infer<typeof currentUserSchema>;
export type InitResponse = z.infer<typeof initResponseSchema>;
