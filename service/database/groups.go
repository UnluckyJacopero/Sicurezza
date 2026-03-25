package database

import (
	"database/sql"
	"errors"
	"time"
)

// CreateGroup crea un nuovo gruppo di chat e vi aggiunge i partecipanti specificati.
// Parametri:
// - name: Nome del gruppo.
// - photo: URL o base64 della foto del gruppo (opzionale).
// - ownerID: ID dell'utente che crea il gruppo (verrà aggiunto automaticamente come membro).
// - members: Slice di ID degli altri utenti da aggiungere al gruppo.
// Ritorna:
// - Conversation: L'oggetto conversazione creato.
// - error: Eventuale errore del database.
func (db *appdbimpl) CreateGroup(name string, photo string, ownerID int64, members []int64) (Conversation, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	// Inizia una transazione SQL per garantire l'atomicità dell'operazione.
	// La creazione del gruppo e l'aggiunta dei membri devono avvenire insieme o fallire insieme.
	tx, err := db.c.Begin()
	if err != nil {
		return Conversation{}, err
	}
	// Assicura il rollback in caso di errore.
	defer func() { _ = tx.Rollback() }()

	now := time.Now()

	// 1. Crea la conversazione nel DB impostando il flag is_group a 1 (true).
	res, err := tx.Exec("INSERT INTO conversations (name, photo, is_group, last_message_timestamp) VALUES (?, ?, 1, ?)", name, photo, now)
	if err != nil {
		return Conversation{}, err
	}
	// Recupera l'ID generato per il nuovo gruppo.
	groupID, err := res.LastInsertId()
	if err != nil {
		return Conversation{}, err
	}

	// 2. Aggiunge il creatore (owner) come primo partecipante nella tabella 'participants'.
	_, err = tx.Exec("INSERT INTO participants (conversation_id, user_id) VALUES (?, ?)", groupID, ownerID)
	if err != nil {
		return Conversation{}, err
	}

	// Prepara lo statement SQL per aggiungere gli altri membri in modo efficiente.
	stmt, err := tx.Prepare("INSERT INTO participants (conversation_id, user_id) VALUES (?, ?)")
	if err != nil {
		return Conversation{}, err
	}
	defer func() { _ = stmt.Close() }()

	// 3. Itera sulla lista dei membri fornita e li aggiunge uno ad uno.
	for _, memberID := range members {
		if memberID == ownerID {
			continue // Salta l'owner se è presente anche nella lista members per evitare duplicati.
		}
		if _, err := stmt.Exec(groupID, memberID); err != nil {
			return Conversation{}, err
		}
	}

	// Conferma la transazione rendendo permanenti le modifiche.
	if err := tx.Commit(); err != nil {
		return Conversation{}, err
	}

	// Restituisce l'oggetto Conversation popolato con i dati del nuovo gruppo.
	return Conversation{
		ID:                   groupID,
		Name:                 name,
		Photo:                photo,
		IsGroup:              true,
		LastMessageTimestamp: now,
	}, nil
}

// AddUsersToGroup aggiunge una lista di utenti a un gruppo esistente.
// Parametri:
// - groupID: ID del gruppo a cui aggiungere i membri.
// - userIDs: Slice di ID degli utenti da aggiungere.
// Ritorna:
// - error: Eventuale errore del database.
func (db *appdbimpl) AddUsersToGroup(groupID int64, userIDs []int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	// Inizia una transazione.
	tx, err := db.c.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Prepara lo statement per l'inserimento.
	stmt, err := tx.Prepare("INSERT INTO participants (conversation_id, user_id) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	// Itera sugli ID degli utenti da aggiungere.
	for _, userID := range userIDs {
		// Verifica preliminare: controlla se l'utente è già membro del gruppo (attivo o cancellato).
		var deleted int
		err := tx.QueryRow("SELECT deleted FROM participants WHERE conversation_id = ? AND user_id = ?", groupID, userID).Scan(&deleted)

		if errors.Is(err, sql.ErrNoRows) {
			// Caso 1: L'utente non è mai stato nel gruppo -> INSERT
			if _, err := stmt.Exec(groupID, userID); err != nil {
				return err
			}
		} else if err == nil {
			// Caso 2: L'utente esiste nella tabella participants.
			if deleted == 1 {
				// Se era stato cancellato (ha lasciato il gruppo), lo riattiviamo -> UPDATE
				_, err = tx.Exec("UPDATE participants SET deleted = 0 WHERE conversation_id = ? AND user_id = ?", groupID, userID)
				if err != nil {
					return err
				}
			}
			// Se deleted == 0, l'utente è già membro attivo: non facciamo nulla.
		} else {
			// Caso 3: Errore imprevisto del DB
			return err
		}
	}

	// Conferma la transazione.
	return tx.Commit()
}

// LeaveGroup gestisce l'uscita di un utente da un gruppo.
// Parametri:
// - groupID: ID del gruppo da lasciare.
// - userID: ID dell'utente che lascia il gruppo.
// Ritorna:
// - error: Eventuale errore del database.
func (db *appdbimpl) LeaveGroup(groupID int64, userID int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	// Inizia una transazione.
	tx, err := db.c.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Check if it is a group
	var isGroup bool
	err = tx.QueryRow("SELECT is_group FROM conversations WHERE id = ?", groupID).Scan(&isGroup)
	if err != nil {
		return err
	}

	if !isGroup {
		// Soft delete for 1-on-1
		_, err = tx.Exec("UPDATE participants SET deleted = 1 WHERE conversation_id = ? AND user_id = ?", groupID, userID)
		if err != nil {
			return err
		}
	} else {
		// 1. Rimozione: Elimina la riga corrispondente nella tabella participants.
		_, err = tx.Exec("DELETE FROM participants WHERE conversation_id = ? AND user_id = ?", groupID, userID)
		if err != nil {
			return err
		}

		// 2. Verifica membri residui: Conta quanti partecipanti sono rimasti nel gruppo.
		var count int
		err = tx.QueryRow("SELECT COUNT(*) FROM participants WHERE conversation_id = ?", groupID).Scan(&count)
		if err != nil {
			return err
		}

		// 3. Pulizia automatica: Se il numero di partecipanti è sceso a 0, il gruppo viene eliminato.
		// Questo evita di avere gruppi "fantasma" nel database senza membri.
		if count == 0 {
			_, err = tx.Exec("DELETE FROM conversations WHERE id = ?", groupID)
			if err != nil {
				return err
			}
		}
	}

	// Conferma la transazione.
	return tx.Commit()
}

// UpdateGroupDetails aggiorna sia il nome che la foto di un gruppo.
// Funzione di utilità generale, non necessariamente legata a un singolo endpoint API.
func (db *appdbimpl) UpdateGroupDetails(groupID int64, name string, photo string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.c.Exec("UPDATE conversations SET name = ?, photo = ? WHERE id = ?", name, photo, groupID)
	return err
}

// SetGroupName aggiorna specificamente il nome di un gruppo.
// Parametri:
// - groupID: ID del gruppo.
// - name: Nuovo nome da assegnare.
// Ritorna:
// - error: Eventuale errore del database.
func (db *appdbimpl) SetGroupName(groupID int64, name string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	// Esegue l'UPDATE assicurandosi che l'ID corrisponda a un gruppo (is_group = 1).
	_, err := db.c.Exec("UPDATE conversations SET name = ? WHERE id = ? AND is_group = 1", name, groupID)
	return err
}

// SetGroupPhoto aggiorna specificamente la foto di un gruppo.
// Parametri:
// - groupID: ID del gruppo.
// - photo: Nuova foto (stringa base64 o URL).
// Ritorna:
// - error: Eventuale errore del database.
func (db *appdbimpl) SetGroupPhoto(groupID int64, photo string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	// Esegue l'UPDATE assicurandosi che l'ID corrisponda a un gruppo (is_group = 1).
	_, err := db.c.Exec("UPDATE conversations SET photo = ? WHERE id = ? AND is_group = 1", photo, groupID)
	return err
}
