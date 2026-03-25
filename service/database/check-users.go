package database

import (
	"fmt"
	"strings"
)

// CheckUsersExist verifica se una lista di ID utente esiste nel database.
// Restituisce true se TUTTI gli utenti esistono, false altrimenti.
func (db *appdbimpl) CheckUsersExist(userIDs []int64) (bool, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if len(userIDs) == 0 {
		return true, nil
	}

	// Costruisce la query dinamica con il numero corretto di placeholder "?".
	placeholders := make([]string, len(userIDs))
	args := make([]interface{}, len(userIDs))
	for i, id := range userIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf("SELECT COUNT(*) FROM users WHERE id IN (%s)", strings.Join(placeholders, ","))

	var count int
	err := db.c.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return false, err
	}

	// Verifica se il numero di utenti trovati corrisponde al numero di ID cercati.
	// Nota: Questo assume che userIDs non contenga duplicati. Se li contiene,
	// bisognerebbe fare SELECT COUNT(DISTINCT id) e rimuovere duplicati da userIDs.
	// Per semplicità qui assumiamo input pulito o accettiamo che count <= len(userIDs).
	// Se count == len(userIDs) siamo sicuri che tutti esistono (se userIDs è unico).
	// Se userIDs ha duplicati, count sarà < len(userIDs) se usiamo IN, perché IN collassa i duplicati nel match.
	// Quindi COUNT(*) su IN restituirà il numero di match unici.
	// Dobbiamo confrontarlo con il numero di ID unici in input.

	uniqueIDs := make(map[int64]bool)
	for _, id := range userIDs {
		uniqueIDs[id] = true
	}

	return count == len(uniqueIDs), nil
}
