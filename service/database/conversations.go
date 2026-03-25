package database

import (
	"database/sql"
	"errors"
	"time"
)

// CreateOneOnOneConversation gestisce la creazione o il recupero di una conversazione privata (1-a-1) tra due utenti.
// Parametri:
// - user1: ID del primo utente (solitamente chi inizia la conversazione).
// - user2: ID del secondo utente (interlocutore).
// Ritorna:
// - Conversation: L'oggetto conversazione (nuovo o esistente).
// - error: Eventuale errore del database.
func (db *appdbimpl) CreateOneOnOneConversation(user1 int64, user2 int64) (Conversation, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	// 1. Verifica esistenza: Controlla se esiste già una conversazione privata tra i due utenti.
	// La query cerca una conversazione che:
	// - Non è un gruppo (is_group = 0).
	// - Ha user1 tra i partecipanti.
	// - Ha user2 tra i partecipanti.
	// Nota: Questa logica assume che esista al massimo una conversazione 1-a-1 tra due utenti specifici.
	query := `
		SELECT c.id, c.name, c.photo, c.is_group, c.last_message_content, c.last_message_photo, c.last_message_timestamp, c.last_message_sender_id
		FROM conversations c
		JOIN participants p1 ON c.id = p1.conversation_id
		JOIN participants p2 ON c.id = p2.conversation_id
		WHERE c.is_group = 0 AND p1.user_id = ? AND p2.user_id = ?
	`
	var c Conversation
	// Variabili per gestire i valori NULL che possono arrivare dal DB (es. se non ci sono messaggi o foto).
	var lastMsgContent sql.NullString
	var lastMsgPhoto sql.NullString
	var lastMsgTime sql.NullTime
	var lastMsgSenderID sql.NullInt64
	var name sql.NullString
	var photo sql.NullString

	// Esegue la query.
	err := db.c.QueryRow(query, user1, user2).Scan(&c.ID, &name, &photo, &c.IsGroup, &lastMsgContent, &lastMsgPhoto, &lastMsgTime, &lastMsgSenderID)
	if err == nil {
		// Caso: Conversazione trovata.
		// Assicura che la conversazione sia visibile per l'utente che la richiede (user1).
		_, _ = db.c.Exec("UPDATE participants SET deleted = 0 WHERE conversation_id = ? AND user_id = ?", c.ID, user1)

		// Popola i campi della struct gestendo i tipi Nullable.
		c.Name = name.String
		c.Photo = photo.String
		c.LastMessageContent = lastMsgContent.String
		c.LastMessagePhoto = lastMsgPhoto.String
		c.LastMessageTimestamp = lastMsgTime.Time
		c.LastMessageSenderID = lastMsgSenderID.Int64
		return c, nil
	}
	// Se l'errore è diverso da "nessuna riga trovata", è un errore reale del DB.
	if !errors.Is(err, sql.ErrNoRows) {
		return Conversation{}, err
	}

	// 2. Creazione nuova conversazione: Se non esiste, crea una nuova conversazione privata.
	// Inizia una transazione per garantire l'integrità dei dati.
	tx, err := db.c.Begin()
	if err != nil {
		return Conversation{}, err
	}
	// Gestione rollback in caso di errore.
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Insert Conversazione
	res, err := tx.Exec("INSERT INTO conversations (is_group, last_message_timestamp) VALUES (0, ?)", time.Now())
	if err != nil {
		return Conversation{}, err
	}
	groupID, err := res.LastInsertId()
	if err != nil {
		return Conversation{}, err
	}

	// Insert Partecipanti
	_, err = tx.Exec("INSERT INTO participants (conversation_id, user_id) VALUES (?, ?)", groupID, user1)
	if err != nil {
		return Conversation{}, err
	}
	_, err = tx.Exec("INSERT INTO participants (conversation_id, user_id) VALUES (?, ?)", groupID, user2)
	if err != nil {
		return Conversation{}, err
	}

	// Commit finale
	if err = tx.Commit(); err != nil {
		return Conversation{}, err
	}

	return Conversation{
		ID:                   groupID,
		IsGroup:              false,
		LastMessageTimestamp: time.Now(),
	}, nil
}

// IsUserInConversation verifica se l'utente specificato è un partecipante della conversazione.
func (db *appdbimpl) IsUserInConversation(conversationID int64, userID int64) (bool, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	var count int
	// Esegue la query per contare le righe corrispondenti.
	err := db.c.QueryRow("SELECT COUNT(*) FROM participants WHERE conversation_id = ? AND user_id = ?", conversationID, userID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetMyConversations recupera tutte le conversazioni a cui partecipa un determinato utente.
// Parametri:
// - userID: ID dell'utente di cui recuperare le conversazioni.
// Ritorna:
// - []Conversation: Slice contenente le conversazioni trovate.
// - error: Eventuale errore del database.
func (db *appdbimpl) GetMyConversations(userID int64) ([]Conversation, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	// Esegue la query per selezionare le conversazioni.
	// Utilizza una JOIN con la tabella participants per filtrare solo quelle dell'utente specificato.
	query := `
		SELECT
			c.id,
			c.is_group,
			CASE
				WHEN c.is_group = 1 THEN c.name
				ELSE u.username
			END AS display_name,
			CASE
				WHEN c.is_group = 1 THEN c.photo
				ELSE u.photo
			END AS display_photo,
			c.last_message_content,
			c.last_message_photo,
			c.last_message_timestamp,
			c.last_message_sender_id
		FROM conversations c
		JOIN participants p ON c.id = p.conversation_id
		LEFT JOIN participants p2 ON c.id = p2.conversation_id AND p2.user_id != ? AND c.is_group = 0
		LEFT JOIN users u ON p2.user_id = u.id
		WHERE p.user_id = ? AND p.deleted = 0
		ORDER BY c.last_message_timestamp DESC
	`
	rows, err := db.c.Query(query, userID, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var convs []Conversation
	for rows.Next() {
		var c Conversation
		// Variabili temporanee per gestire eventuali NULL dal database
		var lastMsgContent sql.NullString
		var lastMsgPhoto sql.NullString
		var displayName sql.NullString
		var displayPhoto sql.NullString
		var lastMsgSenderID sql.NullInt64

		if err := rows.Scan(
			&c.ID,
			&c.IsGroup,
			&displayName,
			&displayPhoto,
			&lastMsgContent,
			&lastMsgPhoto,
			&c.LastMessageTimestamp,
			&lastMsgSenderID,
		); err != nil {
			return nil, err
		}

		c.Name = displayName.String
		c.Photo = displayPhoto.String
		c.LastMessageContent = lastMsgContent.String
		c.LastMessagePhoto = lastMsgPhoto.String
		c.LastMessageSenderID = lastMsgSenderID.Int64

		convs = append(convs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return convs, nil
}

// GetConversation recupera i dettagli di una singola conversazione dato il suo ID.
// Parametri:
// - conversationID: ID univoco della conversazione.
// Ritorna:
// - Conversation: Oggetto contenente i dettagli della conversazione.
// - error: Eventuale errore (es. sql.ErrNoRows se non trovata).
func (db *appdbimpl) GetConversation(conversationID int64, requestingUserID int64) (Conversation, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	// Esegue la query per ottenere i dati della conversazione specifica.
	query := `
		SELECT
			c.id,
			c.is_group,
			CASE
				WHEN c.is_group = 1 THEN c.name
				ELSE u.username
			END AS display_name,
			CASE
				WHEN c.is_group = 1 THEN c.photo
				ELSE u.photo
			END AS display_photo
		FROM conversations c
		LEFT JOIN participants p2 ON c.id = p2.conversation_id AND p2.user_id != ? AND c.is_group = 0
		LEFT JOIN users u ON p2.user_id = u.id
		WHERE c.id = ?
	`
	var c Conversation
	var displayName sql.NullString
	var displayPhoto sql.NullString

	err := db.c.QueryRow(query, requestingUserID, conversationID).Scan(
		&c.ID,
		&c.IsGroup,
		&displayName,
		&displayPhoto,
	)

	if err != nil {
		return c, err
	}

	c.Name = displayName.String
	c.Photo = displayPhoto.String
	return c, nil
}

// GetConversationMembers recupera la lista degli utenti partecipanti a una conversazione.
func (db *appdbimpl) GetConversationMembers(conversationID int64) ([]User, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	rows, err := db.c.Query(`
		SELECT u.id, u.username, IFNULL(u.photo, '')
		FROM users u
		JOIN participants p ON u.id = p.user_id
		WHERE p.conversation_id = ?`, conversationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Photo); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}
