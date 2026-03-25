package database

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// SendMessage invia un messaggio e aggiorna l'anteprima della conversazione
func (db *appdbimpl) SendMessage(conversationID int64, senderID int64, text string, photo string, replyTo *int64, forwarded bool) (Message, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	// Inizia una transazione SQL.
	// È fondamentale usare una transazione perché dobbiamo eseguire due operazioni atomiche:
	// 1. Inserire il messaggio nella tabella 'messages'.
	// 2. Aggiornare i campi 'last_message_*' nella tabella 'conversations'.
	tx, err := db.c.Begin()
	if err != nil {
		return Message{}, err
	}
	// Assicura il rollback della transazione in caso di errore o panic prima del commit.
	defer func() { _ = tx.Rollback() }()

	// 1. Inserimento del messaggio:
	// Esegue l'INSERT nella tabella messages con i dati forniti e il timestamp corrente.
	now := time.Now()
	res, err := tx.Exec("INSERT INTO messages (conversation_id, sender_id, content_text, content_photo, timestamp, reply_to, forwarded) VALUES (?, ?, ?, ?, ?, ?, ?)", conversationID, senderID, text, photo, now, replyTo, forwarded)

	// Controlla eventuali errori durante l'inserimento.
	if err != nil {
		return Message{}, err
	}

	// Recupera l'ID univoco generato per il nuovo messaggio.
	id, err := res.LastInsertId()
	if err != nil {
		return Message{}, err
	}

	// 2. Aggiornamento dell'anteprima della conversazione:
	// Aggiorna la tabella conversations con il contenuto e il timestamp dell'ultimo messaggio.
	// Questo permette di visualizzare rapidamente l'ultimo messaggio nella lista delle conversazioni
	// senza dover eseguire query complesse (JOIN o subquery) ogni volta.
	_, err = tx.Exec("UPDATE conversations SET last_message_content = ?, last_message_photo = ?, last_message_timestamp = ?, last_message_sender_id = ? WHERE id = ?", text, photo, now, senderID, conversationID)
	if err != nil {
		return Message{}, err
	}

	// 3. Resurrezione conversazione:
	// Imposta deleted = 0 per tutti i partecipanti, in modo che la conversazione riappaia se era stata cancellata.
	_, err = tx.Exec("UPDATE participants SET deleted = 0 WHERE conversation_id = ?", conversationID)
	if err != nil {
		return Message{}, err
	}

	// Conferma la transazione rendendo permanenti le modifiche.
	if err := tx.Commit(); err != nil {
		return Message{}, err
	}

	replyToVal := int64(0)
	if replyTo != nil {
		replyToVal = *replyTo
	}

	// Restituisce l'oggetto Message completo.
	return Message{
		ID:             id,
		ConversationID: conversationID,
		SenderID:       senderID,
		ContentText:    text,
		ContentPhoto:   photo,
		Timestamp:      now,
		ReplyTo:        replyToVal,
		Forwarded:      forwarded,
		Reactions:      []Reaction{},
	}, nil
}

// GetMessages recupera una lista paginata di messaggi appartenenti a una specifica conversazione.
// Parametri:
// - conversationID: ID della conversazione.
// - limit: Numero massimo di messaggi da restituire.
// - offset: Numero di messaggi da saltare (per la paginazione).
// Ritorna:
// - []Message: Slice di messaggi recuperati.
// - error: Eventuale errore del database.
func (db *appdbimpl) GetMessages(conversationID int64, limit int, offset int) ([]Message, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	// Esegue la query per selezionare i messaggi.
	// - WHERE conversation_id = ?: Filtra per conversazione.
	// - ORDER BY timestamp DESC: Ordina dal più recente al più vecchio.
	// - LIMIT ? OFFSET ?: Applica la paginazione.
	rows, err := db.c.Query(`
		SELECT id, conversation_id, sender_id, content_text, content_photo, timestamp, reply_to, forwarded, read
		FROM messages 
		WHERE conversation_id = ? 
		ORDER BY timestamp DESC 
		LIMIT ? OFFSET ?`, conversationID, limit, offset)
	if err != nil {
		return nil, err
	}
	// Assicura la chiusura del cursore.
	defer func() { _ = rows.Close() }()

	var msgs []Message
	// Itera sui risultati.
	for rows.Next() {
		var m Message
		// Variabile per gestire il campo reply_to che può essere NULL.
		var replyTo sql.NullInt64

		// Scansiona i dati della riga corrente.
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.ContentText, &m.ContentPhoto, &m.Timestamp, &replyTo, &m.Forwarded, &m.Read); err != nil {
			return nil, err
		}

		// Se reply_to è valido (non NULL), lo assegna al messaggio.
		if replyTo.Valid {
			m.ReplyTo = replyTo.Int64
		}

		// Inizializza la slice delle reazioni vuota.
		m.Reactions = []Reaction{}

		// Aggiunge il messaggio alla lista.
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	_ = rows.Close() // Chiude il cursore prima di procedere a fetchare le reazioni.

	if len(msgs) == 0 {
		return msgs, nil
	}

	// Ottiene tutte le reazioni per questi messaggi in una sola query
	msgIDs := make([]interface{}, len(msgs))
	for i, m := range msgs {
		msgIDs[i] = m.ID
	}

	placeholders := strings.Repeat("?,", len(msgIDs))
	placeholders = placeholders[:len(placeholders)-1] // remove last comma

	query := fmt.Sprintf("SELECT id, message_id, user_id, emoji FROM reactions WHERE message_id IN (%s)", placeholders)

	rRows, err := db.c.Query(query, msgIDs...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rRows.Close() }()

	reactionsMap := make(map[int64][]Reaction)
	for rRows.Next() {
		var r Reaction
		var msgID int64
		if err := rRows.Scan(&r.ID, &msgID, &r.UserID, &r.Emoji); err != nil {
			return nil, err
		}
		reactionsMap[msgID] = append(reactionsMap[msgID], r)
	}
	if err := rRows.Err(); err != nil {
		return nil, err
	}

	// Assegna le reazioni ai messaggi
	for i := range msgs {
		if reactions, ok := reactionsMap[msgs[i].ID]; ok {
			msgs[i].Reactions = reactions
		}
	}

	return msgs, nil
}

// GetMessage recupera un singolo messaggio per ID
func (db *appdbimpl) GetMessage(messageID int64) (Message, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	var m Message
	var replyTo sql.NullInt64
	err := db.c.QueryRow("SELECT id, conversation_id, sender_id, content_text, content_photo, timestamp, reply_to, forwarded, read FROM messages WHERE id = ?", messageID).Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.ContentText, &m.ContentPhoto, &m.Timestamp, &replyTo, &m.Forwarded, &m.Read)
	if err != nil {
		return Message{}, err
	}
	if replyTo.Valid {
		m.ReplyTo = replyTo.Int64
	}

	reactions, err := db.getReactions(m.ID)
	if err != nil {
		return Message{}, err
	}
	m.Reactions = reactions

	return m, nil
}

// MarkConversationAsRead segna come letti i messaggi per l'utente corrente e aggiorna lo stato globale se tutti hanno letto.
func (db *appdbimpl) MarkConversationAsRead(conversationID int64, readerID int64) (bool, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	// 1. Registra la lettura per l'utente corrente (readerID)
	// Inserisce un record in message_reads per tutti i messaggi non letti di questa conversazione
	// che NON sono stati inviati dall'utente stesso.
	res, err := db.c.Exec(`
		INSERT OR IGNORE INTO message_reads (message_id, user_id)
		SELECT id, ? FROM messages 
		WHERE conversation_id = ? AND sender_id != ? AND read = 0
	`, readerID, conversationID, readerID)
	if err != nil {
		return false, err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	// Se non abbiamo segnato nessun nuovo messaggio come letto, non serve fare altro
	if rowsAffected == 0 {
		return false, nil
	}

	// 2. Aggiorna lo stato globale 'read' dei messaggi se TUTTI i partecipanti hanno letto.
	// Recupera i messaggi della conversazione che sono ancora segnati come non letti (read=0).
	// ORDER BY id DESC è fondamentale per l'ottimizzazione: controlliamo prima i messaggi più recenti.
	rows, err := db.c.Query("SELECT id, sender_id FROM messages WHERE conversation_id = ? AND read = 0 ORDER BY id DESC", conversationID)
	if err != nil {
		return true, err // Ritorniamo true perché comunque abbiamo segnato dei messaggi come letti localmente
	}
	defer func() { _ = rows.Close() }()

	var msgIDs []int64
	var senderIDs []int64
	for rows.Next() {
		var mid, sid int64
		if err := rows.Scan(&mid, &sid); err != nil {
			continue
		}
		msgIDs = append(msgIDs, mid)
		senderIDs = append(senderIDs, sid)
	}
	if err := rows.Err(); err != nil {
		return true, err
	}
	_ = rows.Close()

	// Per ogni messaggio non letto, controlla se il numero di letture corrisponde al numero di partecipanti (escluso il mittente).
	// Ottimizzazione: Calcoliamo il numero di partecipanti una volta sola per mittente.
	// Inoltre, se un messaggio recente di un mittente è "letto da tutti", allora anche tutti i suoi messaggi precedenti
	// (che sono stati letti per forza se si legge sequenzialmente) possono essere considerati letti.

	// Cache dei conteggi partecipanti per sender_id
	participantCounts := make(map[int64]int)
	// Cache per sapere se abbiamo già trovato un messaggio "completamente letto" per questo sender
	senderFullyRead := make(map[int64]bool)

	for i, mid := range msgIDs {
		sid := senderIDs[i]

		// Se abbiamo già stabilito che un messaggio più recente di questo mittente è stato letto da tutti,
		// allora anche questo (che è più vecchio) deve essere considerato letto.
		if senderFullyRead[sid] {
			_, _ = db.c.Exec("UPDATE messages SET read = 1 WHERE id = ?", mid)
			continue
		}

		// Recupera il conteggio partecipanti dalla cache o dal DB
		participantCount, ok := participantCounts[sid]
		if !ok {
			err := db.c.QueryRow("SELECT COUNT(*) FROM participants WHERE conversation_id = ? AND user_id != ? AND deleted = 0", conversationID, sid).Scan(&participantCount)
			if err != nil {
				return true, err
			}
			participantCounts[sid] = participantCount
		}

		// Conta quante persone hanno letto questo messaggio
		var readCount int
		err = db.c.QueryRow("SELECT COUNT(*) FROM message_reads WHERE message_id = ?", mid).Scan(&readCount)
		if err != nil {
			return true, err
		}

		// Se tutti hanno letto, segna il messaggio come letto globalmente
		if readCount >= participantCount {
			_, _ = db.c.Exec("UPDATE messages SET read = 1 WHERE id = ?", mid)
			// Ottimizzazione: segna che per questo sender i messaggi più vecchi sono sicuramente letti
			senderFullyRead[sid] = true
		}
	}

	return true, nil
}

// DeleteMessage elimina un messaggio dal database e aggiorna l'anteprima della conversazione se necessario.
// Parametri:
// - messageID: ID del messaggio da eliminare.
// Ritorna:
// - error: Eventuale errore del database.
func (db *appdbimpl) DeleteMessage(messageID int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	// Inizia una transazione per garantire l'atomicità dell'operazione.
	tx, err := db.c.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// 1. Recupero ID conversazione: Prima di cancellare, dobbiamo sapere a quale conversazione appartiene il messaggio.
	// Questo serve per poter aggiornare successivamente l'anteprima della conversazione.
	var conversationID int64
	err = tx.QueryRow("SELECT conversation_id FROM messages WHERE id = ?", messageID).Scan(&conversationID)
	if err != nil {
		return err // Restituisce errore se il messaggio non viene trovato.
	}

	// 2. Cancellazione messaggio: Elimina la riga corrispondente dalla tabella messages.
	_, err = tx.Exec("DELETE FROM messages WHERE id = ?", messageID)
	if err != nil {
		return err
	}

	// 3. Ricalcolo anteprima: Dopo la cancellazione, dobbiamo trovare qual è il "nuovo" ultimo messaggio.
	// Seleziona il messaggio più recente rimasto nella conversazione.
	var lastMsgContent sql.NullString
	var lastMsgPhoto sql.NullString
	var lastMsgTime sql.NullTime
	var lastMsgSenderID sql.NullInt64

	err = tx.QueryRow(`
		SELECT content_text, content_photo, timestamp, sender_id
		FROM messages 
		WHERE conversation_id = ? 
		ORDER BY timestamp DESC 
		LIMIT 1`, conversationID).Scan(&lastMsgContent, &lastMsgPhoto, &lastMsgTime, &lastMsgSenderID)

	// 4. Aggiornamento conversazione:
	if errors.Is(err, sql.ErrNoRows) {
		// Se non ci sono più messaggi, resetta i campi last_message_* a NULL o valori vuoti.
		_, err = tx.Exec("UPDATE conversations SET last_message_content = '', last_message_photo = '', last_message_timestamp = ?, last_message_sender_id = 0 WHERE id = ?", time.Now(), conversationID)
	} else if err == nil {
		// Se esiste un ultimo messaggio, aggiorna i campi con i suoi valori.
		_, err = tx.Exec("UPDATE conversations SET last_message_content = ?, last_message_photo = ?, last_message_timestamp = ?, last_message_sender_id = ? WHERE id = ?", lastMsgContent.String, lastMsgPhoto.String, lastMsgTime.Time, lastMsgSenderID.Int64, conversationID)
	} else {
		// Caso: Errore imprevisto
		return err
	}

	if err != nil {
		return err
	}

	// Conferma la transazione.
	return tx.Commit()
}
