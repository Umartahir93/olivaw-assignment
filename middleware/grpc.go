// Package middleware holds what runs around every request, for gRPC
// (interceptors) and REST (http.Handler wrappers): logging, turning
// unexpected errors into INTERNAL / 500, and recovering from panics.
package middleware

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryLogging logs every call with its method, code and duration.
//
// Handlers return a gRPC status for every error the contract defines. Any
// other error is unexpected: it is logged with its cause and the caller
// gets INTERNAL without details, so internals never leak.
func UnaryLogging(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		res, err := handler(ctx, req)
		st, cause := toStatus(err)

		attrs := []any{
			"method", info.FullMethod,
			"code", st.Code().String(),
			"duration", time.Since(start),
		}
		switch {
		case cause != nil:
			log.ErrorContext(ctx, "grpc call failed", append(attrs, "error", cause)...)
		case st.Code() != codes.OK:
			log.WarnContext(ctx, "grpc call rejected", append(attrs, "message", st.Message())...)
		default:
			// Every measurement is a call, so successful ones only show at
			// debug level.
			log.DebugContext(ctx, "grpc call", attrs...)
		}
		return res, st.Err()
	}
}

// UnaryRecovery turns a panic in a handler into an error, so one bad request
// does not stop the server. Chain it inside UnaryLogging, which then logs
// the panic and answers INTERNAL.
func UnaryRecovery() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (res any, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic: %v\n%s", r, debug.Stack())
			}
		}()
		return handler(ctx, req)
	}
}

// toStatus returns the status the caller gets for err and, if err was
// unexpected, its cause for the log.
func toStatus(err error) (*status.Status, error) {
	if err == nil {
		return status.New(codes.OK, ""), nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return status.FromContextError(err), nil
	}
	if st, ok := status.FromError(err); ok {
		return st, nil
	}
	return status.New(codes.Internal, "internal error"), err
}
