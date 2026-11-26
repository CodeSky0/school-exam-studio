# Web UI 与动效优化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变业务、权限、数据和 A4 打印契约的前提下，完成暖纸感与深玫瑰视觉系统、全站响应式、核心 UX 和 Framer Motion 交互优化。

**Architecture:** 保留 Next.js Server Component 页面与现有 Client Component 数据边界，在 Providers 注入 MotionConfig，并把动画限制在列表、购物车、对话框和移动导航等叶子组件。桌面表格与移动摘要共用同一份真实 Query 数据；原生 dialog 继续承担模态焦点和背景隔离。

**Tech Stack:** Next.js 16、React 19、TypeScript、Tailwind CSS v4、Radix Icons、TanStack Query、Zustand、Framer Motion、Vitest、Playwright。

## Global Constraints

- 包管理器固定为 `pnpm@10.22.0`，Node.js 为 `>=22.12.0`。
- 仅新增 `framer-motion` 一个运行时依赖，图标只能使用 `@radix-ui/react-icons`。
- 浅色页面底色为 `#f7f5f2`，主文字为 `#292624`，accent 为 `#9f4256`，accent hover 为 `#87384a`。
- 所有界面文案保持简体中文，不改变 API、权限、TanStack Query、Zustand、TipTap、OCR 和保存顺序。
- 页面 wrapper 保持 Server Components；动画放入 Client Component 叶子节点。
- 动画只使用 transform 和 opacity，并通过 `MotionConfig reducedMotion="user"` 与 CSS 降级。
- 不使用假数据、静态业务数字、紫蓝霓虹、外发光、居中超大 Hero 或通用三列营销卡片。
- A4 打印尺寸、分页、页边距和 PDF 输出契约不变；屏幕预览覆盖不得进入打印媒体规则。
- 不提交 `node_modules`、`.next`、测试产物、本地环境文件或任何凭据。

---

## 文件结构

- `frontend/app/layout.tsx`：Geist 与 Geist Mono 字体变量。
- `frontend/app/globals.css`：语义颜色、字体、焦点、触控和 reduced-motion 基础。
- `frontend/app/print.css`：独立的屏幕 A4 浏览规则。
- `frontend/components/motion-presets.ts`：共享 variants 与 spring。
- `frontend/components/page-header.tsx`：后台页面标题与操作区。
- `frontend/components/providers.tsx`：QueryClientProvider 与 MotionConfig。
- `frontend/components/ui/animated-dialog.tsx`：原生模态、滚动锁与退出生命周期。
- `frontend/components/ui/status-message.tsx`：成功、信息、警告和错误反馈。
- `frontend/components/mobile-navigation.tsx`：移动模态抽屉与焦点归还。
- `frontend/components/home-paper-preview.tsx`：首页装饰性卷面动画叶子。
- `frontend/components/question/admin-creator-select.tsx`：真实教师与当前管理员选项。
- `frontend/lib/admin.ts`：全分页教师聚合。
- 现有首页、认证、应用壳、题库、组卷、试卷列表、教师管理、仪表盘和个人中心组件：只调整呈现与交互，保留公共 props 和业务调用。

---

### Task 1: 设计令牌、字体、动画基座与基础控件

**Files:**
- Modify: `frontend/package.json`
- Modify: `frontend/pnpm-lock.yaml`
- Modify: `frontend/app/layout.tsx`
- Modify: `frontend/app/globals.css`
- Modify: `frontend/components/providers.tsx`
- Modify: `frontend/components/ui/button.tsx`
- Modify: `frontend/components/ui/input.tsx`
- Modify: `frontend/components/ui/textarea.tsx`
- Modify: `frontend/components/ui/select.tsx`
- Create: `frontend/components/motion-presets.ts`
- Create: `frontend/components/page-header.tsx`
- Create: `frontend/components/ui/status-message.tsx`

**Interfaces:**
- Produces: `PageHeader({ eyebrow, title, description, actions, back, className })`。
- Produces: `StatusMessage({ message, tone, className })`，error 使用 `role="alert"`，其余使用 `role="status"`。
- Produces: `listItemVariants`、`listContainerVariants`、`cartSpring`，供题库和组卷复用。

- [x] **Step 1: 安装已确认的唯一新依赖**

Run:

```bash
pnpm add framer-motion
```

Expected: `package.json` 只新增 `framer-motion`，锁文件由 pnpm 更新。

- [x] **Step 2: 接入字体与 MotionConfig**

`app/layout.tsx` 使用以下字体定义，并把变量放到 `html` 的 `className`：

```tsx
import { Geist, Geist_Mono } from "next/font/google";

const geist = Geist({ subsets: ["latin"], variable: "--font-geist", display: "swap" });
const geistMono = Geist_Mono({ subsets: ["latin"], variable: "--font-geist-mono", display: "swap" });
```

`providers.tsx` 保持现有 QueryClient，返回值改为：

```tsx
return (
  <QueryClientProvider client={queryClient}>
    <MotionConfig reducedMotion="user">{children}</MotionConfig>
  </QueryClientProvider>
);
```

- [x] **Step 3: 替换设计令牌与基础排版**

`globals.css` 的浅色核心值设为：

```css
:root {
  --surface-paper: #f7f5f2;
  --surface-desk: #eeeae5;
  --n1: #fdfcf9;
  --n9: #292624;
  --a: #9f4256;
  --a-strong: #87384a;
  --a-soft: #f5e9eb;
  --success: #3f6b54;
  --warning: #80551f;
  --error: #963f4d;
}
```

字体令牌使用 `var(--font-geist)`、`var(--font-geist-mono)` 与中文系统无衬线回退；工作台 `h1–h4` 使用 sans，`--font-serif` 仅供卷面。移除 body 的放射渐变，保留暖色背景、清晰分隔线和柔和阴影。增加 `:focus-visible` 统一焦点环，以及 coarse pointer 下至少 44px 的按钮目标。

- [x] **Step 4: 创建共享动画和页面原语**

`motion-presets.ts` 导出：

```ts
import type { Transition, Variants } from "framer-motion";

export const cartSpring: Transition = { type: "spring", stiffness: 180, damping: 24, mass: 0.8 };
export const listContainerVariants: Variants = {
  hidden: {},
  visible: { transition: { staggerChildren: 0.035 } },
};
export const listItemVariants: Variants = {
  hidden: { opacity: 0, y: 8 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.2, ease: [0.16, 1, 0.3, 1] } },
  exit: { opacity: 0, x: -10, transition: { duration: 0.16 } },
};
```

`PageHeader` 接收 `eyebrow: string`、`title: string`、`description?: string`、`actions?: ReactNode`、`back?: ReactNode`、`className?: string`，输出左标题右操作、移动单列的 `header`。`StatusMessage` 用 `AnimatePresence mode="wait"` 和 `motion.p` 呈现非空消息，稳定 key 为 `${tone}:${message}`。

- [x] **Step 5: 统一基础控件并运行静态检查**

保留 Button CVA、asChild 与所有表单原生 props。主按钮使用 `bg-accent text-accent-foreground hover:bg-accent-strong`，active 使用 `scale-[0.98]`，disabled 保留明确透明度；输入框使用 `bg-neutral-1`、清晰边框和 accent 焦点环。

Run:

```bash
pnpm lint && pnpm typecheck && pnpm build
```

Expected: 全部通过；若字体下载受网络阻塞，记录错误并改用随 Next.js 构建可访问的字体方案，不保留 Inter。

- [x] **Step 6: Commit**

```bash
git add frontend/package.json frontend/pnpm-lock.yaml frontend/app/layout.tsx frontend/app/globals.css frontend/components/providers.tsx frontend/components/motion-presets.ts frontend/components/page-header.tsx frontend/components/ui
git commit -m "feat(web): 建立暖纸感视觉与动画基础"
```

---

### Task 2: 模态基座、移动导航与键盘可访问性

**Files:**
- Create: `frontend/components/ui/animated-dialog.tsx`
- Modify: `frontend/components/ui/dialog.tsx`
- Modify: `frontend/components/ui/confirm-dialog.tsx`
- Modify: `frontend/components/app-shell.tsx`
- Create: `frontend/components/mobile-navigation.tsx`
- Test: `frontend/tests/e2e/ui-motion.spec.ts`
- Test: `frontend/tests/e2e/admin.spec.ts`

**Interfaces:**
- Produces: `AnimatedDialog({ children, onClose, isPending, labelledBy, describedBy, className })`。
- Produces: `MobileNavigation({ user, open, onClose, triggerRef })`。
- Consumes: 现有 `AppShell({ user, children })`、Dialog 与 ConfirmDialog 公共 props 全部不变。

- [x] **Step 1: 先写移动导航与唯一 ID 的失败用例**

`ui-motion.spec.ts` 使用 390×844 viewport 和已有管理员 storageState，断言：

```ts
await page.setViewportSize({ width: 390, height: 844 });
await page.goto("/admin/dashboard");
await page.getByRole("button", { name: "打开导航" }).click();
await expect(page.getByRole("dialog", { name: "主导航" })).toBeVisible();
await expect(page.getByRole("button", { name: "关闭导航" })).toBeFocused();
await page.keyboard.press("Escape");
await expect(page.getByRole("button", { name: "打开导航" })).toBeFocused();
```

再断言首次 Tab 显示“跳到主内容”，Enter 后 `#main-content` 聚焦。`admin.spec.ts` 增加两个 ConfirmDialog 的 `aria-labelledby` 各自指向本对话框标题，且文档 ID 不重复。

- [x] **Step 2: 运行用例并确认失败原因**

Run:

```bash
pnpm exec playwright test tests/e2e/ui-motion.spec.ts tests/e2e/admin.spec.ts
```

Expected: 移动 dialog、焦点归还、skip link 或唯一 ID 断言失败；若服务环境不可用，保留用例并明确记录阻塞，不删除断言。

- [x] **Step 3: 实现 AnimatedDialog**

组件在 mount effect 中执行 `dialog.showModal()`，cleanup 中关闭 dialog 并释放滚动锁；`onCancel` 始终 `preventDefault()`，仅非 pending 时调用 `onClose`。父组件使用：

```tsx
<AnimatePresence>
  {open ? (
    <AnimatedDialog
      key="dialog"
      onClose={onClose}
      isPending={isPending}
      labelledBy={titleId}
      describedBy={description ? descriptionId : undefined}
      className={className}
    >
      {children}
    </AnimatedDialog>
  ) : null}
</AnimatePresence>
```

`AnimatedDialog` 内层 `motion.div` 使用 `initial={{ opacity: 0, y: 10, scale: 0.985 }}`、`animate={{ opacity: 1, y: 0, scale: 1 }}`、`exit={{ opacity: 0, y: 6, scale: 0.99 }}`，时长 0.2。原生 dialog 在退出完成前保持 mounted，因此保留旧标题、描述和临时密码。

- [x] **Step 4: 修复 Dialog 与 ConfirmDialog**

两者改用 AnimatedDialog；ConfirmDialog 增加：

```tsx
const titleId = useId();
const descriptionId = useId();
```

并把 `aria-labelledby`、`aria-describedby`、标题和描述绑定到这两个唯一值。pending 时 Escape、取消和确认继续禁用。

- [x] **Step 5: 实现移动导航与应用壳**

MobileNavigation 使用原生 `dialog.showModal()`、全视口透明 dialog 和 `motion.div` 侧栏；mount 后聚焦关闭按钮，关闭退出完成后 cleanup 归还 trigger 焦点。监听 `(min-width: 1024px)`，进入桌面时调用 `onClose`。AppShell 添加：

```tsx
<a href="#main-content" className="skip-link">跳到主内容</a>
<main id="main-content" tabIndex={-1} className="app-shell-main mx-auto w-full max-w-[1560px] p-5 sm:p-8 lg:p-10">{children}</main>
```

菜单按钮增加 `ref`、`aria-controls="mobile-navigation"` 和现有 `aria-expanded`；桌面侧栏继续使用原角色导航配置。

- [x] **Step 6: 验证并 Commit**

Run:

```bash
pnpm lint && pnpm typecheck && pnpm exec playwright test tests/e2e/ui-motion.spec.ts tests/e2e/admin.spec.ts
git add frontend/components/ui frontend/components/app-shell.tsx frontend/components/mobile-navigation.tsx frontend/tests/e2e
git commit -m "feat(web): 完善模态导航与键盘交互"
```

Expected: 静态检查通过；可运行环境中新增用例通过，Escape、遮罩、关闭按钮和 resize 均恢复滚动与焦点。

---

### Task 3: 首页、认证页、仪表盘与个人中心视觉重构

**Files:**
- Modify: `frontend/components/home.tsx`
- Create: `frontend/components/home-paper-preview.tsx`
- Modify: `frontend/components/brand.tsx`
- Modify: `frontend/components/auth/auth-layout.tsx`
- Modify: `frontend/components/auth-form.tsx`
- Modify: `frontend/components/auth/login-form.tsx`
- Modify: `frontend/components/auth/register-form.tsx`
- Modify: `frontend/components/auth/init-panel.tsx`
- Modify: `frontend/components/stats-cards.tsx`
- Modify: `frontend/components/admin/admin-dashboard.tsx`
- Modify: `frontend/components/profile/teacher-dashboard.tsx`
- Modify: `frontend/components/admin/school-settings.tsx`
- Modify: `frontend/components/profile/profile-content.tsx`
- Modify: `frontend/components/loading-screen.tsx`
- Test: `frontend/tests/e2e/ui-motion.spec.ts`
- Test: `frontend/tests/e2e/auth.spec.ts`

**Interfaces:**
- Consumes: Task 1 的 PageHeader、StatusMessage 和 variants。
- Preserves: `GuestHome({ error, retry })`、`ProfileContent({ user })`、StatsCards 数据 props 和所有认证 mutation。

- [x] **Step 1: 添加首页与认证响应式失败用例**

在 `ui-motion.spec.ts` 分别以 1440×1000 和 390×844 打开 `/`，断言主标题、登录、注册、初始化入口和装饰卷面存在，并执行：

```ts
const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);
expect(overflow).toBe(false);
```

认证用例断言每页只有一个 `h1`，所有输入有可见 label，reduced-motion 下登录表单仍可提交。

- [x] **Step 2: 实现首页非对称首屏**

首页使用 `lg:grid-cols-[minmax(0,1.05fr)_minmax(360px,0.95fr)]`，左栏依次放 eyebrow、控制尺寸的 `h1`、真实说明、登录/注册/初始化操作；右栏渲染 HomePaperPreview。预览只含“校本试卷”“题目”“解析”等抽象卷面线框，`aria-hidden="true"`，不显示伪造数量或学校名称。动画叶子使用 0.2 秒 opacity/y 进入，无持续漂浮。

- [x] **Step 3: 统一认证状态过渡**

AuthLayout 品牌侧标题改为 `p` 或 `h2`，表单 AuthHeading 输出唯一 `h1`。登录仅显示真实 pending/error；注册与初始化用 `AnimatePresence mode="wait"` 切换表单和真实成功结果，key 分别为 `form` 与 `success`。错误使用 FormError/StatusMessage，不清空退出动画中的学校代码。

- [x] **Step 4: 重构真实统计与个人中心**

StatsCards 保留 `StatCardItem[]`，改为一个容器内的响应式 `grid divide-*` 数据带，数字使用 Geist Mono，不给非交互统计加 misleading hover。管理员与教师仪表盘使用 PageHeader 和真实快捷入口；个人中心桌面双栏、移动单栏，学校设置与密码表单继承统一状态消息和骨架。

- [x] **Step 5: 浏览器检查并 Commit**

Run:

```bash
pnpm dev
```

在浏览器检查 `/`、`/login`、`/register`、`/init`、两个仪表盘、学校设置、`/account` 和 `/teacher/profile` 的桌面、390px、深色与 reduced-motion。随后运行：

```bash
pnpm lint && pnpm typecheck && pnpm exec playwright test tests/e2e/auth.spec.ts tests/e2e/ui-motion.spec.ts
git add frontend/components frontend/tests/e2e
git commit -m "feat(web): 重构公共页面与工作台视觉"
```

Expected: 无横向溢出、无重复 h1、真实会话与统计请求不变，认证和设置回归通过。

---

### Task 4: 管理员创建人姓名筛选与完整分页聚合

**Files:**
- Modify: `frontend/lib/admin.ts`
- Modify: `frontend/lib/admin.test.ts`
- Create: `frontend/components/question/admin-creator-select.tsx`
- Modify: `frontend/components/question/filter-bar.tsx`
- Modify: `frontend/components/question/question-bank.tsx`
- Modify: `frontend/components/admin/admin-question-bank.tsx`
- Test: `frontend/tests/e2e/question.spec.ts`

**Interfaces:**
- Produces: `listAllTeachers(): Promise<CurrentUser[]>`。
- Produces: `adminQueryKeys.teacherOptions`。
- Produces: `AdminCreatorSelect({ user, value, onChange })`，value 与 onChange 均为 UUID 字符串；空字符串表示全部创建人。

- [x] **Step 1: 写分页聚合失败测试**

在 `admin.test.ts` 添加 total=101 的两页响应，第二页含一个跨页重复 ID，并断言：

```ts
await expect(listAllTeachers()).resolves.toHaveLength(100);
expect(fetchMock.mock.calls.map((call) => new URL(call[0] as string).searchParams.get("page"))).toEqual(["1", "2"]);
expect(fetchMock.mock.calls.every((call) => new URL(call[0] as string).searchParams.get("page_size") === "100")).toBe(true);
```

另写第二页返回 500 时 Promise reject、total=0 时只请求第一页的测试。

- [x] **Step 2: 运行测试并确认函数不存在**

Run:

```bash
pnpm test -- lib/admin.test.ts
```

Expected: 因 `listAllTeachers` 未导出而失败。

- [x] **Step 3: 实现最小聚合函数**

```ts
export async function listAllTeachers() {
  const first = await listTeachers({ page: 1, page_size: 100 });
  const pages = [first];
  for (let page = 2; page <= Math.ceil(first.total / 100); page += 1) {
    pages.push(await listTeachers({ page, page_size: 100 }));
  }
  return [...new Map(pages.flatMap((item) => item.items).map((teacher) => [teacher.id, teacher])).values()];
}
```

增加 `teacherOptions: ["admin", "teacher-options"] as const`，不改 `listTeachers` 和现有筛选序列化。

- [x] **Step 4: 实现 AdminCreatorSelect 并同步筛选 draft**

AdminCreatorSelect 用 `useQuery({ queryKey: adminQueryKeys.teacherOptions, queryFn: listAllTeachers })`，选项顺序为“全部创建人”、当前管理员、全部 active/disabled 教师；重复姓名用 `${name} · ${subject || "未设置学科"} · ${email}` 区分。失败时显示 FormError 与“重新读取”按钮，其他筛选保持可用。

QuestionFilterBar 在 `filters` 改变时 `setDraft(filters)`；reset 同时更新 draft 和 applied filters。两个题库父组件删除 `key={JSON.stringify(filters)}`，不改 `creator_id` 序列化。

- [x] **Step 5: 验证并 Commit**

Run:

```bash
pnpm test -- lib/admin.test.ts lib/questions.test.ts && pnpm lint && pnpm typecheck
pnpm exec playwright test tests/e2e/question.spec.ts
git add frontend/lib/admin.ts frontend/lib/admin.test.ts frontend/components/question frontend/components/admin/admin-question-bank.tsx frontend/tests/e2e/question.spec.ts
git commit -m "feat(web): 用教师姓名简化创建人筛选"
```

Expected: 聚合、去重、失败和原 UUID 请求契约测试通过；Playwright 能按教师姓名筛选并观察到对应 `creator_id` 请求。

---

### Task 5: 题库、试卷与教师列表的移动记录和布局动画

**Files:**
- Modify: `frontend/components/question/question-table.tsx`
- Modify: `frontend/components/question/question-bank.tsx`
- Modify: `frontend/components/admin/admin-question-bank.tsx`
- Modify: `frontend/components/paper/paper-list.tsx`
- Modify: `frontend/components/admin/teacher-management.tsx`
- Test: `frontend/tests/e2e/question.spec.ts`
- Test: `frontend/tests/e2e/paper.spec.ts`
- Test: `frontend/tests/e2e/admin.spec.ts`

**Interfaces:**
- Preserves: QuestionTable 全部 props、`renderActions`、canEdit、分页与删除回调。
- Preserves: PaperSummary 列表、删除回退、教师状态与重置密码 mutations。
- Consumes: Task 1 variants、StatusMessage；Task 2 ConfirmDialog；Task 4 draft 同步方式。

- [x] **Step 1: 添加移动记录与焦点失败用例**

三类测试均设置 390×844，断言移动摘要主要操作可见且根节点无横向溢出；桌面仍通过 `getByRole("row")` 读取原表格。题目测试额外断言展开后答案/解析可见且焦点留在展开按钮，筛选后应用按钮焦点不丢失。教师测试分别按 dialog 名称打开禁用和重置确认框。

- [x] **Step 2: 实现题库双呈现与详情动画**

桌面容器使用 `hidden md:block`，移动容器使用 `md:hidden`，确保 CSS 隐藏记录不进入可访问树。移动记录由同一 `data.items` map 渲染，包含题干摘要、学科/年级、题型、难度、标签、创建人、状态、更新时间及原权限操作。展开内容复用 QuestionDetails：

```tsx
<AnimatePresence initial={false}>
  {expanded ? (
    <motion.div key="details" initial={{ opacity: 0, y: -6 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -4 }}>
      <QuestionDetails question={question} />
    </motion.div>
  ) : null}
</AnimatePresence>
```

桌面不为表格插入非法 div；行动画放在合法单元格内容或移动 `motion.article`。所有 key 只用 `question.id`。

- [x] **Step 3: 实现试卷和教师移动记录**

PaperList 移动记录直接显示编辑、打印、删除，次要 UUID 放展开区；TeacherManagement 移动记录直接显示启禁用和重置密码，保留完整邮箱、学科、状态、创建时间和 ID。两者用 `motion.article`、listItemVariants 和稳定 ID，不给整个列表设置 filters/page key。

- [x] **Step 4: 移除筛选重挂载并统一状态反馈**

TeacherFilterBar 在 applied filters 改变时同步 draft，reset 同时 `setDraft(next)` 与 `onApply(next)`，父组件删除 JSON key。三类列表用 StatusMessage 替换静态成功提示；保留原加载、错误、空状态和分页回退逻辑。

- [x] **Step 5: 验证并 Commit**

Run:

```bash
pnpm lint && pnpm typecheck
pnpm exec playwright test tests/e2e/question.spec.ts tests/e2e/paper.spec.ts tests/e2e/admin.spec.ts
git add frontend/components/question frontend/components/admin frontend/components/paper/paper-list.tsx frontend/tests/e2e
git commit -m "feat(web): 优化核心列表移动体验与动效"
```

Expected: 桌面旧流程兼容，390px 主要操作无需横滑，筛选焦点稳定，两个确认框名称和 ID 正确。

---

### Task 6: 组卷购物车动画、排序焦点与 A4 屏幕预览

**Files:**
- Modify: `frontend/components/paper/paper-composer.tsx`
- Modify: `frontend/components/paper/question-picker.tsx`
- Modify: `frontend/components/paper/paper-cart.tsx`
- Modify: `frontend/components/paper/paper-form.tsx`
- Modify: `frontend/components/paper/paper-store-provider.tsx`
- Modify: `frontend/components/paper/paper-print.tsx`
- Modify: `frontend/app/print.css`
- Test: `frontend/tests/e2e/paper.spec.ts`
- Test: `frontend/tests/e2e/print.spec.ts`

**Interfaces:**
- Preserves: `usePaperStore` selectors、add/remove/setScore/move/reorder、HTML5 drag、保存与恢复顺序。
- Consumes: `cartSpring`、`listItemVariants`、StatusMessage、PageHeader。

- [x] **Step 1: 添加购物车与预览失败用例**

paper.spec 添加：添加两题后上移/下移，按钮保持焦点且 DOM 顺序正确；删除后焦点落在下一题、上一题或 `#paper-cart-title`；reduced-motion 下操作一致。print.spec 增加：

```ts
const ratio = await page.locator(".paper-sheet").evaluate((element) => {
  const rect = element.getBoundingClientRect();
  return rect.width / rect.height;
});
expect(ratio).toBeCloseTo(210 / 297, 1);
```

390px 下断言根节点不溢出，`.paper-preview-stage` 可横向滚动，原 A4 PDF MediaBox 断言保留。

- [x] **Step 2: 为购物车加入布局动画**

`ol` 内使用 `AnimatePresence initial={false}` 和 `motion.li`：

```tsx
<motion.li
  key={question.id}
  layout="position"
  variants={listItemVariants}
  initial="hidden"
  animate="visible"
  exit="exit"
  transition={cartSpring}
  data-question-id={question.id}
>
```

保留原 drag events，不添加 Framer drag。move 后通过 `data-question-id` 找同一记录的原操作按钮并聚焦；remove 前计算相邻 ID，退出后聚焦相邻记录或购物车标题。批量恢复失效题目不主动抢焦点。增加 `aria-live="polite"` 的中文排序消息。

- [x] **Step 3: 优化选题、表单与真实状态**

QuestionPicker 保留 keepPreviousData，对已有 li 使用稳定 question ID 和 stagger；把仅 sr-only 的添加消息改为可见 StatusMessage，同时保留播报。PaperForm 只整理间距、错误与 pending；PaperStoreProvider 骨架匹配双栏工作区；保存、加载、失效和 onSaved 顺序不改。

- [x] **Step 4: 隔离屏幕 A4 规则**

将屏幕预览覆盖放入：

```css
@media screen {
  .paper-preview-stage {
    max-width: 100%;
    overflow-x: auto;
    overscroll-behavior-inline: contain;
  }

  .paper-preview-stage .paper-sheet {
    width: 210mm;
    max-width: none;
    margin: 0;
    flex: none;
  }
}
```

PaperPrint 外层复用 `.paper-preview-stage`。不得修改 `@media print` 中 `.paper-sheet` 的复位、A4 page、分页或 UI 隐藏规则。

- [x] **Step 5: 验证并 Commit**

Run:

```bash
pnpm test -- stores/paper-store.test.ts && pnpm lint && pnpm typecheck
pnpm exec playwright test tests/e2e/paper.spec.ts tests/e2e/print.spec.ts
git add frontend/components/paper frontend/app/print.css frontend/tests/e2e/paper.spec.ts frontend/tests/e2e/print.spec.ts
git commit -m "feat(web): 增强组卷反馈与纸张预览"
```

Expected: 购物车原单元测试和 E2E 通过，焦点与顺序可追踪，屏幕纸张不压缩，PDF 仍为 A4。

---

### Task 7: 全量回归、视觉验收与交付

**Files:**
- Modify only when a regression is found: affected frontend source or test files
- Update: `docs/superpowers/plans/2026-11-14-web-ui-motion.md` 的完成勾选

**Interfaces:**
- Consumes: 前六个任务的全部实现。
- Produces: 可复现的验证结果、最终提交和 GitHub main 推送。

- [x] **Step 1: 运行全部前端静态与单元检查**

Run:

```bash
pnpm lint && pnpm typecheck && pnpm test && pnpm build
```

Expected: 全部通过；任何本次引入的错误修复后重跑同一组命令。

- [x] **Step 2: 运行完整 E2E**

Run:

```bash
pnpm test:e2e
```

Expected: 初始化、注册登录、题库、审核、组卷、教师管理、A4 打印和新增 UI 用例全部通过。若 Docker、Go、PostgreSQL 或浏览器二进制阻塞，记录具体命令与错误，不把未执行描述为通过。

- [x] **Step 3: 做最终浏览器视觉检查**

覆盖 1440px、768px、390px，检查首页、认证、管理员和教师工作台；验证深色、reduced-motion、键盘导航、加载/空/错误状态、移动列表操作、购物车重排和屏幕/打印预览。不得通过隐藏内容或缩短业务断言消除失败。

- [x] **Step 4: 检查最终差异与敏感信息**

Run:

```bash
git status --short
git diff --check
git diff --stat
git log --oneline origin/main..HEAD
```

Expected: 无空白错误、无依赖/构建/测试产物、无 Token 或认证头；所有计划项已勾选或有明确阻塞说明。

- [x] **Step 5: Commit 并推送**

如有回归修复或计划状态更新：

```bash
git add frontend docs/superpowers/plans/2026-11-14-web-ui-motion.md
git commit -m "test(web): 完成全站视觉与交互回归"
```

推送：

```bash
git push origin main
```

Expected: GitHub `CodeSky0/school-exam-studio` 的 main 包含设计文档和全部已验证实施提交；最终回复列出主要改动、验证结果、提交范围与任何未解决阻塞。

---

## 完成记录（2026-11-26）

- 补齐 `tests/e2e/ui-motion.spec.ts`：移动导航焦点归还、跳到主内容链接、首页 1440/390 无横向溢出、登录/注册单 h1 与可见标签、已初始化 `/init` 跳转、reduced-motion 登录可提交。
- `admin.spec.ts` 补充确认框 `aria-labelledby` 唯一性与文档 ID 去重、390px 教师摘要操作；`question.spec.ts` 补充 390px 摘要展开答案/解析与筛选焦点稳定；`paper.spec.ts` 补充购物车上移/下移/删除的焦点与 DOM 顺序（含 reduced-motion）；`print.spec.ts` 补充屏幕 A4 比例与 390px 舞台横滚。
- 全部 24 项 E2E 在真实 PostgreSQL 16 + Redis 7 + Chromium 环境通过；静态检查与生产构建通过。
