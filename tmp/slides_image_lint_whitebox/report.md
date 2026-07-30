# Slides 图片遮挡 Lint 白盒验证报告

- Presentation: `IvRosVAsWlTG7cdyLSecIHdanzb` (https://bytedance.larkoffice.com/slides/IvRosVAsWlTG7cdyLSecIHdanzb)
- Fixture PNG: `tmp/slides_image_lint_whitebox/fixture.png` (200×200 red square)
- 上传 `file_token`: `PsZsbeRmhoXQ0Wx1bcScsjnunvf`
- 服务端 XML 回读: `tmp/slides_image_lint_whitebox/readback.xml`
- Lint 原始输出: `tmp/slides_image_lint_whitebox/lint-result.json`
  - summary: 16 张 slide，`error_count=3`, `warning_count=0`, `info_count=5`

## 全局服务端 roundtrip 观察

用于隔离服务端问题和 lint 问题；本次全部通过。

| 属性 | 输入 | 回读 | 结论 |
|---|---|---|---|
| `<img alpha="0">` | H3 设置 | 保留 | ok |
| `<content wrap="false">` | H8 设置 | 保留 | ok |
| `<content paddingLeft>` | H5=200 | 保留 | ok |
| `<content textAlign>` | H6=`right`, H9=段落级 `center` | H6 保留；H9 服务端把 `p@textAlign="center"` 同时上抬到 `content@textAlign="center"` | ok（对 lint 更有利） |
| `<content verticalAlign>` | H7=`bottom` | 保留 | ok |
| `<span fontSize>` | H10=42 | 保留；且服务端将其上抬为 `content fontSize="42"` | ok（对 lint 更有利） |
| `<shape vert>` | V1-V2d | 全部 5 个合法值原样保留 | ok |
| XML 顺序 (img/shape) | H4/V3 图片在文字前 | 保留 | ok |

以上属性齐全，任何 lint 判定偏差都不能归因给 roundtrip 缺属性。

## 用例矩阵结果

Slide id / element id 均取自 `readback.xml`。

| 用例 | slide_id | text id | img id | 预期 lint | 实际 lint | 判定 |
|---|---|---|---|---|---|---|
| H1 真正覆盖 | pJJ | bqq | bqu | `error image_covers_text` | `error image_covers_text` (bqu covers bqq) | pass |
| H2 仅碰空白 | pJF | bqN | bqj | 无 `image_covers_text` | 无 issue | pass |
| H3 透明图 | pJL | bqy | bqM | 无 `image_covers_text` | 无 issue | pass |
| H4 图片在文字后方 (XML 顺序在前) | pJu | bqJ | bqv | 无 `image_covers_text` | 无 issue | pass |
| H5 左 padding | pJB | bqe | bqP | 无 `image_covers_text` | 无 issue | pass |
| H6 右对齐 | pJn | bqx | bqK | 无 `image_covers_text` | 无 issue | pass |
| H7 底部对齐 | pJM | bqW | bqz | 无 `image_covers_text` | 无 issue | pass |
| H8 wrap=false 溢出 | pJV | bqi | bqD | `error image_covers_text` | `error image_covers_text` (bqD covers bqi) | pass |
| H9 段落级对齐 | pJk | bqQ | bqb | probe | 无 issue | pass；见探针结论 |
| H10 行内字号 | pJc | bqL | bqC | probe | `error image_covers_text` (bqC covers bqL) | pass；见探针结论 |
| V1 vert | pJE | bqf | bql | `info image_may_cover_vertical_text`，无 error | 同预期 | pass |
| V2a vert270 | pJq | bqT | bqU | `info image_may_cover_vertical_text` | 同预期 | pass |
| V2b word-art-vert | pJQ | bqr | bqh | `info image_may_cover_vertical_text` | 同预期 | pass |
| V2c word-art-vert-rtl | pJG | bqm | bqE | `info image_may_cover_vertical_text` | 同预期 | pass |
| V2d ea-vert | pJm | bqa | bqw | `info image_may_cover_vertical_text` | 同预期 | pass |
| V3 图在纵向文字前 | pJy | bqO | bqS | 无 vertical info | 无 issue | pass |

- `image_covers_text` error 恰好且仅出现在 H1、H8、H10。
- `image_may_cover_vertical_text` info 恰好出现在 V1、V2a-d。
- H2-H7、H3、H4、V3 无 `image_covers_text` / vertical info。

## 探针（H9 / H10）结论

两条都不是 lint 缺口，但需要显式记录归因，避免以后误改。

### H9：段落级 `textAlign`

- 意图：用 `<content>` 不设对齐、`<p textAlign="center">` 触发段落级对齐，图片放左侧空白，看 lint 是否漏建模段落级 align。
- 服务端表现：把 `p@textAlign="center"` 同步上抬到 `content@textAlign="center"`（回读 XML 内两处都有）。
- lint 表现：`estimate_text_visual_bbox` 直接读 `content@textAlign`，此时是 `center`，短文本 `Hi` 用 fs=32 估宽约 35px，居中后 bbox 落在 shape 中段，与 `(110,120,80,40)` 左侧空白图不相交，因此无 error。
- 结论：**当前实现在段落级对齐路径上，因服务端 up-promote 而“看起来”被覆盖**；不代表 `estimate_text_visual_bbox` 真的读 `<p>@textAlign`。如果后续服务端不再把 `p@textAlign` 上抬，此路径会退化。属于**文档化项**，不修改 lint。

### H10：行内 `<span fontSize>`

- 意图：`<content fontSize="16">` 底稿、`<span fontSize="42">` 单字，测试 lint 是否只用 content 字号建 bbox。
- 服务端表现：将 `<span fontSize="42">` 上抬到 `<content fontSize="42">`。
- lint 表现：`extract_elements` 读到 content fontSize=42，估算 bbox 宽约 23px、高约 50.4px；shape (100,100,400,80)、verticalAlign 默认 `middle`，可视 bbox ≈ (100, 114.8) - (123, 165.2)；图片 (120,115) 80×50 相交约 3px 宽、约 50px 高 → 触发 `image_covers_text`。
- 结论：**lint 实际使用的仍是 content 级 fontSize**，只是服务端替 lint 完成了 up-promote。若测试目的是校验 lint 对纯 span-only 字号的建模，需在“绕过服务端 up-promote”的场景（本地 XML 直跑）再验一次，本次白盒不能证伪。属于**文档化项**，不修改 lint。

## 通过判定

- H1 / H8：唯一 error 且指向正确的 image → text id ✓
- H2 / H3 / H4 / H5 / H6 / H7：无 `image_covers_text` ✓
- V1 / V2a / V2b / V2c / V2d：仅 `info image_may_cover_vertical_text`，无 error ✓
- V3：无纵向 info ✓
- H9 / H10：结果与“当前实现 + 服务端 roundtrip”组合一致，见上文归因。

无失败用例，无待补 lint 修复。

## 交付物清单

- 每页创建前的 XML: `tmp/slides_image_lint_whitebox/cases/{H1..H10,V1,V2a,V2b,V2c,V2d,V3}.xml`
- 完整服务端回读: `tmp/slides_image_lint_whitebox/readback.xml`
- lint 原始输出: `tmp/slides_image_lint_whitebox/lint-result.json`
- 本报告: `tmp/slides_image_lint_whitebox/report.md`
- 创建响应汇总: `tmp/slides_image_lint_whitebox/create_results.json`
