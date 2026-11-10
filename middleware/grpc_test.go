//go:build unit

package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/umartahir93/olivaw-assignment/middleware"
)

var info = &grpc.UnaryServerInfo{FullMethod: "/gateway.v1.RobotService/ReportMeasurement"}

// logLine is the part of a JSON log line the tests check.
type logLine struct {
	Level string `json:"level"`
	Code  string `json:"code"`
	Error string `json:"error"`
}

// newLogger returns a logger that writes JSON lines into buf, debug included.
func newLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// readLog decodes the single line in buf.
func readLog(t *testing.T, buf *bytes.Buffer) logLine {
	t.Helper()
	var line logLine
	decodeLog(t, buf, &line)
	return line
}

// decodeLog decodes the single JSON line in buf into v.
func decodeLog(t *testing.T, buf *bytes.Buffer, v any) {
	t.Helper()
	if err := json.Unmarshal(buf.Bytes(), v); err != nil {
		t.Fatalf("decode log line %q: %v", buf.String(), err)
	}
}

func TestUnaryLogging(t *testing.T) {
	tests := []struct {
		name        string
		handlerErr  error
		wantCode    codes.Code
		wantMessage string
		wantLog     logLine
	}{
		{
			name:       "success",
			handlerErr: nil,
			wantCode:   codes.OK,
			wantLog: logLine{
				Level: "DEBUG",
				Code:  "OK",
			},
		},
		{
			name:        "contract error is passed on",
			handlerErr:  status.Error(codes.InvalidArgument, "robot_id must be a UUID"),
			wantCode:    codes.InvalidArgument,
			wantMessage: "robot_id must be a UUID",
			wantLog: logLine{
				Level: "WARN",
				Code:  "InvalidArgument",
			},
		},
		{
			name:        "unexpected error is logged and hidden",
			handlerErr:  errors.New("database down"),
			wantCode:    codes.Internal,
			wantMessage: "internal error",
			wantLog: logLine{
				Level: "ERROR",
				Code:  "Internal",
				Error: "database down",
			},
		},
		{
			name:        "cancelled call",
			handlerErr:  context.Canceled,
			wantCode:    codes.Canceled,
			wantMessage: context.Canceled.Error(),
			wantLog: logLine{
				Level: "WARN",
				Code:  "Canceled",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			handler := func(context.Context, any) (any, error) { return nil, tt.handlerErr }

			_, err := middleware.UnaryLogging(newLogger(&buf))(context.Background(), nil, info, handler)

			st, _ := status.FromError(err)
			if st.Code() != tt.wantCode || st.Message() != tt.wantMessage {
				t.Errorf("status = %s %q, want %s %q", st.Code(), st.Message(), tt.wantCode, tt.wantMessage)
			}
			if got := readLog(t, &buf); got != tt.wantLog {
				t.Errorf("log = %+v, want %+v", got, tt.wantLog)
			}
		})
	}
}

func TestUnaryRecovery(t *testing.T) {
	var buf bytes.Buffer
	panics := func(context.Context, any) (any, error) { panic("boom") }

	// The interceptors in the order the server chains them: logging outside,
	// recovery inside.
	logging := middleware.UnaryLogging(newLogger(&buf))
	recovery := middleware.UnaryRecovery()
	_, err := logging(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		return recovery(ctx, req, info, panics)
	})

	if code := status.Code(err); code != codes.Internal {
		t.Errorf("code = %s, want %s", code, codes.Internal)
	}
	line := readLog(t, &buf)
	if line.Level != "ERROR" || !strings.HasPrefix(line.Error, "panic: boom") {
		t.Errorf("log = %+v, want an error with the panic", line)
	}
}
