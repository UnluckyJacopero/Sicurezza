package database

import "time"

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Photo    string `json:"photo"`
}

type Conversation struct {
	ID                   int64     `json:"id"`
	Name                 string    `json:"name"`
	Photo                string    `json:"photo"`
	IsGroup              bool      `json:"is_group"`
	LastMessageContent   string    `json:"last_message_content"`
	LastMessagePhoto     string    `json:"last_message_photo"`
	LastMessageTimestamp time.Time `json:"last_message_timestamp"`
	LastMessageSenderID  int64     `json:"last_message_sender_id"`
}

type Message struct {
	ID             int64      `json:"id"`
	ConversationID int64      `json:"conversation_id"`
	SenderID       int64      `json:"sender_id"`
	ContentText    string     `json:"content_text"`
	ContentPhoto   string     `json:"content_photo"`
	Timestamp      time.Time  `json:"timestamp"`
	ReplyTo        int64      `json:"reply_to"`
	Forwarded      bool       `json:"forwarded"`
	Read           bool       `json:"read"`
	Reactions      []Reaction `json:"reactions"`
}

type Reaction struct {
	ID     int64  `json:"id"`
	UserID int64  `json:"user_id"`
	Emoji  string `json:"emoji"`
}
