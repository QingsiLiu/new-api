# Goal: 操作栏全面统一 + 固定列透视修复 + 表格视觉统一收尾

仓库：`/Users/tedliu/code/new-api`（只推 `QingsiLiu/new-api` main，bun 构建，部署走 `ssh ctbuk-443`）

## 确认的症状（用户反馈）

1. 操作列太宽 / 右侧留白
2. 操作列内容被裁切/遮挡
3. 操作列与表头错位
4. **行 hover 时固定操作列下方内容透出**（半透明 bug）
5. 整体风格不统一

## 根因定位

| 症状 | 根因 | 文件 |
|------|------|------|
| hover 透出 | 固定列 hover 背景是 `bg-muted/50` **半透明**，sticky 列下面滚过的内容直接透出来 | `components/data-table/core/column-pinning.ts:64` |
| 列宽不一 | 各表 size 随意：keys=112、channels=140、users=100、models=100、redemptions=88、subscriptions=80、deployments=**重复 key（120 和 180 各写一次，typecheck 已红）** | 各 `*-columns.tsx` |
| 表头错位 | 有的 cell 用 `-ml-2`（users/models/redemptions/subscriptions），有的不用（keys/channels）；有的 `justify-end`（keys），有的左对齐（channels）；只有 channels 给 actions 列配了 `px-2` | 各 row-actions + `channels-table.tsx:402` |
| 裁切/遮挡 | deployments 一行塞 7 个图标按钮（180px 也放不下）| `models/components/deployments-columns.tsx:233-303` |
| 风格不统一 | `mobile-card-list.tsx` 还在用 `radius-surface`，与 2026-07-06 已定的「顶层容器 24px 无边框」冲突 | `components/data-table/layout/mobile-card-list.tsx` |

## 统一规范（本次落地）

**操作栏模式：常用操作 + ⋯ 溢出菜单**（用户已确认）

- cell 结构统一：`<div className='-ml-2 flex items-center gap-1'>`（去掉 keys 的 `justify-end`、channels 的 `min-w-max` 无 `-ml-2`）
- actions 列统一 `px-2`（header+cell 都要，目前只有 channels 有）→ `-ml-2` 后首个图标与表头文字左缘对齐
- 列宽按控件数：**仅菜单=64px；2 控件=88px；3 控件=120px**
- 列定义统一带：`enableSorting: false, enableHiding: false, meta: { pinned: 'right' }`

### 各表落位

| 表 | 现状 | 目标 |
|----|------|------|
| keys | 开关+菜单, 112, justify-end | 2 控件 → **88**，左对齐 |
| channels | 测试+开关+菜单, 140 | 3 控件 → **120** |
| users | 仅菜单, 100 | **64** |
| models | 仅菜单, 100 | **64** |
| redemptions | 仅菜单, 88 | **64** |
| subscriptions | 仅菜单, 80 | **64** |
| deployments | 7 个平铺按钮 + 重复 key | 保留「查看日志」+ ⋯ 菜单（详情/改配置/续期/改名/删除 收进菜单）→ 2 控件 **88**；删除重复的 `size:180/meta` |

## 改动清单

1. **`core/column-pinning.ts`** — hover/选中背景改不透明（修透视 bug）：
   ```
   group-hover:[background-color:color-mix(in_oklch,var(--muted)_50%,var(--card))]
   group-data-[state=selected]:bg-muted 保持（已不透明）
   ```
2. **`models/components/deployments-columns.tsx`** — 删除重复的 `size`/`meta` key（修 typecheck 红）；actions cell 重构为「Eye 日志 + DropdownMenu(详情/配置/续期/改名/删除)」，`size: 88`
3. 7 张表的 columns/row-actions 按上表统一 size + cell 结构
4. 各表 `getColumnClassName` 给 actions 补 `px-2`（keys/users/models/redemptions/subscriptions/deployments；channels 已有）
5. **`layout/mobile-card-list.tsx`** — 4 处 `radius-surface` → `radius-card`，对齐 2026-07-06 表格统一决议（嵌套 dialog/设置页内的 StaticDataTable 保持 surface，不动）

## 验证

- `bun run typecheck`（当前就是红的，改完必须绿）
- `bun run build`
- `bun run lint`（只看新增告警）

## 提交 & 部署

- commit 到 `QingsiLiu/new-api` main：`fix(web): unify table actions column + opaque pinned-column hover`
- 部署（沿用既有流程）：rsync → ctbuk-443 docker build → `docker compose up -d relay-new-api` → 验证 `Up (healthy)`

## 范围外（本次不做，单独列账）

全仓还有 ~24 个文件用 `rounded-lg/xl`、`shadow-md/lg` 等非 token 值（setup 向导、playground、auth、各 dialog）。表格链路之外的全量 token 化建议下一轮按页面清（工作量大、风险分散）。
