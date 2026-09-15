package line

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
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
	Client        *Client
	Names         *NameCache
	Writer        *LogWriter
}

// maxWebhookBodyBytes limits the size of LINE's webhook payloads.
const maxWebhookBodyBytes = 5 << 20 // 5 MiB

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBodyBytes)
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

	t := time.UnixMilli(ev.Timestamp).In(util.TaipeiTZ)
	content := h.describe(ev, chatName, t)

	// The log file is HTML (see LogWriter), so senderName - a LINE display
	// name, entirely up to the sender - must be escaped; content is either
	// already-escaped text or HTML we built ourselves (e.g. an image <a>).
	line := fmt.Sprintf("[%s] %s: %s", t.Format(time.TimeOnly), html.EscapeString(senderName), content)

	return h.Writer.Append(t, chatName, line)
}

func hasEventPrefix(b []byte) bool {
	if len(b) < 10 {
		return false
	}
	if _, err := time.Parse("["+time.TimeOnly+"]", string(b[:10])); err != nil {
		return false
	}
	return true
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
// Image messages are downloaded and saved in a sub-folder (named after
// chatName) of the day's log directory, with the returned summary an HTML
// <a> tag linking to the saved file.
func (h *WebhookHandler) describe(ev Event, chatName string, t time.Time) string {
	if ev.Type != "message" || ev.Message == nil {
		return "[" + ev.Type + " event]"
	}

	switch ev.Message.Type {
	case "text":
		return html.EscapeString(ev.Message.Text)
	case "image":
		return h.describeImage(ev.Message.ID, chatName, t)
	case "":
		return "[empty message]"
	default:
		return "[" + html.EscapeString(ev.Message.Type) + "]"
	}
}

// describeImage downloads an image message's content via the Messaging API
// and saves it under a chatName sub-folder of the day's log directory,
// returning an HTML <a> tag linking to the saved file via ServeContent.
func (h *WebhookHandler) describeImage(messageID, chatName string, t time.Time) string {
	if h.Client == nil {
		return "[image]"
	}

	data, contentType, err := h.Client.MessageContent(messageID)
	if err != nil {
		log.Printf("failed to download image %s: %v", messageID, err)
		return "[image]"
	}

	name := imageFileName(t, contentType)
	relPath, err := h.Writer.SaveFile(t, chatName, name, data)
	if err != nil {
		log.Printf("failed to save image %s: %v", messageID, err)
		return "[image]"
	}

	return imageAnchor(relPath, name)
}

// imageFileName names a saved image after the time it was sent, e.g.
// "image_20260915_225150.jpg" for one sent at 22:51:50 on 2026-09-15.
func imageFileName(t time.Time, contentType string) string {
	return "image_" + t.Format("20060102_150405") + imageExt(contentType)
}

// imageExt returns a file extension for an image message's content type,
// defaulting to ".jpg" since that's what LINE sends in practice.
func imageExt(contentType string) string {
	switch contentType {
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	default:
		return ".jpg"
	}
}

// imageAnchor renders an HTML <a> tag linking to a saved image through
// ServeContent. relPath is the image's path relative to the log directory,
// as returned by LogWriter.SaveFile (e.g.
// "2026-09-15/mychat/image_20260915_225150.jpg").
func imageAnchor(relPath, name string) string {
	return fmt.Sprintf(`<a href="%s">%s</a>`, serveContentURL(relPath), html.EscapeString(name))
}
