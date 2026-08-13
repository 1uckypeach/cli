// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

// AppsDBMcpEnable enables DB MCP for a Miaoda app or a standalone database.
//
// POST /databases/{id}/mcp/enable 或 /apps/{id}/db/mcp/enable（二选一，域段不对称见 db_mcp_common.go）。
// 独立 DB 仅 manage 可调用；App 沿用现有开发权限校验。开关语义幂等：重复启用返回当前状态。
// 【不返回 url】—— 取连接配置用 +db-mcp-get。
var AppsDBMcpEnable = dbMcpSwitchShortcut("enable", "enabled")
