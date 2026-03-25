package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"wasaTxt/service/api/reqcontext"

	"github.com/julienschmidt/httprouter"
)

// doLogin gestisce il processo di autenticazione e registrazione dell'utente.
// Endpoint: POST /session
// Body: Oggetto JSON contenente lo username dell'utente.
// Logica:
// - Se lo username esiste già nel database, effettua il login restituendo l'ID utente esistente.
// - Se lo username non esiste, crea un nuovo utente e restituisce il nuovo ID.
func (rt *_router) doLogin(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// 1. Decodifica l'input JSON dal body della richiesta nella struttura LoginInput.
	var loginReq LoginInput
	if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
		// Se il JSON non è valido, restituisce 400 Bad Request.
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	// Validazione: lo username deve avere una lunghezza minima di 3 caratteri.
	if len(loginReq.Name) < 3 {
		rt.sendErrorResponse(w, http.StatusBadRequest, "bad_request", "Username too short")
		return
	}

	var statusCode int
	// 2. Tenta di recuperare l'utente dal database utilizzando lo username fornito.
	dbUser, err := rt.db.GetUserByName(string(loginReq.Name))

	// 3. Gestione del risultato della ricerca:
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Se l'utente non esiste (ErrNoRows).
			// Crea un nuovo utente con lo username fornito.
			dbUser, err = rt.db.CreateUser(string(loginReq.Name))
			if err != nil {
				ctx.Logger.WithError(err).Error("Can't create user")
				rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Internal Server Error")
				return
			}
			statusCode = http.StatusCreated // Segniamo che è un 201
		} else {
			ctx.Logger.WithError(err).Error("Database error")
			rt.sendErrorResponse(w, http.StatusInternalServerError, "server_error", "Internal Server Error")
			return
		}
	} else {
		statusCode = http.StatusOK // Segniamo che è un 200
	}

	// Impostazione header e status code
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	// 4. Risposta: Invia una risposta JSON contenente l'ID dell'utente (esistente o appena creato).
	_ = json.NewEncoder(w).Encode(struct {
		ID int64 `json:"id"`
	}{
		ID: dbUser.ID,
	})
}
