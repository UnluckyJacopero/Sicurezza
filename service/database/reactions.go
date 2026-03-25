package database

import (
	"database/sql"
	"errors"
)

var ErrReactionNotOwned = errors.New("reaction not found or not owned by user")

// AddReaction aggiunge una reazione (emoji) a un messaggio nel database.
// Parametri:
// - messageID: ID del messaggio a cui aggiungere la reazione.
// - userID: ID dell'utente che aggiunge la reazione.
// - emoji: La stringa dell'emoji.
// Ritorna:
// - int64: L'ID della reazione appena creata.
// - error: Eventuale errore del database.
func (db *appdbimpl) AddReaction(messageID int64, userID int64, emoji string) (int64, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	// Verifica che l'utente non stia reagendo al proprio messaggio.
	var senderID int64
	err := db.c.QueryRow("SELECT sender_id FROM messages WHERE id = ?", messageID).Scan(&senderID)
	if err != nil {
		return 0, err
	}
	if senderID == userID {
		return 0, errors.New("cannot react to own message")
	}

	// Esegue l'INSERT nella tabella reactions.
	res, err := db.c.Exec("INSERT INTO reactions (message_id, user_id, emoji) VALUES (?, ?, ?)", messageID, userID, emoji)
	if err != nil {
		return 0, err
	}
	// Restituisce l'ID generato.
	return res.LastInsertId()
}

// RemoveReaction rimuove una reazione dal database.
// Parametri:
// - reactionID: ID della reazione da rimuovere.
// - userID: ID dell'utente che richiede la rimozione (per verifica proprietà).
// Ritorna:
// - error: Eventuale errore (incluso se la reazione non esiste o non appartiene all'utente).
func (db *appdbimpl) RemoveReaction(reactionID int64, userID int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	// Esegue la DELETE verificando sia l'ID della reazione che l'ID dell'utente.
	res, err := db.c.Exec("DELETE FROM reactions WHERE id = ? AND user_id = ?", reactionID, userID)
	if err != nil {
		return err
	}
	// Controlla quante righe sono state effettivamente cancellate.
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	// Se nessuna riga è stata cancellata, significa che la reazione non esisteva o non apparteneva all'utente.
	if affected == 0 {
		return ErrReactionNotOwned
	}
	return nil
}

// GetReactions recupera le reazioni per un messaggio (Public wrapper with lock)
func (db *appdbimpl) GetReactions(messageID int64) ([]Reaction, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.getReactions(messageID)
}

// getReactions recupera le reazioni per un messaggio (Internal without lock)
func (db *appdbimpl) getReactions(messageID int64) ([]Reaction, error) {
	// Esegue la query per selezionare le reazioni associate al messaggio.
	rows, err := db.c.Query("SELECT id, user_id, emoji FROM reactions WHERE message_id = ?", messageID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	// Itera sui risultati e costruisce la slice di reazioni.
	var reactions []Reaction
	for rows.Next() {
		var r Reaction
		if err := rows.Scan(&r.ID, &r.UserID, &r.Emoji); err != nil {
			return nil, err
		}
		reactions = append(reactions, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return reactions, nil
}

// ToggleReaction gestisce l'aggiunta, rimozione o cambio di reazione
func (db *appdbimpl) ToggleReaction(messageID int64, userID int64, emoji string) (string, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Controlla che l'utente non stia reagendo al proprio messaggio.
	var senderID int64
	err := db.c.QueryRow("SELECT sender_id FROM messages WHERE id = ?", messageID).Scan(&senderID)
	if err != nil {
		return "", err
	}
	if senderID == userID {
		return "", errors.New("cannot react to own message")
	}

	// Verifica se l'utente ha già una reazione a questo messaggio.
	var existingID int64
	var existingEmoji string
	err = db.c.QueryRow("SELECT id, emoji FROM reactions WHERE message_id = ? AND user_id = ?", messageID, userID).Scan(&existingID, &existingEmoji)

	if errors.Is(err, sql.ErrNoRows) {
		// Caso A: Nessuna reazione esistente, aggiungi nuova reazione
		_, err = db.c.Exec("INSERT INTO reactions (message_id, user_id, emoji) VALUES (?, ?, ?)", messageID, userID, emoji)
		if err != nil {
			return "", err
		}
		return "added", nil
	} else if err != nil {
		return "", err
	}

	// Caso B e C: Reazione esistente trovata
	if existingEmoji == emoji {
		// Stessa emoji -> Rimuovi reazione
		_, err = db.c.Exec("DELETE FROM reactions WHERE id = ?", existingID)
		if err != nil {
			return "", err
		}
		return "removed", nil
	} else {
		// Emoji diversa -> Aggiorna (Cambia)
		_, err = db.c.Exec("UPDATE reactions SET emoji = ? WHERE id = ?", emoji, existingID)
		if err != nil {
			return "", err
		}
		return "updated", nil
	}
}
