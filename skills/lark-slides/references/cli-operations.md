# CLI 操作

用 `lark-cli slides` 把事做成：命令、身份、token 解析、创建 / 编辑 / 读取 / 截图 / 上传，以及元素与 verb 的选型。
XML 语法见 `xml-protocol.md`，排版见 `design.md` / `layout.md`，验证见 `verify.md`，排障见 `troubleshooting.md`。

## 身份与认证

默认 `--as user` 并显式指定；权限不足先查是否误用 bot，不默认回退。仅用户明确要求应用身份、或需 bot 持有资源时才用 `--as bot`（bot 创建后会尝试给当前用户授 `full_access`，`permission_grant.status` 为 `granted` / `skipped` / `failed`）。不要擅自转移 owner，用户要转须单独确认。scope 细节以 `../lark-shared/SKILL.md` 为准。

## URL / Token 解析

- `/slides/<token>`：直接作 `xml_presentation_id`。
- `/wiki/<token>`：不能直接用，先 `lark-cli wiki spaces get_node --as user --params '{"token":"<t>"}'` 拿 `obj_token` 并确认 `obj_type=="slides"`。
- 层级：wiki node（`obj_type:slides`）→ `obj_token`（=`xml_presentation_id`）→ slide（`slide_id`）。
- Shortcut（`+xml-get`/`+media-upload`/`+screenshot`/`+replace-slide`/`+replace-pages`）自动解析上述 URL；直接调原生 API 才需自己解析 wiki。

## 创建（`+create`）

```bash
lark-cli slides +create --as user --title "项目汇报"                 # 空白
lark-cli slides +create --as user --title "项目汇报" --slides '[...]'  # 一步加页，每项一页完整 <slide>
```

**方式选择**：简单短 XML（1-3 页、少中文/特殊字符）用 `--slides` 一步；复杂（多页、大段中文、嵌套引号、特殊字符）或 >10 页走两步（先建空白，再 `xml_presentation.slide create` 逐页）。风险在 shell 传参不在页数——1 页够复杂也走两步。

复杂 XML 用 `jq --rawfile` 组装免转义：`--slides "$(jq -n --rawfile s1 a.xml --rawfile s2 b.xml '[$s1,$s2]')"`。

`+create --slides` 里 `<img src="@./x.png">` 会自动上传替换为 `file_token`：路径须 CWD 内相对（绝对/`../` 报 `unsafe file path`）、同图去重、≤20 MB、缺文件校验阶段即报错。

> `--slides` 底层逐页调用、**非原子**。中途失败会停，已建内容保留——先记 `xml_presentation_id`，回读确认状态再续。

返回：`xml_presentation_id` / `title` / `url`（有则展示）/ `revision_id` / `slide_ids`。

提交整页 XML 前先跑 `python3 scripts/xml_text_overlap_lint.py --input <file>`，`summary.error_count` 为 0 才能调接口；lint code 含义与修法见 `troubleshooting.md`。

## 编辑决策树

| 需求 | 用 |
|---|---|
| 已知 `block_id`，换这块（改标题/换图/挪坐标） | `+replace-slide` `block_replace`（`replacement` 根 `id` 由 CLI 自动注入） |
| 只加元素、不动布局 | `+replace-slide` `block_insert`（可选 `insert_before_block_id`，省略则追加页末） |
| 一次动多个元素 | 单次 `--parts` 拼多条，整批原子、任一失败全批不生效、两 action 可混用（≤200 条） |
| 多页整页重建、坐标重排 | `+replace-pages`（原 presentation 内先建新页再删旧页，不生成新链接） |
| 无 shortcut 覆盖的特殊单页操作 | 手动 `slide.create` + `slide.delete` |

无字段级 patch：改一个坐标也要把整块新 XML 用 `block_replace` 写出。局部编辑不整页重建，已有 Slides 不用 `+create` 另建链接。

编辑元素约束：`<td>` 单元格只能 `block_replace` 不能 insert，且整表 `block_replace` 会重建内部 td id、旧 td block_id 立即失效；`<video>` / `<audio>` 非 SML 原生元素、不能写入（insert/replace 返 3350001）；`<polyline>` 的 `points` 读回被服务端规整丢弃（几何已入库）；除 `block_replace` / `block_insert` 外的 action（如 `str_replace`）会被 CLI 拒绝。

读-改-写：先 `+xml-get --slide-id` 读原页挑出目标块 short id，再 `+replace-slide` 改。`slide_id` 与页序不变。`--parts` 支持 `@file`/`-`。`--revision-id` 默认 `-1`（最新版），传不存在版本返 3350002；`--tid` 单人单次留空。

`+replace-pages` 用 `--pages`（每项 `slide_id`+完整 `<slide>`，不支持 `slide_number`、同批不重复 id），非原子，改前先 `--dry-run`/`--validate-only`；默认失败即停，`--continue-on-error` 可继续但最终 partial_failure 非零退出，每页 status 为 `replaced` / `create_failed` / `delete_failed`。

## 元素选型决策树

- 标准数据图表（柱/条/折线/面积/雷达/饼/环/组合）→ 原生 `<chart>`，禁手画替代。
- 流程/时序/架构图 → 优先 `<whiteboard>` Mermaid，`<shape>`+`<line>` 仅 fallback。
- `<chart>` 不支持的图类（散点/漏斗/瀑布）→ `<whiteboard>` SVG。
- 装饰/示意 → whiteboard SVG 或 shape 组合；简单几何/连线 → `<shape>`+`<line>`。
- 表格 → 优先 `rect`+`text` 模拟，其他才 `<table>`。

whiteboard 内部语法、以及按模型身份选 SVG / Mermaid 的规则见 `whiteboard.md`。

## 配图与图标

**图片**：新建用 `+create --slides` 的 `@` 占位符一步到位；给已有页加图用 `+media-upload --file ./pic.png --presentation "$PID"` 拿 `file_token`（`--file` 须 CWD 内相对、≤20 MB），再用 `+replace-slide` 的 `block_insert` 放入，坐标避开现有元素、`width:height` 对齐原图比例避免裁剪。

**图标**：禁盲猜 `iconType`。先 `python3 scripts/iconpark_tool.py search --query "<语义>" --limit 8` 检索，选中的写进 `<icon>` 且必须给非透明 `fillColor`、与背景对比充足；查不到用 shape/line/text 画 fallback，不留空图标位。

## 读取与截图

```bash
lark-cli slides +xml-get --as user --presentation "$PID" --output .lark-slides/plan/$PID/readback.xml  # 全文
lark-cli slides +xml-get --as user --presentation "$PID" --slide-number 2 --raw                        # 单页
lark-cli slides +screenshot --as user --presentation "$PID" --slide-number 1                           # 截图已有页，一次≤10页（--slide-id+--slide-number 合计）
lark-cli slides +screenshot --as user --content @slide.xml --output-name preview                        # 渲染本地单页 XML 预览
```

`+xml-get`：`--slide-id`/`--slide-number` 二选一读单页（`--output` 须 CWD 内相对防截断）；`--revision-id` 默认 `-1`；`--remove-attr-id` 仅全文只读检查；`--raw` 原文写 stdout。

> 截图受应用白名单限制，多数应用不可用。失败只记录，不引导申请 `slides:presentation:screenshot`，改走 `verify.md` 非截图验证，不谎称已截图验收。

## 从模板 / PPTX 导入

先把模板导入成 Slides（不必先加载 lark-drive），再在导入结果上二次创作：

```bash
lark-cli drive +import --as user --file "<template.pptx>" --type slides --json
```

可选 `--name "<标题>"` / `--folder-token <token>`。返回 `ready=false` / `timed_out=true` 时执行返回里的 `next_command`（等价 `lark-cli drive +task_result --scenario import --ticket <TICKET>`）。导入后必须回读、理解每页真实版式再编辑。模板是必须沿用的编辑底稿、非风格参考：只改内容不做设计，不为容纳长文案重画主体，不用新增大卡片遮住原图表/图片/关键 shape。

## 原生 API

调用前必须先查参数，不猜字段：`lark-cli schema slides.<resource>.<method>`。常用：`xml_presentations.get`（全文）、`xml_presentation.slide.create/get/delete/replace`（单页）。原生调用要自己解析 wiki URL，且 `before_slide_id` 必须放 `--data` body 与 slide 同级——放 `--params` 会被当未知 query 静默忽略、新页跑到末尾。
