/*
Package database is the middleware between the app database and the code. All data (de)serialization (save/load) from a
persistent database are handled here. Database specific logic should never escape this package.

To use this package you need to apply migrations to the database if needed/wanted, connect to it (using the database
data source name from config), and then initialize an instance of AppDatabase from the DB connection.

For example, this code adds a parameter in `webapi` executable for the database data source name (add it to the
main.WebAPIConfiguration structure):

	DB struct {
		Filename string `conf:""`
	}

This is an example on how to migrate the DB and connect to it:

	// Avvia Database
	logger.Println("inizializzazione supporto database")
	db, err := sql.Open("sqlite3", "./foo.db")
	if err != nil {
		logger.WithError(err).Error("errore apertura DB SQLite")
		return fmt.Errorf("apertura SQLite: %w", err)
	}
	defer func() {
		logger.Debug("arresto database")
		_ = db.Close()
	}()

Then you can initialize the AppDatabase and pass it to the api package.
*/
package database

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// AppDatabase è l'interfaccia di alto livello per il DB
type AppDatabase interface {
	// Gestione Utenti
	CreateUser(username string) (User, error)
	GetUserByName(username string) (User, error)
	GetUserByID(id int64) (User, error)
	SetUsername(id int64, name string) error
	SetPhoto(id int64, photo string) error
	SearchUsers(query string) ([]User, error)
	CheckUsersExist(userIDs []int64) (bool, error)

	// IsUserInConversation controlla se un utente è membro di una conversazione (gruppo o 1-a-1)
	IsUserInConversation(conversationID int64, userID int64) (bool, error)

	GetConversationMembers(conversationID int64) ([]User, error)

	// Gestione Conversazioni & Gruppi
	CreateGroup(name string, photo string, ownerID int64, members []int64) (Conversation, error)
	CreateOneOnOneConversation(user1 int64, user2 int64) (Conversation, error)
	GetMyConversations(userID int64) ([]Conversation, error)
	GetConversation(conversationID int64, requestingUserID int64) (Conversation, error)
	AddUsersToGroup(groupID int64, userIDs []int64) error
	LeaveGroup(groupID int64, userID int64) error
	UpdateGroupDetails(groupID int64, name string, photo string) error
	SetGroupName(groupID int64, name string) error
	SetGroupPhoto(groupID int64, photo string) error

	// Gestione Messaggi
	SendMessage(conversationID int64, senderID int64, text string, photo string, replyTo *int64, forwarded bool) (Message, error)
	GetMessages(conversationID int64, limit int, offset int) ([]Message, error)
	GetMessage(messageID int64) (Message, error)
	DeleteMessage(messageID int64) error
	MarkConversationAsRead(conversationID int64, readerID int64) (bool, error)

	// Gestione Reazioni
	AddReaction(messageID int64, userID int64, emoji string) (int64, error)
	RemoveReaction(reactionID int64, userID int64) error
	GetReactions(messageID int64) ([]Reaction, error)
	ToggleReaction(messageID int64, userID int64, emoji string) (string, error)

	Ping() error
}

type appdbimpl struct {
	c  *sql.DB
	mu sync.Mutex
}

// New restituisce una nuova istanza di AppDatabase basata sulla connessione SQLite `db`.
func New(db *sql.DB) (AppDatabase, error) {
	if db == nil {
		return nil, errors.New("il database è richiesto per creare un AppDatabase")
	}

	// Abilita le foreign keys per SQLite
	_, err := db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		return nil, err
	}

	// Schema Creation
	tableCreationQueries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			photo TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS conversations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT,
			photo TEXT,
			is_group BOOLEAN NOT NULL DEFAULT 0,
			last_message_content TEXT,
			last_message_photo TEXT,
			last_message_timestamp DATETIME,
			last_message_sender_id INTEGER DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS participants (
			conversation_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			PRIMARY KEY (conversation_id, user_id),
			FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id INTEGER NOT NULL,
			sender_id INTEGER NOT NULL,
			content_text TEXT,
			content_photo TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			reply_to INTEGER,
			forwarded BOOLEAN NOT NULL DEFAULT 0,
			read BOOLEAN NOT NULL DEFAULT 0,
			FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
			FOREIGN KEY (sender_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS reactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			message_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			emoji TEXT NOT NULL,
			FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE (message_id, user_id)
		);`,
		`CREATE TABLE IF NOT EXISTS message_reads (
			message_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			read_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (message_id, user_id),
			FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
	}

	for _, query := range tableCreationQueries {
		if _, err := db.Exec(query); err != nil {
			return nil, fmt.Errorf("errore creazione struttura database: %w", err)
		}
	}

	if _, err := db.Exec("ALTER TABLE messages ADD COLUMN forwarded BOOLEAN NOT NULL DEFAULT 0"); err != nil {
		if !strings.Contains(err.Error(), "duplicate column name") {
			return nil, fmt.Errorf("errore aggiornamento schema messages: %w", err)
		}
	}

	if _, err := db.Exec("ALTER TABLE participants ADD COLUMN deleted BOOLEAN NOT NULL DEFAULT 0"); err != nil {
		if !strings.Contains(err.Error(), "duplicate column name") {
			return nil, fmt.Errorf("errore aggiornamento schema participants: %w", err)
		}
	}

	if _, err := db.Exec("ALTER TABLE messages ADD COLUMN read BOOLEAN NOT NULL DEFAULT 0"); err != nil {
		if !strings.Contains(err.Error(), "duplicate column name") {
			return nil, fmt.Errorf("errore aggiornamento schema messages (read): %w", err)
		}
	}

	if _, err := db.Exec("ALTER TABLE conversations ADD COLUMN last_message_sender_id INTEGER DEFAULT 0"); err != nil {
		if !strings.Contains(err.Error(), "duplicate column name") {
			return nil, fmt.Errorf("errore aggiornamento schema conversations (last_message_sender_id): %w", err)
		}
	}

	// Creazione Indici per Performance
	// È fondamentale indicizzare le colonne usate nelle clausole WHERE e ORDER BY
	indexCreationQueries := []string{
		"CREATE INDEX IF NOT EXISTS idx_messages_conversation_id ON messages(conversation_id);",
		"CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);",
		"CREATE INDEX IF NOT EXISTS idx_messages_conv_timestamp ON messages(conversation_id, timestamp DESC);",
		"CREATE INDEX IF NOT EXISTS idx_participants_user_id ON participants(user_id);",
		"CREATE INDEX IF NOT EXISTS idx_reactions_message_id ON reactions(message_id);",
		"CREATE INDEX IF NOT EXISTS idx_message_reads_message_id ON message_reads(message_id);",
	}

	for _, query := range indexCreationQueries {
		if _, err := db.Exec(query); err != nil {
			return nil, fmt.Errorf("errore creazione indici: %w", err)
		}
	}

	return &appdbimpl{
		c: db,
	}, nil
}

func (db *appdbimpl) Ping() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.c.Ping()
}
