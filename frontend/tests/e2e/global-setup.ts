import fs from "node:fs";
import {
  ADMIN_EMAIL,
  ADMIN_PASSWORD,
  TEACHER_EMAIL,
  TEACHER_PASSWORD,
  adminStorage,
  apiContext,
  authDirectory,
  storedMeta,
  teacherStorage,
  writeMeta,
} from "./helpers";

interface CurrentUserPayload {
  user: { id: string; email: string };
}

async function loginAndStore(email: string, password: string, storage: string) {
  const api = await apiContext();
  const response = await api.post("/api/v1/auth/login", { data: { email, password } });
  if (!response.ok()) {
    await api.dispose();
    return null;
  }
  const body = (await response.json()) as CurrentUserPayload;
  await api.storageState({ path: storage });
  await api.dispose();
  return body.user;
}

export default async function globalSetup(): Promise<void> {
  fs.mkdirSync(authDirectory, { recursive: true });
  const api = await apiContext();
  const statusResponse = await api.get("/api/v1/init/status");
  if (!statusResponse.ok()) {
    throw new Error(`无法读取初始化状态：${statusResponse.status()}`);
  }
  const status = (await statusResponse.json()) as { initialized: boolean };
  let schoolCode = storedMeta()?.schoolCode ?? null;

  if (!status.initialized) {
    const response = await api.post("/api/v1/init", {
      data: {
        school_name: "E2E 测试学校",
        admin_name: "E2E 管理员",
        email: ADMIN_EMAIL,
        password: ADMIN_PASSWORD,
      },
    });
    if (!response.ok()) {
      throw new Error(`初始化学校失败：${response.status()} ${await response.text()}`);
    }
    schoolCode = ((await response.json()) as { school_code: string }).school_code;
  }

  let teacher = await loginAndStore(TEACHER_EMAIL, TEACHER_PASSWORD, teacherStorage);
  if (!teacher && schoolCode) {
    const register = await api.post("/api/v1/auth/register", {
      data: {
        school_code: schoolCode,
        name: "E2E 数学教师",
        subject: "数学",
        email: TEACHER_EMAIL,
        password: TEACHER_PASSWORD,
      },
    });
    if (!register.ok() && register.status() !== 409) {
      throw new Error(`注册 E2E 教师失败：${register.status()} ${await register.text()}`);
    }
    teacher = await loginAndStore(TEACHER_EMAIL, TEACHER_PASSWORD, teacherStorage);
  }
  await api.dispose();

  if (!teacher) {
    throw new Error("无法登录 E2E 教师账号。若 E2E 数据库由其他流程初始化且学校代码未知，请重建 school_db_e2e 后重试。");
  }
  const admin = await loginAndStore(ADMIN_EMAIL, ADMIN_PASSWORD, adminStorage);
  if (!admin) {
    throw new Error("无法登录 E2E 管理员账号。请确认 E2E_ADMIN_EMAIL/E2E_ADMIN_PASSWORD 与数据库一致。");
  }
  writeMeta({ schoolCode, adminId: admin.id, teacherId: teacher.id });
}
