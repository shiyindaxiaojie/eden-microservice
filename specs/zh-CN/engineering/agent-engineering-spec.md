# AI 工程护栏规范

## 目标

为 Codex、Claude Code 和兼容智能体提供紧凑、规范优先的工程流程，减少重复错误和无关上下文。

## 聚焦工作流

`eden-microservice` 是轻量路由；`eden-microservice-diagnose` 先建立稳定复现；`eden-microservice-tdd` 在公开 seam 上实现行为；`eden-microservice-code-review` 分开检查契约与工程风险；`eden-microservice-domain-modeling` 解决术语、归属和身份。仅在术语相关或模糊时读取 `CONTEXT.md`。

常规范围内直接决策，不强制澄清；只有缺失选择会改变行为或授权时才提问。

## 易错点

`agent-pitfalls.md` 只记录用户纠正、失败测试或评审确认的可复用经验。不得记录探索失败、临时工具问题或个人环境问题。
