package logmask

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"
)

// ReplaceAttr filters structured diagnostic attributes. Free-form messages and
// values under unrelated keys still need a safe logging contract at their source.
func ReplaceAttr(groups []string, attr slog.Attr) slog.Attr {
	if err, ok := attr.Value.Any().(error); ok {
		attr.Value = slog.StringValue(fmt.Sprintf("%T/%s", err, diagnosticErrorClass(err)))
		return attr
	}
	if attr.Value.Kind() == slog.KindGroup {
		children := attr.Value.Group()
		filtered := make([]slog.Attr, len(children))
		nested := append(append([]string(nil), groups...), attr.Key)
		for i, child := range children {
			filtered[i] = ReplaceAttr(nested, child)
		}
		attr.Value = slog.GroupValue(filtered...)
		return attr
	}
	for _, group := range groups {
		if sensitiveDiagnosticKey(group) {
			attr.Value = slog.StringValue(MaskedValue)
			return attr
		}
	}
	if sensitiveDiagnosticKey(attr.Key) {
		attr.Value = slog.StringValue(MaskedValue)
	}
	return attr
}

func sensitiveDiagnosticKey(key string) bool {
	// Collapse separators so request_uri, requestURI and Request-URI agree.
	key = strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, key)
	switch key {
	case "route", "routetemplate", "status", "requestid", "errortype", "errorclass":
		return false
	case "auth", "error", "err":
		return true
	}
	for _, sensitive := range []string{
		"name", "patronymic", "email", "phone", "password", "token", "secret", "cookie",
		"authorization", "endpoint", "dsn", "query", "body", "sql", "uri", "url", "path",
		"errormessage", "lasterror",
	} {
		if strings.Contains(key, sensitive) {
			return true
		}
	}
	return false
}

func diagnosticErrorClass(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline"
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, sql.ErrNoRows):
		return "not_found"
	case errors.Is(err, fs.ErrPermission):
		return "permission"
	default:
		return "other"
	}
}
