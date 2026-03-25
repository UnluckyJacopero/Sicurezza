package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"wasaTxt/service/api/reqcontext"

	"github.com/julienschmidt/httprouter"
)

// createGroup gestisce la creazione di un nuovo gruppo.
// Endpoint: POST /users/:user/groups
func (rt *_router) createGroup(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Estrae l'ID utente dai parametri dell'URL (ps) e lo converte in int64.
	urlUserID, err := strconv.ParseInt(ps.ByName("user"), 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid user ID")
		return
	}

	// Verifica che l'utente autenticato (ctx.UserID) corrisponda all'utente specificato nell'URL (urlUserID).
	// Se non corrispondono, restituisce un errore di autorizzazione.
	if !rt.checkAuth(w, ctx.UserID, urlUserID) {
		return
	}

	// Inizializza la struttura per contenere i dati di input del gruppo.
	var gInput GroupInput
	// Decodifica il corpo della richiesta JSON nella struttura gInput.
	// Se il JSON non è valido, restituisce un errore 400 Bad Request.
	if err := json.NewDecoder(r.Body).Decode(&gInput); err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}

	// Valida che il nome del gruppo non sia vuoto.
	// Se la lunghezza è inferiore a 1, restituisce un errore 400 Bad Request.
	if len(gInput.Name) < 1 {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Group name empty")
		return
	}

	// Inizializza una slice per contenere gli ID dei membri da aggiungere al gruppo.
	memberIDs := make([]int64, 0)
	// Itera sulla lista di utenti fornita nell'input.
	for _, u := range gInput.Members.Users {
		// Aggiunge l'ID di ogni utente alla slice memberIDs, convertendolo in int64.
		memberIDs = append(memberIDs, int64(u.UserID))
	}

	// Validazione Membri: Verifica che tutti gli utenti specificati esistano nel database.
	exists, err := rt.db.CheckUsersExist(memberIDs)
	if err != nil {
		rt.baseLogger.WithError(err).Error("Error checking users existence")
		rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Internal Server Error")
		return
	}
	if !exists {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "One or more users do not exist")
		return
	}

	// Chiama il metodo del database per creare il gruppo.
	// Passa il nome del gruppo, una stringa vuota per la foto (default), l'ID del creatore e la lista dei membri.
	dbGroup, err := rt.db.CreateGroup(string(gInput.Name), "", ctx.UserID, memberIDs)
	if err != nil {
		// Se c'è un errore nel database, lo logga e restituisce un errore 500 Internal Server Error.
		rt.baseLogger.WithError(err).Error("Database error creating group")
		rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Internal Server Error")
		return
	}

	// Recupera i membri del gruppo appena creato per popolare la risposta
	members, err := rt.db.GetConversationMembers(dbGroup.ID)
	if err != nil {
		rt.baseLogger.WithError(err).Error("Error getting group members")
		rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Internal Server Error")
		return
	}

	apiMembers := make([]User, len(members))
	for i, m := range members {
		apiMembers[i] = User{
			UserID:   ResourceId(m.ID),
			Username: Name(m.Username),
			Photo:    Photo(m.Photo),
		}
	}

	// Costruisce l'oggetto di risposta API con i dati del gruppo appena creato.
	apiGroup := Group{
		GroupID:   ResourceId(dbGroup.ID), // ID del gruppo generato dal DB
		GroupName: Name(dbGroup.Name),     // Nome del gruppo
		Users:     Users{Users: apiMembers},
	}

	// Notifica i partecipanti via WebSocket
	rt.notifyConversation(dbGroup.ID, WebSocketMessage{
		Type: "CONVERSATION_UPDATED",
		Payload: map[string]interface{}{
			"conversation_id": dbGroup.ID,
			"action":          "group_created",
		},
	})

	// Invia la risposta JSON con codice 201 Created e l'oggetto apiGroup.
	rt.sendJSONResponse(w, http.StatusCreated, apiGroup)
}

// addToGroup permette di aggiungere nuovi membri a un gruppo esistente.
// Endpoint: PUT /users/:user/groups/:group
func (rt *_router) addToGroup(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Estrae l'ID del gruppo e l'ID dell'utente dai parametri dell'URL.
	groupID, err := strconv.ParseInt(ps.ByName("group"), 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid group ID")
		return
	}
	userID, err := strconv.ParseInt(ps.ByName("user"), 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid user ID")
		return
	}

	// Verifica che l'utente autenticato sia autorizzato a compiere questa azione.
	if !rt.checkAuth(w, ctx.UserID, userID) {
		return
	}

	// Verifica che l'utente sia membro del gruppo
	if !rt.checkConversationMember(w, groupID, userID) {
		return
	}

	// Inizializza la struttura per contenere la lista di utenti da aggiungere.
	var users Users
	// Decodifica il corpo della richiesta JSON.
	if err := json.NewDecoder(r.Body).Decode(&users); err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}

	// Prepara una slice di int64 per gli ID degli utenti da aggiungere.
	var idsToAdd []int64
	// Itera sulla lista di utenti ricevuta e estrae gli ID.
	for _, u := range users.Users {
		idsToAdd = append(idsToAdd, int64(u.UserID))
	}

	// Chiama il metodo del database per aggiungere gli utenti specificati al gruppo.
	err = rt.db.AddUsersToGroup(groupID, idsToAdd)
	if err != nil {
		// Se si verifica un errore (es. utente già presente, gruppo non trovato), logga l'errore e risponde con 500.
		rt.baseLogger.WithError(err).Error("Error adding users to group")
		rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Could not add users")
		return
	}

	// Notifica i partecipanti via WebSocket
	rt.notifyConversation(groupID, WebSocketMessage{
		Type: "CONVERSATION_UPDATED",
		Payload: map[string]interface{}{
			"conversation_id": groupID,
			"action":          "members_added",
		},
	})

	// Se tutto va a buon fine, restituisce 200 OK e un messaggio di conferma.
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`"Action successfully completed"`))
}

// leaveGroup permette a un utente di abbandonare un gruppo.
// Endpoint: DELETE /users/:user/groups/:group/members/:member
// L'implementazione attuale usa l'ID utente dal contesto di autenticazione.
func (rt *_router) leaveGroup(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Estrae l'ID del gruppo dai parametri dell'URL.
	groupID, err := strconv.ParseInt(ps.ByName("group"), 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid group ID")
		return
	}

	// Verifica che l'utente sia autenticato (ID presente nel contesto).
	// Questo controllo è ridondante se il middleware di autenticazione è già attivo, ma aggiunge sicurezza.
	if ctx.UserID == 0 {
		rt.sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "Login required")
		return
	}

	// Verifica che l'utente sia membro del gruppo
	if !rt.checkConversationMember(w, groupID, ctx.UserID) {
		return
	}

	// Chiama il metodo del database per rimuovere l'utente dal gruppo.
	// Se l'utente è l'ultimo membro, il gruppo potrebbe essere eliminato (logica gestita nel DB).
	err = rt.db.LeaveGroup(groupID, ctx.UserID)
	if err != nil {
		// Logga l'errore e restituisce 500 Internal Server Error.
		rt.baseLogger.WithError(err).Error("Error leaving group")
		rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Internal Error")
		return
	}

	// Notifica i partecipanti via WebSocket
	rt.notifyConversation(groupID, WebSocketMessage{
		Type: "CONVERSATION_UPDATED",
		Payload: map[string]interface{}{
			"conversation_id": groupID,
			"action":          "member_left",
		},
	})

	// Restituisce 200 OK per indicare successo.
	w.WriteHeader(http.StatusOK)
}

// setGroupName permette di modificare il nome di un gruppo esistente.
// Endpoint: PUT /users/:user/groups/:group/name
func (rt *_router) setGroupName(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Estrae gli ID dai parametri dell'URL.
	groupID, err := strconv.ParseInt(ps.ByName("group"), 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid group ID")
		return
	}
	userID, err := strconv.ParseInt(ps.ByName("user"), 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid user ID")
		return
	}

	// Verifica l'autorizzazione dell'utente.
	if !rt.checkAuth(w, ctx.UserID, userID) {
		return
	}

	// Verifica che l'utente sia membro del gruppo
	if !rt.checkConversationMember(w, groupID, userID) {
		return
	}

	// Decodifica il nuovo nome dal body della richiesta.
	var input GroupNameInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}

	// Aggiorna il nome nel database.
	err = rt.db.SetGroupName(groupID, string(input.Name))
	if err != nil {
		rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Error updating")
		return
	}

	// Notifica i partecipanti via WebSocket
	rt.notifyConversation(groupID, WebSocketMessage{
		Type: "CONVERSATION_UPDATED",
		Payload: map[string]interface{}{
			"conversation_id": groupID,
			"action":          "group_renamed",
		},
	})

	// Restituisce 200 OK.
	w.WriteHeader(http.StatusOK)
}

// setGroupPhoto permette di modificare la foto di un gruppo esistente.
// Endpoint: PUT /users/:user/groups/:group/photo
func (rt *_router) setGroupPhoto(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Estrae gli ID dai parametri dell'URL.
	groupID, err := strconv.ParseInt(ps.ByName("group"), 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid group ID")
		return
	}
	userID, err := strconv.ParseInt(ps.ByName("user"), 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid user ID")
		return
	}

	// Verifica l'autorizzazione dell'utente.
	if !rt.checkAuth(w, ctx.UserID, userID) {
		return
	}

	// Verifica che l'utente sia membro del gruppo
	if !rt.checkConversationMember(w, groupID, userID) {
		return
	}

	// Decodifica la nuova foto (base64 o URL) dal body della richiesta.
	var input PhotoInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}

	// Aggiorna la foto nel database.
	err = rt.db.SetGroupPhoto(groupID, string(input.Photo))
	if err != nil {
		rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Error updating")
		return
	}

	// Notifica i partecipanti via WebSocket
	rt.notifyConversation(groupID, WebSocketMessage{
		Type: "CONVERSATION_UPDATED",
		Payload: map[string]interface{}{
			"conversation_id": groupID,
			"action":          "group_photo_updated",
		},
	})

	// Restituisce 200 OK.
	w.WriteHeader(http.StatusOK)
}
