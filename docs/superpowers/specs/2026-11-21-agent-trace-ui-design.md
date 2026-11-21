# Agent Loop 执行轨迹前端设计（2026-11-21）

## 目标
让 AI 助手页面从「像普通聊天」升级为「Agent 工作台」：运行中一眼看清 AI 在做什么，完成后过程可回溯，整体符合 Yohaku 暖纸设计语言。

## 范围
仅呈现层小规模重构，不改动 SSE 协议、数据模型（`ToolCall`/`TimelineEntry`/`LiveTurn`）与三栏骨架。

## 设计

### 1. Agent 执行轨迹（`agent-trace.tsx`，新增）
- **折叠头（始终可见）**：运行中显示脉冲点 + `processSummary` 摘要 + 当前动作（「正在调用『检索题库』」/「正在思考…」）+ 已完成步数；完成后显示绿色勾选 + 「运行了 X 秒，调用 N 次工具」。
- **轨迹主体**：按 timeline 真实顺序渲染。
  - 工具步骤：`01` 编号 + 状态圆点（运行中带扩散脉冲环、成功绿、失败红、跳过黄）+ 纵向连接线 + 中文工具名 + 关键参数 chips（学科/年级/关键词/知识点/题量/题目短 ID，最多 3 个）+ 状态徽标 + 耗时 + 摘要；失败展示错误，警告逐条列出；保留「涉及题目（复制 ID / 打开题库）」「试卷草稿（打开试卷）」与「参数与返回 JSON」展开。
  - 思路条目：浅色左边线卡片 + `思路` 标签，长文本限高滚动，默认完整呈现。
- **动效**：步骤 `motion-rise` 依次淡入（40ms 步进）；运行中圆点 `trace-pulse` 扩散；完成勾选 `motion-pop`。尊重全局 `prefers-reduced-motion`。

### 2. 空状态（`message-list.tsx`）
- Hero：徽标 + 「AI 题库工作台」+ 一句说明（录题、体检、组卷、资料检索）。
- 技能按分类分组：文档与资料 / 题库整理 / 试卷生成 / 数据分析 / 产出复核，每组标题带图标与数量，卡片双列。

### 3. 技能面板（`skills-panel.tsx`）
- 技能 tab 增加分类筛选 chips（全部/文档/题库/试卷/分析/复核），过滤展示；工具 tab 不变。

### 4. 共享
- 扩展 skill icon 映射至全部 13 种 icon 值，避免统一回退魔法棒。
- `tool-call-timeline.tsx` 的富交互迁入 `agent-trace.tsx`，原文件删除；`message-bubble.tsx` 改由 `AgentTrace` 渲染过程。

## 验证
- `pnpm typecheck`、`pnpm lint`、`pnpm vitest run lib/ai-*.test.ts`。