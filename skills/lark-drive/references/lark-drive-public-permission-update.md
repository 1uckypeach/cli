# drive +public-permission-update（更新公开权限设置）

本 skill 对应 shortcut：`lark-cli drive +public-permission-update`。

用于更新云文档/云文件的公开权限设置。

> [!CAUTION]
> 这是 `high-risk-write` 操作，会改变文档公开访问或协作边界。必须先用 `--dry-run` 确认请求体；真正执行时需要 `--yes`。

## 何时使用

- 用户明确要求修改“链接分享”“对外分享”“谁可以评论/复制/下载/管理协作者”等公开权限设置。
- 用户提供文档 URL 或 token，并且已经确认要修改目标文档的权限策略。

不要用它来“申请自己访问文档”；申请权限走 [`drive +apply-permission`](lark-drive-apply-permission.md)。

## 身份与权限

- 支持 `--as user` 和 `--as bot`。
- 所需 scope：`docs:permission.setting:write_only`。
- `--type` 会作为 query 参数传给接口。URL 输入可自动推断；bare token 必须显式传。

## 常用命令

```bash
# 先预览：关闭对外分享，并关闭链接分享
lark-cli drive +public-permission-update \
  --token "https://example.feishu.cn/docx/doxcnxxxxxxxxx" \
  --external-access-entity closed \
  --link-share-entity closed \
  --dry-run --as user

# 真正执行：确认 target 和 body 后加 --yes
lark-cli drive +public-permission-update \
  --token "https://example.feishu.cn/docx/doxcnxxxxxxxxx" \
  --external-access-entity closed \
  --link-share-entity closed \
  --yes --as user

# 使用 bare token 时必须显式传 --type
lark-cli drive +public-permission-update \
  --token "doxcnxxxxxxxxx" --type docx \
  --external-access-entity closed \
  --link-share-entity closed \
  --yes --as user
```

## 参数

| 参数 | 必填 | 说明 |
|------|------|------|
| `--token` | 是 | 目标 token 或完整 URL。支持 `/docx/`、`/sheets/`、`/base/`、`/bitable/`、`/file/`、`/wiki/`、`/doc/`、`/mindnotes/`、`/minutes/`、`/slides/` 路径自动提取 token |
| `--type` | URL 可省略；bare token 必填 | 目标类型：`doc`、`sheet`、`file`、`wiki`、`bitable`、`docx`、`mindnote`、`minutes`、`slides` |
| `--security-entity` | 否 | 谁可以复制内容、创建副本、打印、下载：`anyone_can_view`、`anyone_can_edit`、`only_full_access` |
| `--comment-entity` | 否 | 谁可以评论：`anyone_can_view`、`anyone_can_edit` |
| `--share-entity` | 否 | 从组织维度，设置谁可以查看、添加、移除协作者：`anyone`、`same_tenant` |
| `--manage-collaborator-entity` | 否 | 谁可以管理协作者：`collaborator_can_view`、`collaborator_can_edit`、`collaborator_full_access` |
| `--link-share-entity` | 否 | 链接分享设置：`tenant_readable`、`tenant_editable`、`anyone_readable`、`anyone_editable`、`partner_tenant_readable`、`partner_tenant_editable`、`closed` |
| `--copy-entity` | 否 | 谁可以创建副本：`anyone_can_view`、`anyone_can_edit`、`only_full_access` |
| `--external-access-entity` | 否 | 对外分享设置：`open`、`closed`、`allow_share_partner_tenant` |
| `--perm-type` | 否 | 权限范围：`container`、`single_page`。`single_page` 仅支持 `--link-share-entity` 和/或 `--external-access-entity`，不能混用其它权限字段 |
| `--dry-run` | 否 | 只打印请求，不执行 |
| `--yes` | 执行时必填 | 确认 high-risk-write 操作 |

至少要指定一个实际权限字段：`--security-entity`、`--comment-entity`、`--share-entity`、`--manage-collaborator-entity`、`--link-share-entity`、`--copy-entity`、`--external-access-entity`。单独传 `--perm-type` 会被拒绝。

## 请求体形状

```json
{
  "external_access_entity": "closed",
  "link_share_entity": "closed"
}
```

## Wiki URL

传入 `/wiki/<node_token>` 时，shortcut 会以 `type=wiki` 直接调用公开权限接口。如果你要修改 wiki 背后的实际 docx/sheet/bitable 对象，先用 `drive +inspect` 或 `wiki spaces get_node` 拿到底层 `obj_token` 和 `obj_type`，再用 bare token + `--type <obj_type>` 调用。

## 常见错误

| 错误码 | 含义 | 引导 |
|--------|------|------|
| `1063001` | 参数异常 | 检查 token 和 `--type` 是否匹配、资源是否存在、字段枚举是否为 v2 文档支持值。`--external-access-entity` 与 `--link-share-entity` 同时传时可能出现参数冲突；`--perm-type single_page` 也有字段限制 |
| `1063002` | 权限不足 | 确认当前 user 或 bot 是目标文档协作者，并具备编辑或管理权限；bot 场景需要先给文档添加应用权限 |
| `1063003` | 操作不被允许 | 通常是企业策略、可见性、协作者上限或已有权限更高导致；不要简单提示补 scope |
| `1063004` | 用户无分享权限 | 确认调用身份对目标文档有分享权限 |
| `1063005` | 资源已删除 | 确认目标云文档仍存在 |

### 策略 / 密级拦截错误

调用 `lark-cli drive +public-permission-update` 返回以下错误码时，优先按租户策略、对外分享开关或文档密级处理，不要简单提示补 scope。

| 错误码 | 含义 | 引导 |
|--------|------|------|
| `91009` | 对外分享被租户安全策略管控，当前用户无法开启 | 提示用户：对外分享能力被租户安全策略统一管控，无法通过当前命令或当前用户直接开启；需要联系租户管理员调整组织级对外分享策略。 |
| `91010` | 文档对外分享未打开 | 按用户目标分流，不要扩大变更面：只关闭对外分享时仅传 `--external-access-entity closed`；只关闭链接分享时仅传 `--link-share-entity closed`；只有用户明确要求彻底关闭公开访问时，才同时传 `--external-access-entity closed --link-share-entity closed`。只有用户明确要求开放外部访问时，才提示先在文档权限设置中打开对外分享并确认风险后重试。 |
| `91011` | 对外分享被文档密级管控 | 提示用户：对外分享被密级策略拦截，需要打开目标文档，在文档内发起密级豁免或进行密级降级后再重试；回复中必须给出目标文档 URL。 |
| `91012` | 权限设置被文档密级管控 | 提示用户：该权限设置被密级策略拦截，需要打开目标文档，在文档内发起密级豁免或进行密级降级后再重试；回复中必须给出目标文档 URL。 |

遇到 `91011` 或 `91012` 时，如果用户最初提供的是文档 URL，直接把该 URL 原样返回给用户作为操作入口；如果上下文只有 token，先尽量通过已有上下文、搜索结果或元数据恢复目标文档 URL，再给出可点击的文档 URL。

### 服务端运行时错误引导

以下场景 CLI 侧无法静态校验（依赖当前文档状态），需要根据服务端返回的错误信息引导用户：

**link_share 与当前 external 冲突：** 如果只传了 `--link-share-entity` 没传 `--external-access-entity`，服务端会用当前文档的 external 状态校验 link_share 是否合法。例如当前文档 external=closed，设置 link_share=anyone_readable 会被拒绝。按用户目标分流：如果目标是收紧或关闭公开访问，改为同时传 `--external-access-entity closed` 和 `--link-share-entity closed`；只有用户明确要求互联网或外部可访问时，才提示先确认风险，再同时传 `--external-access-entity open` 和对应的 `--link-share-entity`。不要把这类冲突默认解释为需要打开对外分享。

**单页面独立权限：** `--perm-type single_page` 只适用于有 container 的文档，会只修改当前页面的链接分享或对外分享设置；legacy doc 不支持该能力，会被服务端按参数错误拒绝。单页面权限可以与容器权限不同；服务端返回或查询结果中的 `lock_switch=true` 表示当前页面已限制权限、不再继承父级页面权限。

遇到企业策略、对外分享或密级拦截时，不要把它们简单归类成缺少 scope，应引导用户检查租户安全策略和文档权限设置。
