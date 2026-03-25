package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"wasaTxt/service/api/reqcontext"
	"wasaTxt/service/database"

	"github.com/julienschmidt/httprouter"
)

// commentMessage gestisce l'aggiunta di una reazione (emoticon) a un messaggio.
// Endpoint: PUT /users/:user/conversations/:conversation_id/messages/:message/reactions
// Parametri URL:
// - user: ID dell'utente che aggiunge la reazione.
// - message: ID del messaggio a cui aggiungere la reazione.
// Body: Oggetto JSON contenente l'emoticon.
func (rt *_router) commentMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Estrae e valida l'ID del messaggio.
	messageID, err := strconv.ParseInt(ps.ByName("message"), 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid message ID")
		return
	}
	// Estrae e valida l'ID della conversazione.
	conversationID, err := strconv.ParseInt(ps.ByName("conversation_id"), 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid conversation ID")
		return
	}
	// Estrae e valida l'ID dell'utente.
	userID, err := strconv.ParseInt(ps.ByName("user"), 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid user ID")
		return
	}

	// Verifica che l'utente autenticato corrisponda all'utente specificato nell'URL.
	if userID != ctx.UserID {
		rt.sendErrorResponse(w, http.StatusForbidden, "forbidden", "Forbidden")
		return
	}

	// Verifica che l'utente sia membro della conversazione
	if !rt.checkConversationMember(w, conversationID, userID) {
		return
	}

	// Decodifica il body della richiesta per ottenere l'emoticon.
	var input NewReactionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	// Validazione: l'emoticon non può essere vuota.
	if len(input.Emoticon) == 0 {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Emoticon is required")
		return
	}
	// Validazione Emoji: Controllo base sulla lunghezza per evitare abusi.
	// Le emoji sono caratteri unicode, ma raramente superano i 4-8 byte (anche quelle composte).
	// Impostiamo un limite ragionevole (es. 10 caratteri/byte) per evitare stringhe lunghe.
	if len(input.Emoticon) > 10 {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid emoticon")
		return
	}

	// Invoca il metodo del database per aggiungere la reazione.
	action, err := rt.db.ToggleReaction(messageID, userID, input.Emoticon)
	if err != nil {
		if err.Error() == "cannot react to own message" {
			rt.sendErrorResponse(w, http.StatusForbidden, "forbidden", "Cannot react to own message")
			return
		}
		// Logga l'errore e restituisce 500 Internal Server Error.
		rt.baseLogger.WithError(err).Error("Error adding reaction")
		rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Internal Server Error")
		return
	}

	// Notifica i partecipanti via WebSocket
	rt.notifyConversation(conversationID, WebSocketMessage{
		Type: "CONVERSATION_UPDATED",
		Payload: map[string]interface{}{
			"conversation_id": conversationID,
			"message_id":      messageID,
			"action":          "reaction_updated",
		},
	})

	// Restituisce 200 OK e l'azione eseguita.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"action": action, "emoticon": input.Emoticon})
}

// uncommentMessage permette di rimuovere una reazione precedentemente aggiunta a un messaggio.
// Endpoint: DELETE /users/:user/conversations/:conversation_id/messages/:message/reactions/:reaction
// Parametri URL:
// - user: ID dell'utente che rimuove la reazione.
// - reaction: ID della reazione da rimuovere.
func (rt *_router) uncommentMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Estrae e valida l'ID della reazione.
	reactionID, err := strconv.ParseInt(ps.ByName("reaction"), 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid reaction ID")
		return
	}
	// Estrae e valida l'ID della conversazione.
	conversationID, err := strconv.ParseInt(ps.ByName("conversation_id"), 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid conversation ID")
		return
	}
	// Estrae e valida l'ID dell'utente.
	userID, err := strconv.ParseInt(ps.ByName("user"), 10, 64)
	if err != nil {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid user ID")
		return
	}

	// Verifica l'autorizzazione: solo l'utente che ha messo la reazione può rimuoverla.
	if userID != ctx.UserID {
		rt.sendErrorResponse(w, http.StatusForbidden, "forbidden", "Forbidden")
		return
	}

	// Verifica che l'utente sia membro della conversazione
	if !rt.checkConversationMember(w, conversationID, userID) {
		return
	}

	// Invoca il metodo del database per rimuovere la reazione.
	err = rt.db.RemoveReaction(reactionID, userID)
	if err != nil {
		if errors.Is(err, database.ErrReactionNotOwned) {
			rt.sendErrorResponse(w, http.StatusForbidden, "forbidden", "Cannot remove this reaction")
			return
		}
		rt.baseLogger.WithError(err).Error("Error removing reaction")
		rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Internal Server Error")
		return
	}

	// Notifica i partecipanti via WebSocket
	rt.notifyConversation(conversationID, WebSocketMessage{
		Type: "CONVERSATION_UPDATED",
		Payload: map[string]interface{}{
			"conversation_id": conversationID,
			"action":          "reaction_removed",
		},
	})

	// Restituisce 200 OK.
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`"Action successfully completed"`))
}
