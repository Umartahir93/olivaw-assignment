//go:build unit

package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/umartahir93/olivaw-assignment/middleware"
)

// httpLogLine is the part of a JSON log line the tests check.
type httpLogLine struct {
	Level  string `json:"level"`
	Status int    `json:"status"`
}

func TestHTTPLogging(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantLog httpLogLine
	}{
		{
			name:    "success without an explicit status",
			handler: func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{}")) },
			wantLog: httpLogLine{
				Level:  "INFO",
				Status: http.StatusOK,
			},
		},
		{
			name:    "client error",
			handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) },
			wantLog: httpLogLine{
				Level:  "WARN",
				Status: http.StatusNotFound,
			},
		},
		{
			name:    "server error",
			handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
			wantLog: httpLogLine{
				Level:  "ERROR",
				Status: http.StatusInternalServerError,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			rec := httptest.NewRecorder()

			middleware.HTTPLogging(newLogger(&buf), tt.handler).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/gateways", nil))

			if rec.Code != tt.wantLog.Status {
				t.Errorf("response status = %d, want %d", rec.Code, tt.wantLog.Status)
			}
			var got httpLogLine
			decodeLog(t, &buf, &got)
			if got != tt.wantLog {
				t.Errorf("log = %+v, want %+v", got, tt.wantLog)
			}
		})
	}
}

func TestHTTPRecovery(t *testing.T) {
	var buf bytes.Buffer
	rec := httptest.NewRecorder()
	panics := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") })

	middleware.HTTPRecovery(newLogger(&buf), panics).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/gateways", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	var got logLine
	decodeLog(t, &buf, &got)
	if got.Level != "ERROR" || !strings.HasPrefix(got.Error, "panic: boom") {
		t.Errorf("log = %+v, want an error with the panic", got)
	}
}
