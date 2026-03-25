package api

import (
	"encoding/json"
	"github.com/julienschmidt/httprouter"
	"net/http"
	"strconv"
	"unicode/utf8"
	"wasaTxt/service/api/reqcontext"
)

// findUsers permette di cercare utenti nel sistema utilizzando una stringa di ricerca parziale (username).
// Endpoint: GET /users
// Parametri Query:
// - found_user: Stringa da cercare all'interno degli username (es. "mar" troverà "mario", "marco", etc.).
func (rt *_router) findUsers(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Estrae il parametro di query 'found_user'.
	query := r.URL.Query().Get("found_user")

	// Validazione: il parametro di ricerca è obbligatorio e non può essere vuoto.
	if len(query) < 1 {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Query parameter 'found_user' is required")
		return
	}

	// Esegue la ricerca nel database invocando il metodo SearchUsers.
	// Restituisce una lista di utenti che corrispondono al criterio di ricerca.
	dbUsers, err := rt.db.SearchUsers(query)
	if err != nil {
		// Logga l'errore e restituisce 500 Internal Server Error.
		ctx.Logger.WithError(err).Error("Search users failed")
		rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Internal Server Error")
		return
	}

	// Inizializza la struttura di risposta API (Users) che contiene una slice di User.
	apiUsers := Users{Users: make([]User, len(dbUsers))}

	// Itera sugli utenti restituiti dal database e li converte nel formato API.
	for i, u := range dbUsers {
		apiUsers.Users[i] = User{
			UserID:   ResourceId(u.ID), // ID utente
			Username: Name(u.Username), // Username
			Photo:    Photo(u.Photo),   // Foto profilo
		}
	}

	// Restituisce la lista degli utenti trovati in formato JSON con codice 200 OK.
	w.Header().Set("Content-Type", "application/json")
	rt.sendJSONResponse(w, http.StatusOK, apiUsers)
}

// setMyUserName permette all'utente autenticato di aggiornare il proprio username.
// Endpoint: PUT /users/:user/username
// Parametri URL:
// - user: ID dell'utente che vuole cambiare username.
// Body: Oggetto JSON contenente il nuovo username.
func (rt *_router) setMyUserName(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Estrae l'ID utente dai parametri dell'URL.
	idStr := ps.ByName("user")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid ID")
		return
	}

	// Verifica di sicurezza: controlla che l'ID nell'URL corrisponda all'ID dell'utente autenticato (dal token).
	// Un utente non può cambiare lo username di un altro utente.
	if id != ctx.UserID {
		rt.sendErrorResponse(w, http.StatusForbidden, "forbidden", "Forbidden")
		return
	}

	// Decodifica il body della richiesta per ottenere il nuovo username.
	var input UsernameInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}

	// Validazione: lo username deve avere una lunghezza compresa tra 3 e 30 caratteri.
	if utf8.RuneCountInString(string(input.Name)) < 3 || utf8.RuneCountInString(string(input.Name)) > 30 {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Username must be between 3 and 30 chars")
		return
	}

	// Invoca il metodo del database per aggiornare lo username.
	err = rt.db.SetUsername(id, string(input.Name))
	if err != nil {
		// Se l'aggiornamento fallisce (es. username già in uso), logga l'errore e restituisce 409 Conflict.
		ctx.Logger.WithError(err).Error("Set username failed")
		rt.sendErrorResponse(w, http.StatusConflict, "conflict", "Conflict or Server Error")
		return
	}

	// Notifica i partecipanti via WebSocket
	convs, err := rt.db.GetMyConversations(id)
	if err == nil {
		for _, c := range convs {
			rt.notifyConversation(c.ID, WebSocketMessage{
				Type: "CONVERSATION_UPDATED",
				Payload: map[string]interface{}{
					"conversation_id": c.ID,
				},
			})
		}
	} else {
		ctx.Logger.WithError(err).Error("Failed to get conversations for notification")
	}

	// Costruisce l'oggetto User aggiornato da restituire nella risposta.
	// Nota: qui non stiamo recuperando la foto aggiornata dal DB, ma solo confermando il nuovo nome.
	userAPI := User{
		UserID:   ResourceId(id),
		Username: input.Name,
	}

	// Restituisce la risposta JSON con il nuovo username e codice 200 OK.
	w.Header().Set("Content-Type", "application/json") // Aggiunto Content-Type
	rt.sendJSONResponse(w, http.StatusOK, userAPI)
}

// setMyPhoto permette all'utente autenticato di aggiornare la propria foto profilo.
// Endpoint: PUT /users/:user/photo
// Parametri URL:
// - user: ID dell'utente che vuole cambiare la foto.
// Body: Oggetto JSON contenente la nuova foto (es. in formato base64 o URL).
func (rt *_router) setMyPhoto(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Estrae l'ID utente dai parametri dell'URL.
	idStr := ps.ByName("user")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid ID")
		return
	}

	// Verifica di sicurezza: controlla che l'ID nell'URL corrisponda all'ID dell'utente autenticato.
	// Impedisce la modifica della foto di altri utenti.
	if id != ctx.UserID {
		rt.sendErrorResponse(w, http.StatusForbidden, "forbidden", "Forbidden")
		return
	}

	// Decodifica il body della richiesta per ottenere i dati della nuova foto.
	var input PhotoInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}

	// Invoca il metodo del database per aggiornare la foto dell'utente.
	err = rt.db.SetPhoto(id, string(input.Photo))
	if err != nil {
		// Logga l'errore e restituisce 500 Internal Server Error.
		ctx.Logger.WithError(err).Error("Set photo failed")
		rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Internal Server Error")
		return
	}

	// Notifica tutte le conversazioni a cui partecipa l'utente
	convs, err := rt.db.GetMyConversations(id)
	if err == nil {
		for _, c := range convs {
			rt.notifyConversation(c.ID, WebSocketMessage{
				Type: "CONVERSATION_UPDATED",
				Payload: map[string]interface{}{
					"conversation_id": c.ID,
				},
			})
		}
	} else {
		ctx.Logger.WithError(err).Error("Failed to get conversations for notification")
	}

	// Restituisce 200 OK per indicare che l'operazione è andata a buon fine.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"message": "The photo was uploaded successfully"}`))
}
