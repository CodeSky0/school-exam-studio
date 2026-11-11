import { apiRequest } from "./api";
import {
  initResponseSchema,
  initStatusSchema,
  logoutResponseSchema,
  userResponseSchema,
  type InitFormValues,
  type LoginFormValues,
  type RegisterFormValues,
} from "./schemas";

export function getInitStatus() {
  return apiRequest("/api/v1/init/status", initStatusSchema);
}

export function initializeSchool(values: InitFormValues) {
  return apiRequest("/api/v1/init", initResponseSchema, {
    method: "POST",
    body: JSON.stringify({
      school_name: values.schoolName,
      admin_name: values.adminName,
      email: values.email,
      password: values.password,
    }),
  });
}

export function registerTeacher(values: RegisterFormValues) {
  return apiRequest("/api/v1/auth/register", userResponseSchema, {
    method: "POST",
    body: JSON.stringify({
      school_code: values.schoolCode,
      name: values.name,
      subject: values.subject,
      email: values.email,
      password: values.password,
    }),
  });
}

export function login(values: LoginFormValues) {
  return apiRequest("/api/v1/auth/login", userResponseSchema, {
    method: "POST",
    body: JSON.stringify(values),
  });
}

export function logout() {
  return apiRequest("/api/v1/auth/logout", logoutResponseSchema, { method: "POST" });
}

export function changePassword(values: { old_password: string; new_password: string }) {
  return apiRequest("/api/v1/auth/change-password", logoutResponseSchema, {
    method: "POST",
    body: JSON.stringify(values),
  });
}

export async function getCurrentUser() {
  const response = await apiRequest("/api/v1/auth/me", userResponseSchema);
  return response.user;
}
