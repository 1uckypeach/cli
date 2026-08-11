# whiteboard +reset-version（回退画板版本）

> **前置条件：** 先阅读 [`../../lark-shared/SKILL.md`](../../lark-shared/SKILL.md) 了解认证、全局参数和安全规则。

将画板回退到指定的历史版本，用于撤销一次异常或误操作的编辑。回退是整板生效的写操作，会覆盖当前内容。

## 参数

| 参数                   | 必填 | 说明                                              |
|----------------------|----|-------------------------------------------------|
| `--whiteboard-token` | 是  | 画板 token，需要拥有画板的编辑权限                             |
| `--target-revision`  | 是  | 要回退到的目标历史版本号，正整数字符串（如 `10221`）                  |

## 说明

- 这是**写操作**，回退后当前内容会被目标版本覆盖，执行前请确认版本号无误。
- 回退为同步操作，成功后返回被回退到的版本号。

## 如何获取 `--target-revision`

目标版本号来自**上一次写操作返回的 `previous_revision`**——它就是那次写入前的画板版本号，即"异常操作发生前"的干净版本。

- 用 [`+update`](lark-whiteboard-update.md) 以 `raw` 格式更新成功后，返回值 / 输出会带 `previous_revision`（如 `10221`）。写入后先记录该值。
- 一旦发现该次更新异常，把记录的 `previous_revision` 作为 `--target-revision` 传入本命令即可回退。
- 画板 OpenAPI 目前**未提供**独立的版本列表接口，`previous_revision` 是可靠的版本号来源；若未提前记录，可从飞书画板界面的历史记录中人工查证。

典型流程：

```bash
# 1) 更新画板，记录返回的 previous_revision（例如 10221）
lark-cli whiteboard +update --whiteboard-token "wbcnxxxxxxxx" \
  --input_format raw --source @./nodes.json --overwrite --as user
# -> Revision before this write: 10221 ...

# 2) 若这次更新有误，回退到写入前的版本
lark-cli whiteboard +reset-version --whiteboard-token "wbcnxxxxxxxx" \
  --target-revision "10221" --as user
```

## 示例

### 回退画板到指定版本

```bash
lark-cli whiteboard +reset-version \
  --whiteboard-token "wbcnxxxxxxxx" \
  --target-revision "10221" \
  --as user
```
