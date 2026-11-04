package services

import (
	"encoding/json"
	"log/slog"
	"strings"
)

// webpush-go v1.4.0: 4096 record bytes minus 16 AEAD tag, 86 header and 1 delimiter.
const pushPayloadMaxBytes = 3993
const pushPayloadMaxTypeLen = 50 // Notification.Type's existing varchar(50)

func preparePushMessage(payload PushPayload) ([]byte, bool) {
	// Stop counting as soon as the bound is exceeded, even for huge input.
	count := 0
	for range payload.Type {
		count++
		if count > pushPayloadMaxTypeLen {
			return nil, false
		}
	}
	if payload.ApplicationID != nil {
		id := *payload.ApplicationID
		payload.ApplicationID = &id
	}
	body := buildPushMessage(payload)
	return body, len(body) <= pushPayloadMaxBytes
}

func buildPushMessage(payload PushPayload) []byte {
	payload.Title = truncatePushText(payload.Title, pushPayloadMaxTitleLen)
	payload.Message = truncatePushText(payload.Message, pushPayloadMaxMessageLen)
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("push: не удалось сериализовать payload", "error", err)
		return []byte("{}")
	}
	return body
}

// Bound the scan/allocation to the retained prefix rather than converting the
// entire input to []rune. Preserve the existing extra ellipsis and JSON's UTF-8
// replacement behavior for short strings containing invalid bytes.
func truncatePushText(s string, max int) string {
	var prefix strings.Builder
	count := 0
	for _, r := range s {
		if count == max {
			return prefix.String() + "…"
		}
		prefix.WriteRune(r)
		count++
	}
	return strings.Clone(s)
}
