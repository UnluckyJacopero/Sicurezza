package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"wasaTxt/service/api/reqcontext"

	"github.com/julienschmidt/httprouter"
)

// sendMessage gestisce l'invio di un nuovo messaggio all'interno di una conversazione.
// Endpoint: POST /users/:user/conversations/:conversation_id/messages
// Parametri URL:
// - user: ID dell'utente mittente.
// - conversation_id: ID della conversazione in cui inviare il messaggio.
// Body: Oggetto JSON contenente il corpo del messaggio (testo e/o foto).
func (rt *_router) sendMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Estrae gli ID dai parametri del percorso.
	userID, _ := strconv.ParseInt(ps.ByName("user"), 10, 64)
	convID, _ := strconv.ParseInt(ps.ByName("conversation_id"), 10, 64)

	// Verifica che l'utente autenticato sia autorizzato ad inviare messaggi come 'userID'.
	if !rt.checkAuth(w, ctx.UserID, userID) {
		return
	}

	// Verifica che l'utente sia membro della conversazione
	if !rt.checkConversationMember(w, convID, userID) {
		return
	}

	// Decodifica il corpo della richiesta JSON nella struttura MessageInput.
	var msgIn MessageInput
	if err := json.NewDecoder(r.Body).Decode(&msgIn); err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}

	// Estrae il contenuto testuale e fotografico dal body del messaggio.
	// Gestisce i puntatori nil nel caso in cui uno dei campi sia assente.
	var textContent, photoContent string
	if msgIn.Body.Text != nil {
		textContent = string(*msgIn.Body.Text)
	} else if msgIn.Body.Caption != nil {
		// Se 'text' è vuoto ma c'è una 'caption' (comune con le foto), usiamo quella
		textContent = string(*msgIn.Body.Caption)
	}

	if msgIn.Body.Photo != nil {
		photoContent = string(*msgIn.Body.Photo)
	}

	// Validazione: il messaggio deve contenere almeno testo o una foto.
	// Se entrambi sono vuoti, restituisce un errore 400 Bad Request.
	if textContent == "" && photoContent == "" {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Message cannot be empty")
		return
	}

	var replyToID *int64
	// Supporto sia per Body (msgIn.ReplyTo) che per Query Param (reply_to_message_id) come da specifica OpenAPI
	if msgIn.ReplyTo != nil {
		id := int64(*msgIn.ReplyTo)
		replyToID = &id
	} else if qReplyTo := r.URL.Query().Get("reply_to_message_id"); qReplyTo != "" {
		if id, err := strconv.ParseInt(qReplyTo, 10, 64); err == nil {
			replyToID = &id
		}
	}

	if replyToID != nil {
		id := *replyToID

		// Verifica integrità Reply-To: Se è una risposta, verifica che il messaggio originale esista
		// e appartenga alla stessa conversazione.
		origMsg, err := rt.db.GetMessage(id)
		if err != nil {
			// Se non trova il messaggio (o errore DB), restituisce errore.
			// Qui assumiamo che GetMessage ritorni errore se non trova.
			// Potremmo raffinare controllando se è sql.ErrNoRows.
			rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Reply-to message not found")
			return
		}
		if origMsg.ConversationID != convID {
			rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Cannot reply to a message from another conversation")
			return
		}
	}

	// Invoca il metodo del database per salvare il messaggio.
	// Passa l'ID della conversazione, l'ID del mittente e i contenuti.
	dbMsg, err := rt.db.SendMessage(convID, userID, textContent, photoContent, replyToID, false)
	if err != nil {
		// Logga l'errore e restituisce 500 Internal Server Error.
		rt.baseLogger.WithError(err).Error("Error sending message")
		rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Internal Server Error")
		return
	}

	// Costruisce l'oggetto di risposta API con i dettagli del messaggio creato.
	txt := Text(dbMsg.ContentText)
	var photoPtr *Photo
	if dbMsg.ContentPhoto != "" {
		p := Photo(dbMsg.ContentPhoto)
		photoPtr = &p
	}
	// Gestisce il campo ReplyTo
	var respReplyTo *ResourceId
	if dbMsg.ReplyTo != 0 {
		rId := ResourceId(dbMsg.ReplyTo)
		respReplyTo = &rId
	}
	respMsg := Message{
		MessageID:      ResourceId(dbMsg.ID),
		ConversationID: ResourceId(dbMsg.ConversationID),
		SenderID:       ResourceId(dbMsg.SenderID),
		SendTime:       Timestamp(dbMsg.Timestamp.Format(time.RFC3339)),
		ReplyTo:        respReplyTo,
		Body: BodyMessage{
			Text:  &txt,
			Photo: photoPtr,
		},
		Reactions: []Reaction{},
		Forwarded: dbMsg.Forwarded,
		Status:    "sent",
	}

	// Notifica i partecipanti via WebSocket
	rt.notifyConversation(convID, WebSocketMessage{
		Type:    "NEW_MESSAGE",
		Payload: respMsg,
	})

	// Restituisce la risposta JSON con codice 201 Created.
	rt.sendJSONResponse(w, http.StatusCreated, respMsg)
}

// deleteMessage permette a un utente di eliminare un messaggio inviato precedentemente.
// Endpoint: DELETE /users/:user/conversations/:conversation_id/messages/:message
// Parametri URL:
// - user: ID dell'utente che richiede l'eliminazione.
// - message: ID del messaggio da eliminare.
func (rt *_router) deleteMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Estrae gli ID dai parametri del percorso.
	userID, _ := strconv.ParseInt(ps.ByName("user"), 10, 64)
	conversationID, _ := strconv.ParseInt(ps.ByName("conversation_id"), 10, 64)
	messageID, _ := strconv.ParseInt(ps.ByName("message"), 10, 64)

	// Verifica l'autorizzazione dell'utente.
	if !rt.checkAuth(w, ctx.UserID, userID) {
		return
	}

	// Verifica che l'utente sia membro della conversazione
	if !rt.checkConversationMember(w, conversationID, userID) {
		return
	}

	// Recupera il messaggio dal database per verificare che esista e controllarne la proprietà.
	msg, err := rt.db.GetMessage(messageID)
	if err != nil {
		// Se il messaggio non viene trovato, restituisce 404 Not Found.
		rt.sendErrorResponse(w, http.StatusNotFound, "not_found", "Message not found")
		return
	}
	// Verifica che l'utente richiedente sia effettivamente il mittente del messaggio.
	// Solo il mittente può eliminare il proprio messaggio.
	if msg.SenderID != userID {
		rt.sendErrorResponse(w, http.StatusForbidden, "forbidden", "Not your message")
		return
	}

	// Procede con l'eliminazione del messaggio dal database.
	err = rt.db.DeleteMessage(messageID)
	if err != nil {
		// Logga l'errore e restituisce 500 Internal Server Error.
		rt.baseLogger.WithError(err).Error("Error deleting message")
		rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Internal Server Error")
		return
	}

	// Notifica i partecipanti via WebSocket
	rt.notifyConversation(conversationID, WebSocketMessage{
		Type: "CONVERSATION_UPDATED",
		Payload: map[string]interface{}{
			"conversation_id": conversationID,
			"message_id":      messageID,
			"action":          "deleted",
		},
	})

	// Restituisce 200 OK e un messaggio di conferma.
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`"Action successfully completed"`))
}

// forwardMessage inoltra un messaggio esistente a un'altra conversazione
func (rt *_router) forwardMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
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
	messageID, err := strconv.ParseInt(ps.ByName("message"), 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid message ID")
		return
	}
	destIDStr := r.URL.Query().Get("destination_id")
	destConvID, err := strconv.ParseInt(destIDStr, 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid destination conversation ID")
		return
	}

	if !rt.checkAuth(w, ctx.UserID, userID) {
		return
	}

	// Verifica che l'utente sia membro della conversazione di origine
	if !rt.checkConversationMember(w, conversationID, userID) {
		return
	}

	// Verifica che l'utente sia membro della conversazione di destinazione
	if !rt.checkConversationMember(w, destConvID, userID) {
		return
	}

	// Recupera il messaggio originale
	originalMsg, err := rt.db.GetMessage(messageID)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusNotFound, "not_found", "Message not found")
		return
	}

	// Invia un nuovo messaggio con lo stesso contenuto nella conversazione di destinazione
	newMsg, err := rt.db.SendMessage(destConvID, userID, originalMsg.ContentText, originalMsg.ContentPhoto, nil, true)
	if err != nil {
		rt.baseLogger.WithError(err).Error("Forward failed")
		rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Internal Server Error")
		return
	}
	// Costruisce l'oggetto di risposta API con i dettagli del messaggio creato.
	txt := Text(newMsg.ContentText)
	var photoPtr *Photo
	if newMsg.ContentPhoto != "" {
		p := Photo(newMsg.ContentPhoto)
		photoPtr = &p
	}
	respMsg := Message{
		MessageID:      ResourceId(newMsg.ID),
		ConversationID: ResourceId(newMsg.ConversationID),
		SenderID:       ResourceId(newMsg.SenderID),
		SendTime:       Timestamp(newMsg.Timestamp.Format(time.RFC3339)),
		Body: BodyMessage{
			Text:  &txt,
			Photo: photoPtr,
		},
		Reactions: []Reaction{},
		Forwarded: newMsg.Forwarded,
		Status:    "sent",
	}

	// Notifica i partecipanti via WebSocket
	rt.notifyConversation(destConvID, WebSocketMessage{
		Type:    "NEW_MESSAGE",
		Payload: respMsg,
	})

	// Restituisce la risposta JSON con codice 201 Created.
	rt.sendJSONResponse(w, http.StatusCreated, respMsg)
}
