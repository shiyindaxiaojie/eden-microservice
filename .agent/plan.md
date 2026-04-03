# 告警配置事件触发逻辑完整实现

## 问题分析

当前系统已有：

- ✅ 事件记录机制（`State.AppendEvent()` → `EventLog.Append()`）
- ✅ 告警规则配置（`alert.Rule` 包含 `event_code`, `threshold`, `window_sec`, `channel_ids`）
- ✅ 通知渠道配置（`notify.Channel`）
- ✅ 通知发送引擎（`notify.Engine.Send()`）
- ✅ 模板变量定义（title/body template）

**缺失的关键链路**：没有告警评估器将事件与规则匹配，实现"在指定时间窗口内事件达到阈值次数后触发通知"的逻辑。

## 设计方案

### 核心：Alert Evaluator（告警评估器）

```
事件流: AppendEvent() → EventLog.Append()
                    ↓ (新增回调)
              AlertEvaluator.Evaluate(event)
                    ↓
         匹配规则 → 滑动窗口计数 → 达到阈值?
                    ↓ YES
         渲染模板 → 查找通知渠道 → Engine.Send()
```

### 关键设计决策

1. **滑动窗口**：为每条规则维护事件时间戳列表，评估时清理过期记录，统计窗口内事件数
2. **冷却机制**：触发通知后设置冷却期（= window_sec），防止同一规则短时间内重复通知
3. **模板渲染**：使用 Go 的 `strings.NewReplacer` 替换 `{{ var }}` 变量
4. **线程安全**：评估器使用 `sync.Mutex` 保护内部状态
5. **回调钩子**：在 `catalog.State` 上添加 `OnEvent` 回调，事件追加后异步触发评估

---

## Proposed Changes

### 告警评估器（核心新增）

#### [NEW] [evaluator.go](file:///d:/Workspaces/Git/eden-go-registry/internal/alert/evaluator.go)

告警评估器，包含：

- `Evaluator` 结构体：持有规则提供者、通知渠道提供者、通知引擎
- 滑动窗口状态：`map[ruleID][]time.Time` 记录每条规则匹配到的事件时间戳
- 冷却状态：`map[ruleID]time.Time` 记录上次通知时间
- `Evaluate(event)` 方法：
  1. 遍历所有启用的规则
  2. 匹配 `event_code`
  3. 将当前事件时间戳加入对应规则的滑动窗口
  4. 清理窗口外的过期时间戳
  5. 如果窗口内计数 ≥ threshold 且不在冷却期：
     - 渲染模板（替换变量）
     - 查找对应通知渠道
     - 通过 Engine.Send() 发送通知
     - 重置窗口计数，设置冷却期
- `renderTemplate()` 方法：替换模板变量
- `defaultTitle/defaultBody()` 方法：规则未自定义模板时使用默认格式

---

### 事件回调钩子

#### [MODIFY] [state.go](file:///d:/Workspaces/Git/eden-go-registry/internal/catalog/state.go)

- 新增 `onEventCallback func(*Event)` 字段
- 新增 `SetOnEventCallback(fn)` 方法
- 在 `AppendEvent()` 中，事件追加后，异步调用回调（`go callback(event)`）

---

### 系统初始化集成

#### [MODIFY] [handler.go](file:///d:/Workspaces/Git/eden-go-registry/internal/transport/http/handler.go)

- 新增 `alertEvaluator *alert.Evaluator` 字段
- 在 `NewHandler()` 中创建 `Evaluator` 实例

#### [MODIFY] [main.go](file:///d:/Workspaces/Git/eden-go-registry/cmd/server/main.go)

- 将 `alertEvaluator` 的 `Evaluate` 方法注册为 `State.OnEvent` 回调
- 修改 `NewHandler` 签名以接受 catalog State，用于注册回调

---

## Open Questions

> [!IMPORTANT]
> **Handler vs main.go 初始化位置**：评估器需要同时访问 alert store、notify store 和 notify engine。当前这些都在 Handler 中创建。建议在 Handler 中创建评估器并注册回调，这样不需要修改 main.go 签名。是否同意？

> [!NOTE]
> **冷却期策略**：建议触发通知后，冷却期等于规则的 `window_sec`。例如规则配置 300 秒窗口、3 次阈值，触发后 300 秒内不再重复通知。是否合理？

---

## Verification Plan

### Automated Tests

- `go test ./internal/alert/...` — 评估器单元测试
- `go build ./cmd/server/...` — 编译通过

### Manual Verification

- 配置一条告警规则（如 service_offline 事件，阈值 1，窗口 300 秒）
- 手动触发服务下线事件
- 验证通知是否通过配置的渠道发送
