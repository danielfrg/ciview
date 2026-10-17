package main

import (
	"context"
	"encoding/json"
	"fmt"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"os"
	"strings"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	s, e := sdk.NewClient(&sdk.Implementation{Name: "container-smoke", Version: "1"}, nil).Connect(ctx, &sdk.StreamableClientTransport{Endpoint: os.Args[1], DisableStandaloneSSE: true}, nil)
	if e != nil {
		panic(e)
	}
	defer s.Close()
	v, e := s.ListTools(ctx, nil)
	if e != nil {
		panic(e)
	}
	if len(v.Tools) != 4 {
		panic("Unexpected tool count")
	}
	out, e := s.CallTool(ctx, &sdk.CallToolParams{Name: "get_source_status", Arguments: map[string]any{}})
	if e != nil {
		panic(e)
	}
	wantError := len(os.Args) > 2 && os.Args[2] == "unavailable"
	if out.IsError != wantError {
		panic(fmt.Sprintf("Unexpected tool result: %v", out))
	}
	content, _ := json.Marshal(out.Content)
	if wantError && !strings.Contains(string(content), "App discovery unavailable") {
		panic("Unexpected discovery failure")
	}
	fmt.Printf("MCP initialize/list/call passed: %d tools\n", len(v.Tools))
}
