package api

// ResourceId rappresenta un numero di identificazione generale (es. utenti, gruppi, conversazioni)
type ResourceId int64

// Name rappresenta il nome di un utente o di un gruppo
type Name string

// Photo rappresenta una foto codificata in base64
type Photo string

// Timestamp rappresenta una stringa temporale (formato: date-time)
type Timestamp string

// LoginInput rappresenta l'input per il login
type LoginInput struct {
	Name Name `json:"name"`
}

// UsernameInput rappresenta l'input per cambiare il nome utente
type UsernameInput struct {
	Name Name `json:"name"`
}

// PhotoInput rappresenta l'input per caricare una foto
type PhotoInput struct {
	Photo Photo `json:"photo"`
}

// User rappresenta il profilo utente
type User struct {
	Username Name       `json:"username"`
	Photo    Photo      `json:"photo,omitempty"`
	UserID   ResourceId `json:"user_id"`
}

// Users rappresenta una lista di utenti
type Users struct {
	Users []User `json:"users"`
}

// Members rappresenta una lista di ID utenti (usato per input gruppi)
type Members []ResourceId

// Text rappresenta il contenuto testuale
type Text string

// ContentText rappresenta il corpo di un messaggio di testo
type ContentText struct {
	Text Text `json:"text"`
}

// ContentPhoto rappresenta il corpo di un messaggio con foto
type ContentPhoto struct {
	Photo   Photo `json:"photo"`
	Caption Text  `json:"caption,omitempty"`
}

// BodyMessage rappresenta il contenuto del messaggio (testo o foto)
// Go non supporta nativamente "oneOf" nelle struct facilmente, quindi usiamo puntatori opzionali per gestire entrambi i casi.
type BodyMessage struct {
	Text    *Text  `json:"text,omitempty"`
	Photo   *Photo `json:"photo,omitempty"`
	Caption *Text  `json:"caption,omitempty"`
}

// MessageInput rappresenta l'input per inviare un nuovo messaggio
type MessageInput struct {
	Body    BodyMessage `json:"body"`
	ReplyTo *ResourceId `json:"reply_to,omitempty"`
}

// Message rappresenta un messaggio in una conversazione
type Message struct {
	MessageID      ResourceId  `json:"message_id"`
	ConversationID ResourceId  `json:"conversation_id"`
	Body           BodyMessage `json:"body"`
	SenderID       ResourceId  `json:"sender_id"`
	SenderName     Name        `json:"sender_name,omitempty"`
	SendTime       Timestamp   `json:"send_time"`
	ReplyTo        *ResourceId `json:"reply_to,omitempty"`
	Status         string      `json:"status,omitempty"` // sent, received, read
	Forwarded      bool        `json:"forwarded,omitempty"`
	Reactions      []Reaction  `json:"reactions,omitempty"`
}

type Reaction struct {
	ReactionID ResourceId `json:"reaction_id"`
	UserID     ResourceId `json:"user_id"`
	Emoticon   string     `json:"emoticon"`
}

// NewReactionInput rappresenta l'input per creare una nuova reazione
type NewReactionInput struct {
	Emoticon string `json:"emoticon"`
}

// Group rappresenta un gruppo
type Group struct {
	GroupName Name       `json:"groupname"`
	Photo     Photo      `json:"photo,omitempty"`
	GroupID   ResourceId `json:"group_id"`
	Users     Users      `json:"members"`
}

// GroupInput rappresenta l'input per creare un nuovo gruppo
type GroupInput struct {
	Name    Name  `json:"name"`
	Members Users `json:"members"`
}

// GroupNameInput rappresenta l'input per cambiare il nome di un gruppo
type GroupNameInput struct {
	Name Name `json:"name"`
}

// Conversation rappresenta un oggetto conversazione dettagliato
type Conversation struct {
	ConversationID ResourceId `json:"conversation_id"`
	Name           Name       `json:"name,omitempty"`
	Photo          Photo      `json:"photo,omitempty"`
	IsGroup        bool       `json:"is_group"`
	Participants   Users      `json:"participants,omitempty"`
	LastMsg        *Message   `json:"last_msg,omitempty"`
	Messages       []Message  `json:"messages,omitempty"`
}

// ConversationSummary rappresenta un riepilogo di una conversazione
type ConversationSummary struct {
	ConversationID ResourceId `json:"conversation_id"`
	Name           Name       `json:"name,omitempty"`
	Photo          Photo      `json:"photo,omitempty"`
	IsGroup        bool       `json:"is_group"`
	LastMsg        *Message   `json:"last_msg,omitempty"`
}

// ConversationCollection rappresenta una lista di conversazioni
type ConversationCollection struct {
	Conversations []ConversationSummary `json:"conversations"`
}

// ErrorItem rappresenta una descrizione dell'errore
type ErrorItem struct {
	Code     string `json:"code"`
	Reason   string `json:"reason"`
	Message  string `json:"message,omitempty"`
	MoreInfo string `json:"more_info,omitempty"`
}

// Error rappresenta la risposta di errore dell'API
type Error struct {
	Trace  string      `json:"trace,omitempty"`
	Errors []ErrorItem `json:"errors"`
}
