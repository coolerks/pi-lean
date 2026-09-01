# Pi 官方文档中文翻译报告

## 范围

- 研究基线：`853a80d26c90a14c1886f0ebb8ffaae133ca2185`
- 来源：`packages/agent/docs/` 4 页、`packages/coding-agent/docs/` 30 页，共 34 页。
- 目标：`ai-agent-learning/pi-docs/` 下对应的 34 个 `.mdx` 页面。
- `harness.v2.md` 是工作区未跟踪的设计稿；报告保留这一事实边界，不将它描述为已交付实现。

## 翻译规则

1. 以英文源文件和当前源码为准，不以旧目标译文反推内容。
2. 普通叙述译为自然中文；API、类型、字段、命令、路径、协议值和正式名称保持原样或保留必要英文。
3. 保留原文的标题层级、段落、列表、表格、代码围栏、Mermaid、链接和提示。
4. `## 完整原文对照` 之后逐字保留英文源文件，作为审阅和回溯依据；该部分不得翻译或格式化。
5. 源文档中的仓库相对源码/示例链接在中文站点中改为固定研究 commit 的 GitHub 链接；站内文档链接改为 Mintlify 路由。

## 完成情况

| 页面组 | 页面数 | 结构审计结果 |
| --- | ---: | --- |
| Agent 文档 | 4 | 4/4 有中文正文和完整英文附录 |
| Coding Agent 文档 | 30 | 30/30 有中文正文和完整英文附录 |
| 合计 | 34 | `translation-map.json` 34/34 |

本轮补齐并组装了 `extensions.mdx`、`rpc.mdx` 和 `sdk.mdx` 的完整正文；同时修复了 TUI 翻译中遗漏的“什么时候需要这种模式”小节标题。

关键结构指标：

- `extensions.md`：122 个正文标题、109 个代码块。
- `rpc.md`：88 个正文标题、126 个代码块。
- `sdk.md`：33 个正文标题、37 个代码块。
- 3 页的代码围栏/语言标签/代码块数量与英文源一致；代码审计未发现实质性代码 token 差异，仅有尾随空白归一化。
- 全部 34 页英文附录 byte-equivalent：`python3 /tmp/check_appendices.py` 输出 34/34 `OK`。
- Mintlify 站内链接检查通过；三页中的章节锚点已按中文标题重新映射。

## 验证

已执行：

```text
cd ai-agent-learning && npx --yes mint@latest validate
cd ai-agent-learning && npx --yes mint@latest broken-links
cd ai-agent-learning && npx --yes mint@latest a11y
cd ai-agent-learning && npx --yes mint@latest dev --port 3000
curl -fsS http://127.0.0.1:3000/
curl -fsS http://127.0.0.1:3000/00-guide/reading-path
```

结果：

- `validate` 通过。
- `broken-links` 通过，无断链。
- `a11y` 无 MDX、图片或视频可访问性错误；品牌主色仍只有 AAA 建议 WARN。
- 本地首页和阅读路径均返回 HTML；dev server 已停止。
- 未执行 Vercel 部署、push 或 PR 操作。

根目录 `npm run check` 的既有 Pi monorepo models catalog 类型错误仍记录在 `VALIDATION.md`，本次没有修改 Pi 核心或生成模型文件。
