package database

// CreateUser inserisce un nuovo utente nel database.
// Parametri:
// - username: Il nome utente scelto.
// Ritorna:
// - User: L'oggetto utente creato con il nuovo ID.
// - error: Eventuale errore (es. username duplicato se c'è un vincolo UNIQUE).
func (db *appdbimpl) CreateUser(username string) (User, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	// Esegue l'INSERT nella tabella users. La foto viene inizializzata come stringa vuota.
	res, err := db.c.Exec("INSERT INTO users (username, photo) VALUES (?, '')", username)
	if err != nil {
		return User{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, err
	}
	// Restituisce l'oggetto User popolato.
	return User{ID: id, Username: username}, nil
}

// GetUserByName cerca un utente nel database tramite il suo username.
// Parametri:
// - username: Il nome utente da cercare.
// Ritorna:
// - User: L'oggetto utente trovato.
// - error: Eventuale errore (es. sql.ErrNoRows se non trovato).
func (db *appdbimpl) GetUserByName(username string) (User, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	var u User
	// Esegue la query SELECT. Usa IFNULL(photo, '') per gestire eventuali valori NULL nel campo photo.
	err := db.c.QueryRow("SELECT id, username, IFNULL(photo, '') FROM users WHERE username = ?", username).Scan(&u.ID, &u.Username, &u.Photo)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// GetUserByID recupera un utente tramite il suo ID univoco.
// Parametri:
// - id: L'ID dell'utente.
// Ritorna:
// - User: L'oggetto utente trovato.
// - error: Eventuale errore.
func (db *appdbimpl) GetUserByID(id int64) (User, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	var u User
	// Esegue la query SELECT per ID.
	err := db.c.QueryRow("SELECT id, username, IFNULL(photo, '') FROM users WHERE id = ?", id).Scan(&u.ID, &u.Username, &u.Photo)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// SetUsername aggiorna il nome utente di un utente esistente.
// Parametri:
// - id: L'ID dell'utente.
// - name: Il nuovo username.
// Ritorna:
// - error: Eventuale errore (es. violazione vincolo UNIQUE se il nome è già preso).
func (db *appdbimpl) SetUsername(id int64, name string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.c.Exec("UPDATE users SET username = ? WHERE id = ?", name, id)
	return err
}

// SetPhoto aggiorna la foto profilo di un utente.
// Parametri:
// - id: L'ID dell'utente.
// - photo: La nuova foto (stringa base64 o URL).
// Ritorna:
// - error: Eventuale errore.
func (db *appdbimpl) SetPhoto(id int64, photo string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.c.Exec("UPDATE users SET photo = ? WHERE id = ?", photo, id)
	return err
}

// SearchUsers cerca utenti il cui username contiene la stringa specificata (ricerca parziale).
// Parametri:
// - query: La stringa da cercare.
// Ritorna:
// - []User: Una lista di utenti che corrispondono alla ricerca.
// - error: Eventuale errore.
func (db *appdbimpl) SearchUsers(query string) ([]User, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	// Prepara il pattern per la clausola LIKE (es. "%query%").
	// La ricerca è case-insensitive per default in SQLite per i caratteri ASCII.
	q := "%" + query + "%"

	// Esegue la query.
	rows, err := db.c.Query("SELECT id, username, IFNULL(photo, '') FROM users WHERE username LIKE ?", q)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var users []User
	// Itera sui risultati e costruisce la slice di utenti.
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
