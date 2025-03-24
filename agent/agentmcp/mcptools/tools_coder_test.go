package mcptools_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cdr.dev/slog/sloggers/slogtest"
	"github.com/coder/coder/v2/agent/agentmcp/mcptools"
	"github.com/coder/coder/v2/codersdk"
	"github.com/coder/coder/v2/pty/ptytest"
	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoderReportTask(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	pty := ptytest.New(t)
	mcpSrv, closeSrv := startTestMCPServer(ctx, t, pty.Input(), pty.Output())
	t.Cleanup(func() {
		_ = closeSrv()
	})

	mcptools.RegisterCoderReportTask(mcpSrv, slogtest.Make(t, nil))

	// TODO: Mock the coder server. The task reporting endpoint is not implemented yet.

	ctr := mcp.JSONRPCRequest{
		ID:      "1",
		JSONRPC: "2.0",
		Request: mcp.Request{
			Method: "tools/call",
		},
		Params: struct { // Unfortunately, there is no type for this yet.
			Name      string         "json:\"name\""
			Arguments map[string]any "json:\"arguments,omitempty\""
			Meta      *struct {
				ProgressToken mcp.ProgressToken "json:\"progressToken,omitempty\""
			} "json:\"_meta,omitempty\""
		}{
			Name: "coder_report_task",
			Arguments: map[string]any{
				"summary":             "Test summary",
				"link":                "https://example.com",
				"emoji":               "🔍",
				"done":                false,
				"coder_url":           "http://localhost:3000",
				"coder_session_token": "test-token",
			},
		},
	}

	require.NoError(t, json.NewEncoder(pty.Input()).Encode(ctr), "failed to encode call tool request")
	_ = pty.ReadLine(ctx) // skip the echo

	resp := pty.ReadLine(ctx)
	var jrpcErr mcp.JSONRPCError
	require.NoError(t, json.NewDecoder(strings.NewReader(resp)).Decode(&jrpcErr), "failed to decode call tool error result")
	require.Empty(t, jrpcErr.Error.Message)
	require.Zero(t, jrpcErr.Error.Code)
	var jrpcResp mcp.JSONRPCResponse
	require.NoError(t, json.NewDecoder(strings.NewReader(resp)).Decode(&jrpcResp), "failed to decode call tool result")
}

func TestCoderWhoami(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	pty := ptytest.New(t)
	mcpSrv, closeSrv := startTestMCPServer(ctx, t, pty.Input(), pty.Output())
	t.Cleanup(func() {
		_ = closeSrv()
	})

	mockUser := codersdk.User{
		ReducedUser: codersdk.ReducedUser{
			MinimalUser: codersdk.MinimalUser{
				ID:       uuid.New(),
				Username: "testuser",
			},
		},
	}
	mockCoderServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !assert.Equal(t, r.Header.Get("Coder-Session-Token"), "test-token", "coder_session_token is not set") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		assert.NoError(t, json.NewEncoder(w).Encode(mockUser))
	}))
	t.Cleanup(mockCoderServer.Close)

	mcptools.RegisterCoderWhoami(mcpSrv)

	ctr := mcp.JSONRPCRequest{
		ID:      "1",
		JSONRPC: "2.0",
		Request: mcp.Request{
			Method: "tools/call",
		},
		Params: struct { // Unfortunately, there is no type for this yet.
			Name      string         "json:\"name\""
			Arguments map[string]any "json:\"arguments,omitempty\""
			Meta      *struct {
				ProgressToken mcp.ProgressToken "json:\"progressToken,omitempty\""
			} "json:\"_meta,omitempty\""
		}{
			Name: "coder_whoami",
			Arguments: map[string]any{
				"coder_url":           mockCoderServer.URL,
				"coder_session_token": "test-token",
			},
		},
	}

	require.NoError(t, json.NewEncoder(pty.Input()).Encode(ctr), "failed to encode call tool request")
	_ = pty.ReadLine(ctx) // skip the echo

	resp := pty.ReadLine(ctx)

	var sb strings.Builder
	userJSON, err := json.Marshal(mockUser)
	require.NoError(t, err)
	require.NoError(t, json.NewEncoder(&sb).Encode(mcp.JSONRPCResponse{
		ID:      "1",
		JSONRPC: "2.0",
		Result: mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(string(userJSON)),
			},
		},
	}))
	require.JSONEq(t, sb.String(), resp)
}

func TestCoderListWorkspaces(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	pty := ptytest.New(t)
	mcpSrv, closeSrv := startTestMCPServer(ctx, t, pty.Input(), pty.Output())
	t.Cleanup(func() {
		_ = closeSrv()
	})

	mockWorkspaces := codersdk.WorkspacesResponse{
		Count: 1,
		Workspaces: []codersdk.Workspace{
			{
				ID:   uuid.New(),
				Name: "test-workspace",
			},
		},
	}
	mockCoderServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !assert.Equal(t, r.Header.Get("Coder-Session-Token"), "test-token", "coder_session_token is not set") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		assert.NoError(t, json.NewEncoder(w).Encode(mockWorkspaces))
	}))
	t.Cleanup(mockCoderServer.Close)

	mcptools.RegisterCoderListWorkspaces(mcpSrv)

	ctr := mcp.JSONRPCRequest{
		ID:      "1",
		JSONRPC: "2.0",
		Request: mcp.Request{
			Method: "tools/call",
		},
		Params: struct { // Unfortunately, there is no type for this yet.
			Name      string         "json:\"name\""
			Arguments map[string]any "json:\"arguments,omitempty\""
			Meta      *struct {
				ProgressToken mcp.ProgressToken "json:\"progressToken,omitempty\""
			} "json:\"_meta,omitempty\""
		}{
			Name: "coder_list_workspaces",
			Arguments: map[string]any{
				"coder_url":           mockCoderServer.URL,
				"coder_session_token": "test-token",
			},
		},
	}

	require.NoError(t, json.NewEncoder(pty.Input()).Encode(ctr), "failed to encode call tool request")
	_ = pty.ReadLine(ctx) // skip the echo

	resp := pty.ReadLine(ctx)

	var sb strings.Builder
	workspacesJSON, err := json.Marshal(mockWorkspaces)
	require.NoError(t, err)
	require.NoError(t, json.NewEncoder(&sb).Encode(mcp.JSONRPCResponse{
		ID:      "1",
		JSONRPC: "2.0",
		Result: mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(string(workspacesJSON)),
			},
		},
	}))
	require.JSONEq(t, sb.String(), resp) // TODO: what's missing here?
}

func startTestMCPServer(ctx context.Context, t testing.TB, stdin io.Reader, stdout io.Writer) (*server.MCPServer, func() error) {
	t.Helper()

	mcpSrv := server.NewMCPServer(
		"Test Server",
		"0.0.0",
		server.WithInstructions(""),
		server.WithLogging(),
	)

	stdioSrv := server.NewStdioServer(mcpSrv)

	cancelCtx, cancel := context.WithCancel(ctx)
	closeCh := make(chan struct{})
	done := make(chan error)
	go func() {
		defer close(done)
		srvErr := stdioSrv.Listen(cancelCtx, stdin, stdout)
		done <- srvErr
	}()

	go func() {
		select {
		case <-closeCh:
			cancel()
		case <-done:
			cancel()
		}
	}()

	return mcpSrv, func() error {
		close(closeCh)
		return <-done
	}
}
