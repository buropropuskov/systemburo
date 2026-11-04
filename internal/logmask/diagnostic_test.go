package logmask

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

type diagnosticPanicError struct{}

func (diagnosticPanicError) Error() string {
	panic("error text must not be formatted")
}

func TestDiagnosticReplaceAttrErrorDoesNotCallError(t *testing.T) {
	attr := ReplaceAttr(nil, slog.Any("failure", diagnosticPanicError{}))
	if !strings.Contains(attr.Value.String(), "diagnosticPanicError/other") {
		t.Fatal("typed error diagnostic missing")
	}
}

func TestDiagnosticReplaceAttrSensitiveKeys(t *testing.T) {
	for _, key := range []string{
		"USERNAME", "email", "Phone", "current_password", "accessToken",
		"client-secret", "Cookie", "Authorization", "endpoint_host",
		"DSN", "RawQuery", "request_body", "sql", "requestURI", "raw_url",
		"template_path", "last_error", "error",
		"last_name", "FirstName", "full-name", "patronymic", "name",
	} {
		t.Run(key, func(t *testing.T) {
			got := ReplaceAttr(nil, slog.String(key, "PRIVATE-VALUE"))
			if got.Value.String() != MaskedValue {
				t.Fatal("sensitive value was retained")
			}
		})
	}
}

func TestDiagnosticReplaceAttrJSONWithGroups(t *testing.T) {
	var out bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&out, &slog.HandlerOptions{ReplaceAttr: ReplaceAttr}))
	log.With(slog.Any("error", fmt.Errorf("PRIVATE-WRAPPER: %w", context.Canceled))).
		WithGroup("operation").
		With("access_token", "PRIVATE-TOKEN").
		Info("fixed diagnostic",
			slog.Group("response", "route", "/users/:id", "status", 503, "request_id", "request-123", "error_type", "*pgconn.PgError", "rows", 7),
			slog.Group("body", "arbitrary", "PRIVATE-BODY"),
			slog.String("url", "PRIVATE-URL"))
	for _, marker := range []string{"PRIVATE-WRAPPER", "PRIVATE-TOKEN", "PRIVATE-BODY", "PRIVATE-URL"} {
		if strings.Contains(out.String(), marker) {
			t.Fatal("private marker appeared in JSON diagnostics")
		}
	}
	for _, safe := range []string{"canceled", "/users/:id", "request-123", "*pgconn.PgError", `"rows":7`, `"status":503`} {
		if !strings.Contains(out.String(), safe) {
			t.Fatalf("safe diagnostic %q missing", safe)
		}
	}
	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal("diagnostic output is not JSON")
	}

	out.Reset()
	log.WithGroup("Authorization").With("arbitrary", "PRIVATE-AUTH").
		Info("fixed diagnostic", "count", 9)
	if strings.Contains(out.String(), "PRIVATE-AUTH") {
		t.Fatal("WithGroup-sensitive value appeared in output")
	}
}
