package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/julienschmidt/httprouter"
	"wasaTxt/service/api/reqcontext"
)

// checkConversationMember verifica se l'utente fa parte della conversazione.
// Se l'utente NON è membro o c'è un errore, invia la risposta HTTP appropriata e restituisce false.
// Se l'utente è membro, restituisce true.
func (rt *_router) checkConversationMember(w http.ResponseWriter, conversationID int64, userID int64) bool {
	in, err := rt.db.IsUserInConversation(conversationID, userID)
	if err != nil {
		rt.baseLogger.WithError(err).Error("Error checking conversation membership")
		rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Internal Server Error")
		return false
	}
	if !in {
		rt.sendErrorResponse(w, http.StatusForbidden, "forbidden", "You are not a participant of this conversation")
		return false
	}
	return true
}

// sendJSONResponse invia una risposta HTTP con il payload specificato in formato JSON.
// Imposta l'header Content-Type a application/json e lo status code.
func (rt *_router) sendJSONResponse(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}

// sendErrorResponse invia una risposta di errore standardizzata in formato JSON.
// Crea un oggetto Error contenente il codice, la motivazione (dallo status HTTP) e il messaggio di dettaglio.
func (rt *_router) sendErrorResponse(w http.ResponseWriter, status int, code string, message string) {
	errResp := Error{
		Errors: []ErrorItem{
			{
				Code:    code,
				Reason:  http.StatusText(status),
				Message: message,
			},
		},
	}
	rt.sendJSONResponse(w, status, errResp)
}

// wrap è un middleware che analizza la richiesta e aggiunge un RequestContext.
// Genera un UUID per la richiesta, configura il logger e verifica l'eventuale token di autenticazione.
// Se il token è valido, popola il campo UserID nel contesto.
func (rt *_router) wrap(fn func(http.ResponseWriter, *http.Request, httprouter.Params, reqcontext.RequestContext)) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Genera un UUID univoco per tracciare la richiesta
		reqUUID, err := uuid.NewV4()
		if err != nil {
			rt.baseLogger.WithError(err).Error("can't generate request uuid")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Crea il contesto iniziale con UUID e logger
		var ctx = reqcontext.RequestContext{
			ReqUUID: reqUUID,
			Logger:  rt.baseLogger.WithField("req_id", reqUUID),
		}

		// Estrae il Bearer token dall'header Authorization e imposta ctx.UserID se valido
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			userID, err := strconv.ParseInt(token, 10, 64)
			if err == nil {
				ctx.UserID = userID
				ctx.Logger = ctx.Logger.WithField("user_id", userID)
			} else {
				ctx.Logger.WithError(err).Warn("Invalid token format")
			}
		}

		// Chiama l'handler effettivo passando il contesto arricchito
		fn(w, r, ps, ctx)
	}
}

// liveness è un endpoint semplice per verificare che il servizio sia attivo.
// Restituisce sempre 200 OK.
func (rt *_router) liveness(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.WriteHeader(http.StatusOK)
}

// checkAuth verifica se l'utente è autenticato e aut  orizzato ad accedere alla risorsa.
// Parametri:
// - reqUser: ID dell'utente che effettua la richiesta (dal contesto).
// - ownerUser: ID del proprietario della risorsa (opzionale, 0 se non applicabile).
// Ritorna true se l'utente è autorizzato, altrimenti invia una risposta di errore e ritorna false.
func (rt *_router) checkAuth(w http.ResponseWriter, reqUser int64, ownerUser int64) bool {
	// 1. Verifica login: l'utente deve avere un ID valido (diverso da 0)
	if reqUser == 0 {
		rt.sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "You must be logged in")
		return false
	}

	// 2. Verifica proprietà risorsa: se specificato un proprietario, l'utente deve coincidere
	if ownerUser != 0 && reqUser != ownerUser {
		rt.sendErrorResponse(w, http.StatusForbidden, "forbidden", "You can only access your own resources")
		return false
	}

	return true
}
