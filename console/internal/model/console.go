package model

// PushMessage represents a push message (matches Python console_push_store)
type PushMessage struct {
	ID        string  `json:"id"`
	Text      string  `json:"text"`
	Timestamp float64 `json:"-"` // internal use, not exposed to JSON
	SessionID string  `json:"-"` // internal use, not exposed to JSON
}

// PushMessagesResponse represents push messages response
type PushMessagesResponse struct {
	Messages []*PushMessage `json:"messages"`
}

// ConsoleMessage represents a simplified message for API response
type ConsoleMessage struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}
