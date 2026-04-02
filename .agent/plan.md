# 告警配置 UI 重新设计

## 问题分析

当前告警配置页面核心问题：

1. 概念割裂：模板 / 规则分散在子 tab，需来回跳转
2. 模板变量裸露：`{{ event_code }}` 等无任何说明，用户不知道含义和用法
3. 三步操作链（渠道 → 模板 → 规则）没有引导

## 设计方案

### 核心理念：**"规则直接绑定渠道，模板作为高级选项内联"**

- **简化主流程**：规则编辑器直接选择通知渠道，系统内置默认通知格式
- **保留自定义能力**：在规则编辑器中提供"自定义模板"折叠区域
- **变量参考面板**：在自定义模板区域旁边展示可用变量列表和说明

### 数据结构变更

**告警规则** 从 `template_ids` 改为：

- `channel_ids: string[]` — 直接关联渠道
- `title_template?: string` — 可选，内联标题模板
- `body_template?: string` — 可选，内联内容模板
- 留空时使用系统默认通知格式

**删除 AlertTemplate 独立实体** — 模板不再是独立管理的对象，而是内联在规则中的可选高级选项。

### 可用模板变量

基于 Event 结构定义和 Rule 配置，以下变量在模板渲染时可用：

| 变量名             | 来源            | 说明               | 示例值                  |
| ------------------ | --------------- | ------------------ | ----------------------- |
| `{{ event_code }}` | Rule.event_code | 触发事件类型       | `service_offline`       |
| `{{ event_name }}` | 事件中文名映射  | 事件类型的中文描述 | `服务下线`              |
| `{{ threshold }}`  | Rule.threshold  | 触发阈值次数       | `3`                     |
| `{{ window_sec }}` | Rule.window_sec | 统计窗口秒数       | `300`                   |
| `{{ window_min }}` | window_sec / 60 | 统计窗口分钟数     | `5`                     |
| `{{ service }}`    | Event.service   | 触发事件的服务名   | `order-service`         |
| `{{ instance }}`   | Event.instance  | 触发事件的实例地址 | `10.0.1.5:8080`         |
| `{{ message }}`    | Event.message   | 事件描述信息       | `Instance deregistered` |
| `{{ timestamp }}`  | Event.timestamp | 事件发生时间       | `2026-04-02 13:30:00`   |
| `{{ rule_name }}`  | Rule.name       | 告警规则名称       | `服务下线告警`          |
| `{{ count }}`      | 实际触发次数    | 窗口内实际累计次数 | `5`                     |

### UI 交互设计

**规则编辑器（右侧面板）布局：**

```
┌───────────────────────────────────────────────┐
│  新增告警规则                    [取消] [保存]  │
│  当事件达到阈值时，通过指定渠道发送告警通知       │
├───────────────────────────────────────────────┤
│  规则名称: [_____________]                     │
│  事件类型: [服务下线    ▼]   启用: [✓]         │
│  阈值次数: [3        ▲▼]                      │
│  统计窗口: [300      ▲▼] 秒                   │
│  通知渠道: [钉钉通知, 邮件 ▼] (多选)            │
├───────────────────────────────────────────────┤
│  ▸ 自定义消息模板（可选）                       │
│  ┌─默认格式预览──────────────────────────────┐ │
│  │ [Eden] 服务下线                          │ │
│  │ 事件：服务下线（service_offline）          │ │
│  │ 触发条件：300 秒内达到 3 次               │ │
│  │ 请尽快检查对应服务实例与节点状态。           │ │
│  └──────────────────────────────────────────┘ │
│                                               │
│  展开后:                                      │
│  ▾ 自定义消息模板（可选）                      │
│  ┌─可用变量─────────────────┐                 │
│  │ {{ event_code }}  事件类型 │                │
│  │ {{ event_name }}  事件名称 │  ← 点击插入     │
│  │ {{ threshold  }}  阈值次数 │                │
│  │ {{ window_sec }}  窗口秒数 │                │
│  │ {{ service    }}  服务名   │                │
│  │ {{ instance   }}  实例地址 │                │
│  │ ...                      │                │
│  └──────────────────────────┘                 │
│  标题模板: [Eden 告警 - {{ event_name }}___]    │
│  内容模板: [________________________]          │
│           [________________________]          │
│           [________________________]          │
└───────────────────────────────────────────────┘
```

---

## Proposed Changes

### 后端数据结构

#### [MODIFY] [store.go](file:///d:/Workspaces/Git/eden-go-registry/internal/alert/store.go)

- **删除** `Template` 结构体
- **Rule 结构体变更**：
  - 删除 `TemplateIDs []string`
  - 新增 `ChannelIDs []string` `json:"channel_ids"`
  - 新增 `TitleTemplate string` `json:"title_template,omitempty"`
  - 新增 `BodyTemplate string` `json:"body_template,omitempty"`
- **Config 结构体变更**：
  - 删除 `Templates []Template`
- 更新 `normalizeConfig` / `defaultConfig`

#### [MODIFY] [store_test.go](file:///d:/Workspaces/Git/eden-go-registry/internal/alert/store_test.go)

- 更新测试用例适配新结构

---

### 前端 API 类型

#### [MODIFY] [index.ts](file:///d:/Workspaces/Git/eden-go-registry/web/src/api/registry/index.ts)

- **删除** `AlertTemplate` 接口
- **AlertRule** 接口：
  - 删除 `template_ids`
  - 新增 `channel_ids: string[]`
  - 新增 `title_template?: string`
  - 新增 `body_template?: string`
- **AlertConfig** 接口：删除 `templates` 字段

---

### 前端 UI

#### [MODIFY] [settings.vue](file:///d:/Workspaces/Git/eden-go-registry/web/src/views/settings.vue)

**Script 变更：**

- 删除所有模板独立实体相关代码（`templateForm`、`templateEditorVisible`、`templateMode`、`TEMPLATE_PRESETS`、`openTemplateDialog`、`saveTemplateDraft`、`removeTemplate`、`pagedTemplates`、`alertSubTab` 等）
- 新增 `TEMPLATE_VARIABLES` 常量数组，列出所有可用变量及说明
- 新增 `showCustomTemplate` ref，控制折叠
- 新增 `defaultTitleTemplate` / `defaultBodyTemplate` 计算属性（根据当前规则表单动态预览）
- 新增 `insertVariable(varName)` 方法
- `emptyRule()` 更新为 `channel_ids`、`title_template`、`body_template`
- `saveRuleDraft()` 更新验证逻辑

**Template 变更 — 告警配置 tab：**

- 删除"告警模板 / 事件阈值规则"子 tab 切换
- 直接展示告警规则列表 + 编辑器（与消息渠道页面同构）
- 规则编辑器增加：
  - "通知渠道"多选下拉（直接选消息渠道）
  - 折叠式"自定义消息模板"区域
    - 默认折叠，显示系统默认格式的只读预览
    - 展开后显示：变量参考面板（可点击插入）+ 标题模板输入 + 内容模板输入
  - 引导文字说明
- 规则卡片展示：事件中文名、阈值窗口描述、绑定渠道名称

**Style 变更：**

- 删除模板子 tab 相关样式
- 新增变量参考面板样式（标签式、可点击）
- 新增折叠区域样式
- 新增默认格式预览样式

---

## Verification Plan

### Automated Tests

- `go test ./internal/alert/...` — 后端结构变更不破坏编译
- `npm run build` — 前端编译通过

### Manual Verification

- 浏览器验证告警规则完整 CRUD 流程
- 验证变量参考面板展示和点击插入功能
- 验证默认格式预览根据表单输入动态更新
- 验证消息渠道页面不受影响
