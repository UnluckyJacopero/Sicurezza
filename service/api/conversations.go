package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"wasaTxt/service/api/reqcontext"

	"github.com/julienschmidt/httprouter"
)

// getMyConversations recupera e restituisce l'elenco di tutte le conversazioni a cui partecipa l'utente specificato.
// Endpoint: GET /users/:user/conversations
// Parametri:
// - user: ID dell'utente di cui si vogliono recuperare le conversazioni.
// Risposta:
// - 200 OK: Restituisce un oggetto JSON contenente la lista delle conversazioni (ConversationCollection).
// - 400 Bad Request: Se l'ID utente non è valido.
// - 403 Forbidden: Se l'utente richiedente non è autorizzato a visualizzare le conversazioni dell'utente target.
// - 500 Internal Server Error: Se si verifica un errore nel recupero dei dati dal database.
func (rt *_router) getMyConversations(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Estrae l'ID utente dai parametri del percorso (URL) e lo converte in un intero a 64 bit.
	userID, err := strconv.ParseInt(ps.ByName("user"), 10, 64)
	// Se la conversione fallisce (es. l'ID non è numerico), restituisce un errore 400 Bad Request.
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid user ID")
		return
	}

	// Verifica che l'utente autenticato (ctx.UserID) sia lo stesso dell'utente richiesto (userID).
	// Questo garantisce che un utente possa vedere solo le proprie conversazioni.
	if !rt.checkAuth(w, ctx.UserID, userID) {
		return
	}

	// Invoca il metodo del database per ottenere la lista delle conversazioni associate all'utente.
	convs, err := rt.db.GetMyConversations(userID)
	// Se si verifica un errore durante l'interazione con il database, logga l'errore e restituisce 500.
	if err != nil {
		rt.baseLogger.WithError(err).Error("Error getting conversations")
		rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Internal Server Error")
		return
	}

	// Inizializza la struttura di risposta API per la collezione di conversazioni.
	apiConvs := ConversationCollection{
		Conversations: make([]ConversationSummary, len(convs)),
	}

	// Itera su ogni conversazione recuperata dal database per convertirla nel formato API.
	for i, c := range convs {
		// Converte il contenuto dell'ultimo messaggio e l'eventuale foto in tipi specifici dell'API.
		txt := Text(c.LastMessageContent)
		photo := Photo(c.LastMessagePhoto)

		// Popola l'oggetto ConversationSummary con i dati della conversazione corrente.
		apiConvs.Conversations[i] = ConversationSummary{
			ConversationID: ResourceId(c.ID), // ID univoco della conversazione
			Name:           Name(c.Name),     // Nome della conversazione (o del gruppo)
			Photo:          Photo(c.Photo),   // Foto della conversazione (o del gruppo)
			IsGroup:        c.IsGroup,
			// Costruisce l'oggetto LastMsg per l'anteprima dell'ultimo messaggio.
			LastMsg: &Message{
				Body: BodyMessage{
					Text:  &txt,   // Testo dell'ultimo messaggio
					Photo: &photo, // Foto dell'ultimo messaggio (se presente)
				},
				// Formatta il timestamp dell'ultimo messaggio nel formato standard RFC3339.
				SendTime: Timestamp(c.LastMessageTimestamp.Format(time.RFC3339)),
				SenderID: ResourceId(c.LastMessageSenderID),
			},
		}
	}

	// Invia la risposta JSON con codice 200 OK contenente la lista delle conversazioni.
	rt.sendJSONResponse(w, http.StatusOK, apiConvs)
}

// createConversation gestisce la creazione di una nuova conversazione 1-a-1 o il recupero di una esistente.
// Endpoint: POST /users/:user/conversations/:conversation_id
// Nota: In questo contesto, :conversation_id nell'URL rappresenta l'ID dell'utente con cui si vuole conversare (targetID).
// Parametri:
// - user: ID dell'utente che inizia la conversazione (deve corrispondere all'utente autenticato).
// - conversation_id: ID dell'utente destinatario (interlocutore).
// Risposta:
// - 201 Created: Restituisce l'oggetto Conversation creato o recuperato.
// - 400 Bad Request: Se gli ID forniti non sono validi.
// - 403 Forbidden: Se l'utente non è autorizzato.
// - 500 Internal Server Error: Errore del server.
func (rt *_router) createConversation(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Estrae e converte l'ID dell'utente chiamante dai parametri dell'URL.
	userID, err := strconv.ParseInt(ps.ByName("user"), 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid user ID")
		return
	}
	// Estrae e converte l'ID dell'utente destinatario (targetID) dai parametri dell'URL.
	// Nota: il parametro si chiama "conversation_id" per convenzione REST, ma qui funge da UserID target.
	targetID, err := strconv.ParseInt(ps.ByName("conversation_id"), 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid target ID")
		return
	}

	// Verifica che l'utente autenticato sia autorizzato ad agire per conto di 'userID'.
	if !rt.checkAuth(w, ctx.UserID, userID) {
		return
	}

	// Check Self-Chat: Impedisce di creare una chat con se stessi.
	if userID == targetID {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Cannot create a conversation with yourself")
		return
	}

	// Decodifica il body della richiesta per ottenere un eventuale messaggio iniziale.
	var msgIn MessageInput
	if err := json.NewDecoder(r.Body).Decode(&msgIn); err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	// Invoca il metodo del database per creare una nuova conversazione 1-a-1 tra i due utenti
	// o per recuperare quella esistente se c'è già.
	c, err := rt.db.CreateOneOnOneConversation(userID, targetID)
	if err != nil {
		// Logga l'errore e restituisce 500 Internal Server Error in caso di fallimento.
		rt.baseLogger.WithError(err).Error("Error creating conversation")
		rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Internal Server Error")
		return
	}

	// Notifica i partecipanti via WebSocket che una conversazione è stata creata/acceduta
	rt.notifyConversation(c.ID, WebSocketMessage{
		Type: "CONVERSATION_UPDATED",
		Payload: map[string]interface{}{
			"conversation_id": c.ID,
		},
	})

	// Decodifica il body della richiesta per ottenere un eventuale messaggio iniziale.
	// Estraiamo testo e foto dal body decodificato
	var textContent string
	var photoContent string
	if msgIn.Body.Text != nil {
		textContent = string(*msgIn.Body.Text)
	}
	if msgIn.Body.Photo != nil {
		photoContent = string(*msgIn.Body.Photo)
	}

	// Se c'è del contenuto, lo inviamo usando la funzione SendMessage esistente
	if textContent != "" || photoContent != "" {
		dbMsg, err := rt.db.SendMessage(c.ID, userID, textContent, photoContent, nil, false)
		if err != nil {
			// Logghiamo l'errore ma non blocchiamo tutto, la conversazione ormai esiste
			rt.baseLogger.WithError(err).Error("Error sending initial message")
		} else {
			// Costruisce il messaggio per la notifica
			txt := Text(dbMsg.ContentText)
			var photoPtr *Photo
			if dbMsg.ContentPhoto != "" {
				p := Photo(dbMsg.ContentPhoto)
				photoPtr = &p
			}
			respMsg := Message{
				MessageID:      ResourceId(dbMsg.ID),
				ConversationID: ResourceId(dbMsg.ConversationID),
				SenderID:       ResourceId(dbMsg.SenderID),
				SendTime:       Timestamp(dbMsg.Timestamp.Format(time.RFC3339)),
				Body: BodyMessage{
					Text:  &txt,
					Photo: photoPtr,
				},
				Reactions: []Reaction{},
				Forwarded: dbMsg.Forwarded,
			}

			rt.notifyConversation(c.ID, WebSocketMessage{
				Type:    "NEW_MESSAGE",
				Payload: respMsg,
			})
		}
	}

	// Prepara i dati della conversazione per la risposta API.
	apiName := c.Name
	apiPhoto := c.Photo

	// Se la foto è vuota, è una chat 1-a-1. Dobbiamo prendere i dati dell'altro utente.
	if apiPhoto == "" {
		targetUser, err := rt.db.GetUserByID(targetID)
		if err == nil {
			apiPhoto = targetUser.Photo
			// Se anche il nome della chat è vuoto, usiamo lo username dell'altro
			if apiName == "" {
				apiName = targetUser.Username
			}
			// Se non riesce a recuperare i dettagli dell'utente target, logga un avviso.
		} else {
			rt.baseLogger.WithError(err).Warn("Could not fetch target user details")
		}
	}

	// Recupera i partecipanti della conversazione
	members, err := rt.db.GetConversationMembers(c.ID)
	if err != nil {
		rt.baseLogger.WithError(err).Error("Error getting conversation members")
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

	// Costruisce l'oggetto di risposta API con i dettagli della conversazione.
	apiConv := Conversation{
		ConversationID: ResourceId(c.ID), // ID della conversazione
		Name:           Name(apiName),    // Nome (potrebbe essere vuoto per 1-a-1, gestito dal frontend o query)
		Photo:          Photo(apiPhoto),  // Foto della conversazione
		IsGroup:        c.IsGroup,
		Participants:   Users{Users: apiMembers},
		Messages:       []Message{}, // Inizializza la lista messaggi vuota (non vengono restituiti alla creazione)
	}

	// Restituisce la risposta JSON con codice 201 Created.
	rt.sendJSONResponse(w, http.StatusCreated, apiConv)
}

// getConversation recupera i dettagli di una specifica conversazione e i suoi messaggi, supportando la paginazione.
// Endpoint: GET /users/:user/conversations/:conversation_id
// Parametri URL:
// - user: ID dell'utente richiedente.
// - conversation_id: ID della conversazione da recuperare.
// Parametri Query:
// - limit: Numero massimo di messaggi da restituire (default 50).
// - offset: Numero di messaggi da saltare (per paginazione, default 0).
func (rt *_router) getConversation(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Estrae gli ID dai parametri del percorso.
	userID, _ := strconv.ParseInt(ps.ByName("user"), 10, 64)
	conversationID, _ := strconv.ParseInt(ps.ByName("conversation_id"), 10, 64)

	// Verifica che l'utente sia autenticato e autorizzato.
	if !rt.checkAuth(w, ctx.UserID, userID) {
		return
	}

	// L'utente è nella conversazione?
	// Usiamo una verifica manuale invece dell'helper per poter restituire 404 Not Found invece di 403 Forbidden.
	in, err := rt.db.IsUserInConversation(conversationID, userID)
	if err != nil {
		rt.baseLogger.WithError(err).Error("Error checking conversation membership")
		rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Internal Server Error")
		return
	}
	if !in {
		// Restituisce 404 per non rivelare l'esistenza della conversazione a chi non è membro.
		rt.sendErrorResponse(w, http.StatusNotFound, "not_found", "Conversation not found")
		return
	}

	// Imposta i valori di default per la paginazione dei messaggi.
	limit := 50
	offset := 0

	// Legge e valida il parametro di query 'limit'.
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}
	// Legge e valida il parametro di query 'offset'.
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	}

	// Recupera i metadati della conversazione dal database.
	c, err := rt.db.GetConversation(conversationID, ctx.UserID)
	if err != nil {
		// Se la conversazione non esiste, restituisce 404 Not Found.
		rt.sendErrorResponse(w, http.StatusNotFound, "not_found", "Conversation not found")
		return
	}

	// Segna i messaggi come letti (Mark as Read)
	// Questa operazione aggiorna lo stato 'read' dei messaggi ricevuti dall'utente corrente.
	// Lo facciamo prima di recuperare i messaggi così l'utente vedrà lo stato aggiornato se ricarica,
	// ma per chi invia, vedrà il cambiamento al prossimo polling.
	markedAsRead, err := rt.db.MarkConversationAsRead(conversationID, ctx.UserID)
	if err == nil && markedAsRead {
		// Notifica i partecipanti via WebSocket che i messaggi sono stati letti
		rt.notifyConversation(conversationID, WebSocketMessage{
			Type: "CONVERSATION_UPDATED",
			Payload: map[string]interface{}{
				"conversation_id": conversationID,
				"action":          "messages_read",
				"reader_id":       ctx.UserID,
			},
		})
	}

	// Recupera i messaggi della conversazione dal database applicando limit e offset.
	msgs, err := rt.db.GetMessages(conversationID, limit, offset)
	if err != nil {
		rt.baseLogger.WithError(err).Error("Error getting messages")
		rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Internal Server Error")
		return
	}

	// Converte i messaggi per l'API
	apiMsgs := make([]Message, len(msgs))
	for i, m := range msgs {
		txt := Text(m.ContentText)
		// Gestione sicura per evitare null pointer se la foto è vuota (opzionale, dipende dal frontend)
		var photoPtr *Photo
		if m.ContentPhoto != "" {
			p := Photo(m.ContentPhoto)
			photoPtr = &p
		} else {
			p := Photo("")
			photoPtr = &p
		}

		// Gestione sicura puntatore ReplyTo
		var replyToPtr *ResourceId
		if m.ReplyTo != 0 {
			r := ResourceId(m.ReplyTo)
			replyToPtr = &r
		}

		var apiReactions []Reaction
		for _, r := range m.Reactions {
			apiReactions = append(apiReactions, Reaction{
				ReactionID: ResourceId(r.ID),
				UserID:     ResourceId(r.UserID),
				Emoticon:   r.Emoji,
			})
		}

		// Costruisce l'oggetto Message API.
		// Inseriamo i messaggi in ordine inverso (dall'ultimo al primo)
		// così otteniamo l'ordine cronologico (Oldest -> Newest) senza un secondo ciclo.
		status := "received"
		if m.SenderID == ctx.UserID {
			status = "sent"
		}
		if m.Read {
			status = "read"
		}

		apiMsgs[len(msgs)-1-i] = Message{
			MessageID:      ResourceId(m.ID),             // ID del messaggio
			ConversationID: ResourceId(m.ConversationID), // ID della conversazione
			SenderID:       ResourceId(m.SenderID),       // ID del mittente
			ReplyTo:        replyToPtr,
			Body: BodyMessage{
				Text:  &txt,     // Contenuto testuale
				Photo: photoPtr, // Contenuto fotografico (se presente)
			},
			// Formatta il timestamp di invio.
			SendTime:  Timestamp(m.Timestamp.Format(time.RFC3339)),
			Forwarded: m.Forwarded,
			Reactions: apiReactions,
			Status:    status,
		}
	}

	// Recupera i partecipanti della conversazione
	members, err := rt.db.GetConversationMembers(conversationID)
	if err != nil {
		rt.baseLogger.WithError(err).Error("Error getting conversation members")
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

	// Costruisce l'oggetto Conversation completo da restituire.
	apiConv := Conversation{
		ConversationID: ResourceId(c.ID),
		Name:           Name(c.Name),
		Photo:          Photo(c.Photo),
		IsGroup:        c.IsGroup,
		Participants:   Users{Users: apiMembers},
		Messages:       apiMsgs, // Include la lista dei messaggi paginati
	}

	// Invia la risposta JSON con codice 200 OK.
	rt.sendJSONResponse(w, http.StatusOK, apiConv)
}

// setConversationSeen segna tutti i messaggi di una conversazione come letti per l'utente specificato.
// Endpoint: PUT /users/:user/conversations/:conversation_id/seen
func (rt *_router) setConversationSeen(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	userID, err := strconv.ParseInt(ps.ByName("user"), 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid user ID")
		return
	}
	conversationID, err := strconv.ParseInt(ps.ByName("conversation_id"), 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid conversation ID")
		return
	}

	if !rt.checkAuth(w, ctx.UserID, userID) {
		return
	}

	if !rt.checkConversationMember(w, conversationID, userID) {
		return
	}

	marked, err := rt.db.MarkConversationAsRead(conversationID, userID)
	if err != nil {
		rt.baseLogger.WithError(err).Error("Error marking conversation as read")
		rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Internal Server Error")
		return
	}

	if marked {
		rt.notifyConversation(conversationID, WebSocketMessage{
			Type: "CONVERSATION_UPDATED",
			Payload: map[string]interface{}{
				"conversation_id": conversationID,
				"action":          "messages_read",
				"reader_id":       userID,
			},
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

// IsUserInConversation verifica se un utente è partecipante di una conversazione (o gruppo)
// Parametri:
// - conversationID: ID della conversazione da verificare.
// - userID: ID dell'utente da controllare.
// Restituisce:
// - true se l'utente è nella conversazione, false altrimenti.
// - error in caso di problemi nel recupero dei dati.
func (rt *_router) IsUserInConversation(conversationID int64, userID int64) (bool, error) {
	// Esegue una query nel database per verificare la partecipazione dell'utente nella conversazione.
	// Restituisce true se trova un record corrispondente, false altrimenti.
	return rt.db.IsUserInConversation(conversationID, userID)
}
