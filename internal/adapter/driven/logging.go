package driven

import (
	"context"
	"log/slog"
)

// Shared slog attribute keys for all adapters.
const (
	LogKeyBackend   = "backend"
	LogKeyOperation = "op"
	LogKeyIssueKey  = "key"
	LogKeyTitle     = "title"
	LogKeyProject   = "project"
	LogKeyQuery     = "query"
	LogKeyName      = "name"
	LogKeyCount     = "count"
	LogKeyID        = "id"
	LogKeyTeam      = "team"
	LogKeyStatus    = "status"
	LogKeyError     = "error"
	LogKeyCacheHit  = "cache_hit"
	LogKeyElapsed   = "elapsed"

	logMsgAPIError = "API error"
)

// Log message constants for sloglint compliance.
const (
	logMsgOp     = "op"
	logMsgOpDone = "op.done"
	logMsgWrite  = "write"
	logMsgError  = "op.error"
)

// LogOp logs a debug-level operation start.
func LogOp(ctx context.Context, backend, op string, attrs ...slog.Attr) {
	attrs = append([]slog.Attr{
		slog.String(LogKeyBackend, backend),
		slog.String(LogKeyOperation, op),
	}, attrs...)
	slog.LogAttrs(ctx, slog.LevelDebug, logMsgOp, attrs...)
}

// LogWrite logs an info-level write operation.
func LogWrite(ctx context.Context, backend, op string, attrs ...slog.Attr) {
	attrs = append([]slog.Attr{
		slog.String(LogKeyBackend, backend),
		slog.String(LogKeyOperation, op),
	}, attrs...)
	slog.LogAttrs(ctx, slog.LevelInfo, logMsgWrite, attrs...)
}

// LogError logs a warning-level adapter error (for adapters using client libraries, not raw HTTP).
func LogError(ctx context.Context, backend, op string, err error, attrs ...slog.Attr) {
	attrs = append([]slog.Attr{
		slog.String(LogKeyBackend, backend),
		slog.String(LogKeyOperation, op),
		slog.String(LogKeyError, err.Error()),
	}, attrs...)
	slog.LogAttrs(ctx, slog.LevelWarn, logMsgError, attrs...)
}

// LogOpDone logs a debug-level operation completion with duration.
func LogOpDone(ctx context.Context, backend, op string, attrs ...slog.Attr) {
	attrs = append([]slog.Attr{
		slog.String(LogKeyBackend, backend),
		slog.String(LogKeyOperation, op),
	}, attrs...)
	slog.LogAttrs(ctx, slog.LevelDebug, logMsgOpDone, attrs...)
}

// LogAPIError logs a warning-level API error with status code and message.
func LogAPIError(ctx context.Context, backend, method, path string, status int, errMsg string) {
	slog.LogAttrs(ctx, slog.LevelWarn, logMsgAPIError,
		slog.String(LogKeyBackend, backend),
		slog.String(LogKeyOperation, method+" "+path),
		slog.Int(LogKeyStatus, status),
		slog.String(LogKeyError, errMsg),
	)
}
