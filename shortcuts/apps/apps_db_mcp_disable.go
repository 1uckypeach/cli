// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package apps

// AppsDBMcpDisable disables DB MCP for a Miaoda app or a standalone database.
//
// POST /databases/{id}/mcp/disable 或 /apps/{id}/db/mcp/disable（二选一）。
// 【停用而非删除配置】—— 再次 enable 可直接恢复，标识与权限规则同 +db-mcp-enable。
var AppsDBMcpDisable = dbMcpSwitchShortcut("disable", "disabled")
