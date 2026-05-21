# Sentinel · 开发文档

| 文件 | 内容 |
|---|---|
| [openapi.yaml](./openapi.yaml) | OpenAPI 3 契约，前端可用 `openapi-typescript` 直接生成 type-safe client |
| [erd.mmd](./erd.mmd) | 数据库实体关系图（mermaid） |
| [sequence/analyze.mmd](./sequence/analyze.mmd) | 一次实时通话分析的完整时序 |
| [sequence/login.mmd](./sequence/login.mmd) | 登录 + JWT 双 token + 刷新 + 注销 |

## 渲染 mermaid

VSCode 装 `bierner.markdown-mermaid` 插件即可在 markdown 中直接看图。
或者用 mermaid CLI：
```bash
npm i -g @mermaid-js/mermaid-cli
mmdc -i docs/erd.mmd -o docs/erd.svg
```
