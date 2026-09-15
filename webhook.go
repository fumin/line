package line

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/fumin/line/util"
)

// WebhookHandler handles LINE Messaging API webhook requests: it verifies
// the request signature, then logs every message event to disk.
type WebhookHandler struct {
	ChannelSecret string
	Names         *NameCache
	Writer        *LogWriter
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	if !h.validSignature(r.Header.Get("X-Line-Signature"), body) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var payload WebhookBody
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	for _, ev := range payload.Events {
		if err := h.handleEvent(ev); err != nil {
			log.Printf("failed to handle event: %v", err)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) validSignature(header string, body []byte) bool {
	if header == "" || h.ChannelSecret == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(h.ChannelSecret))
	mac.Write(body)
	expected := mac.Sum(nil)

	got, err := base64.StdEncoding.DecodeString(header)
	if err != nil {
		return false
	}
	return hmac.Equal(expected, got)
}

func (h *WebhookHandler) handleEvent(ev Event) error {
	chatName := h.chatName(ev.Source)
	senderName := h.Names.UserName(ev.Source.UserID, ev.Source.GroupID, ev.Source.RoomID)
	content := describe(ev)

	t := time.UnixMilli(ev.Timestamp).In(util.TaipeiTZ)
	line := fmt.Sprintf("[%s] %s: %s", t.Format("15:04:05"), senderName, content)

	return h.Writer.Append(t, chatName, line)
}

// chatName returns the name used for the log file: the group/room name for
// group and room chats, or the other person's name for a direct 1:1 chat.
func (h *WebhookHandler) chatName(src Source) string {
	switch src.Type {
	case "group":
		return h.Names.GroupName(src.GroupID)
	case "room":
		return "room_" + src.RoomID
	default: // "user": direct 1:1 chat
		return h.Names.UserName(src.UserID, "", "")
	}
}

// describe renders a short human-readable summary of an event's content.
func describe(ev Event) string {
	if ev.Type != "message" || ev.Message == nil {
		return "[" + ev.Type + " event]"
	}

	switch ev.Message.Type {
	case "text":
		return ev.Message.Text
	case "":
		return "[empty message]"
	default:
		return "[" + ev.Message.Type + "]"
	}
}
