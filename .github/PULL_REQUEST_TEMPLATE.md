<!--
Agents should select the template using the Pull requests rules in `AGENTS.md`.
-->
# ⚠️ 提交说明 / PR Notice

English template: `.github/PULL_REQUEST_TEMPLATE/en.md`

> [!IMPORTANT]
>
> - 提交者按验证证据核对并精炼正文，保留维护者需要复核的要点。正文与后续评论仅提交已核对的内容。Agent 负责提交时，按仓内流程安排独立审查，并在下方记录结果。
> - 开启透传请勿反馈问题；透传会直接发送内容，不会经过 new-api 的逻辑处理。
> - 请按本模板填写后再提交。

## 🔗 关联任务 / Related Issue
- 新功能请填写下方 Issue 编号；若还没有对应 Issue，请先自行创建。功能讨论请放在 Issue 中进行。
- 改动较大或方向性变更，请先在关联 Issue 中与维护者达成一致，再提交 PR。
- Bug 修复请关联对应 Issue。设计取舍、理解偏差或预期不一致，更适合作为讨论或功能请求。

- Closes #

## 🚀 变更类型 / Type of change
- [ ] 🐛 Bug 修复 (Bug fix)
- [ ] ✨ 新功能 (New feature)
- [ ] ⚡ 性能优化 / 重构 (Refactor)
- [ ] 📝 文档更新 (Documentation)

## 📝 变更描述 / Description
(简述做了什么、为什么生效。不得直接粘贴未经人工过滤的大段 AI 文本。如果难以简述，建议先拆分范围，或在 Issue 中与维护者对齐。)

## 文档影响 / Documentation impact
(按[文档影响规则](../.agents/rules/development-workflow.md#文档影响)列出变更、更新的文档和依据，或说明无影响的理由。涉及跨仓时记录关联任务或 PR、状态及发布验收条件。)

## 📸 运行证明 / Proof of Work
(请写明如何验证：实际命令或步骤，以及观察结果。仅声明 `go build` 通过或测试通过，不视为有效证明。UI 变更请附截图或录屏；Bug 修复请说明复现过程与修复后结果。)

## 审查记录 / Review record
(Agent 提交时记录独立审查的提交 SHA、结论、复核的验证证据和未验证项。)

## ✅ 提交前检查项 / Checklist
- [ ] **非重复提交:** 我已搜索现有的 [Issues](https://github.com/QuantumNous/new-api/issues) 与 [PRs](https://github.com/QuantumNous/new-api/pulls)，确认不是重复提交。
- [ ] **新功能关联 Issue:** 若此 PR 标记为 `New feature`，我已关联对应 Issue；若尚无 Issue，我已先自行创建。
- [ ] **事前沟通:** 若改动较大或涉及方向性变更，已在关联 Issue 中与维护者沟通并达成一致。
- [ ] **功能范围:** 本 PR 不是 Coding Plan、逆向渠道、第三方封装接口，也不是对 Codex 渠道类型的改动。
- [ ] **非透传:** 本 PR 不是针对开启透传后的转发行为；透传会直接发送内容，不会经过 new-api 的逻辑处理。
- [ ] **范围聚焦:** 本 PR 为一项聚焦改动，未包含无关代码。
- [ ] **模型契约:** 若改动涉及模型、模型页、operation、参数或 provider/task adaptor，已更新治理 spec、重新生成 `contracts.json` 并运行 `make model-contract-check`；未手改生成物、未新增按模型硬编码表。
- [ ] **本地验证:** 已按变更路径实际验证，并写明命令与观察结果。仅声明 `go build` 通过或测试通过，不视为有效证明。
- [ ] **安全合规:** 代码中无敏感凭据，且符合项目代码规范。
