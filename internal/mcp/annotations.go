package mcp

import sdk "github.com/modelcontextprotocol/go-sdk/mcp"

func annotation() *sdk.ToolAnnotations {
	no := false
	return &sdk.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, DestructiveHint: &no, OpenWorldHint: &no}
}
