# API 说明

- 前缀：`/api/v1`
- 认证：登录成功后签发 HS256 JWT（2 小时过期），写入 `HttpOnly`、`SameSite=Strict` Cookie；生产环境带 `Secure`。声明包含 `user_id`、`school_id`、`role`、`session_version`。
- 租户与角色只能来自认证上下文，客户端传入的学校字段一律忽略；跨学校资源返回 404，角色不足返回 403。
- 所有修改请求必须携带与 `CORS_ORIGIN` 一致的 `Origin` 头。
- 登录与注册按 IP 与账号组合限流（Redis 滑动窗口）。

## 统一错误格式

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "请求内容未通过校验",
    "details": []
  },
  "request_id": "..."
}
```

状态码：400、401、403、404、409、413、415、422、429、503、504、500。客户端不会收到 SQL、堆栈、内部路径或密钥。

## 初始化与认证

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/init/status` | 公开 | 返回 `{"initialized": boolean}` |
| POST | `/init` | 公开（仅一次） | 创建学校与管理员，事务内咨询锁保证只成功一次；重复返回 409；响应仅展示一次 `school_code`（8 位数字） |
| POST | `/auth/register` | 公开 | 凭学校代码注册 active 教师，邮箱校内唯一 |
| POST | `/auth/login` | 公开 | 错误邮箱/密码返回相同通用错误；成功设置认证 Cookie |
| POST | `/auth/logout` | 登录 | 退出登录 |
| GET | `/auth/me` | 登录 | 当前用户；禁用或会话版本变化返回 401 |
| POST | `/auth/change-password` | 登录 | 修改密码并递增会话版本 |

## 题目

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/questions` | 教师/管理员 | 组合筛选：`subject`、`grade`、`question_type`、`tag`、`creator_id`、`status`、`difficulty`、`page`、`page_size`（≤100）。教师见自己全部状态及他人 published，管理员见本校全部 |
| POST | `/questions` | 教师 | 创建草稿，`creator_id` 强制为当前用户，学科须与教师一致；富文本白名单消毒 |
| POST | `/questions/resolve` | 教师/管理员 | 按 ID 数组批量返回本校已发布题目，供购物车恢复校验 |
| GET | `/questions/:id` | 教师/管理员 | 按 ID + 学校过滤 |
| PATCH | `/questions/:id` | 教师（本人草稿） | 更新草稿 |
| DELETE | `/questions/:id` | 教师（本人草稿） | 软删除 |
| PATCH | `/admin/questions/:id/status` | 管理员 | 仅允许 draft→published、draft/published→archived |

题型：`single_choice`、`multiple_choice`、`true_false`、`fill_blank`、`essay`。状态：`draft`、`published`、`archived`。

## OCR

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | `/ocr/recognize` | 教师/管理员 | multipart 字段 `image`，PNG/JPEG/WebP，最大 5 MB、2000 万像素；返回 `{"text":"已知 $x^2=4$"}`，不写数据库。上游失败映射 502/503/504 |

OCR 结果必须由教师校对后通过正常题目接口保存，原图不持久化。

## CSV 导入

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | `/questions/import/validate` | 教师 | 只解析校验，返回有效行数、最多 50 行预览与带真实行号的错误，不写库 |
| POST | `/questions/import` | 教师 | 重新读取原文件并完整校验，单事务导入当前学校、当前教师的草稿；任一行失败返回 422 且零写入 |

限制：5 MB、1,000 数据行、固定中文表头、学科与当前教师一致、难度 1–5、选择题校验选项与答案。

## 试卷

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/papers` | 教师（本人）/管理员（本校） | 分页列表 |
| POST | `/papers` | 教师 | 事务内逐题校验学校与 published 状态、学科年级一致、题目不重复，替换明细并汇总总分 |
| GET | `/papers/:id` | 教师（本人）/管理员（本校） | 返回卷面信息与按 `order_index` 排序的题目明细 |
| PATCH | `/papers/:id` | 教师（本人）/管理员（本校） | 同一事务更新 |
| DELETE | `/papers/:id` | 教师（本人）/管理员（本校） | 软删除 |

分值为两位小数字符串，数据库 `NUMERIC(8,2)`；后端定点复核总分，前端使用整数百分计算（0.10 + 0.20 = 0.30）。

## 管理与统计

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/admin/teachers` | 管理员 | 按学科、状态筛选教师 |
| PATCH | `/admin/teachers/:id/status` | 管理员 | 启用/禁用，禁用立即使旧会话失效 |
| POST | `/admin/teachers/:id/reset-password` | 管理员 | 生成至少 16 位随机临时密码并递增会话版本，仅响应一次 |
| GET | `/admin/school` | 管理员 | 学校名称与只读学校代码 |
| PATCH | `/admin/school` | 管理员 | 修改学校名称 |
| GET | `/admin/stats` | 管理员 | 本校教师、题目、试卷统计 |
| GET | `/teacher/stats` | 教师 | 当前教师题目、试卷统计 |

## 健康检查

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/healthz` | 进程存活，返回 `{"status":"ok"}` |
| GET | `/readyz` | 数据库未就绪返回 503 |

OCR sidecar 独立提供 `/healthz`、`/readyz`（模型加载完成才返回 200）与 `POST /v1/recognize`。
