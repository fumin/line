package line

// WebhookBody is the top-level payload LINE POSTs to a webhook URL.
type WebhookBody struct {
	Destination string  `json:"destination"`
	Events      []Event `json:"events"`
}

// Event is a single LINE webhook event (message, join, leave, follow, ...).
type Event struct {
	Type       string   `json:"type"`
	Timestamp  int64    `json:"timestamp"`
	Source     Source   `json:"source"`
	ReplyToken string   `json:"replyToken"`
	Message    *Message `json:"message"`
}

// Source identifies where an event came from.
type Source struct {
	Type    string `json:"type"` // "user", "group", or "room"
	UserID  string `json:"userId"`
	GroupID string `json:"groupId"`
	RoomID  string `json:"roomId"`
}

// Message is the payload of a "message" event.
type Message struct {
	Type string `json:"type"` // "text", "image", "sticker", "video", ...
	ID   string `json:"id"`
	Text string `json:"text"`
}
