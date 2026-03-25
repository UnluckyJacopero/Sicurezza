package api

import (
	"net/http"
	"strconv"
	"wasaTxt/service/api/reqcontext"

	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Permette connessioni da qualsiasi origine; in produzione, restringere questo
	},
}

// WebSocketMessage rappresenta un messaggio inviato tramite WebSocket.
type WebSocketMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

func (rt *_router) subscribeToStream(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Verifica l'autenticazione dell'utente
	userID := ctx.UserID
	if userID == 0 {
		token := r.URL.Query().Get("token")
		if token != "" {
			uid, err := strconv.ParseInt(token, 10, 64)
			if err == nil {
				userID = uid
			}
		}
	}

	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		rt.baseLogger.WithError(err).Error("Failed to upgrade connection")
		return
	}

	rt.registerClient(userID, conn)
	defer rt.unregisterClient(userID, conn)

	// Mantieni la connessione attiva e leggi i messaggi (anche se non ci aspettiamo messaggi dal client)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (rt *_router) registerClient(userID int64, conn *websocket.Conn) {
	rt.connsMutex.Lock()
	defer rt.connsMutex.Unlock()

	rt.conns[userID] = append(rt.conns[userID], conn)
}

func (rt *_router) unregisterClient(userID int64, conn *websocket.Conn) {
	rt.connsMutex.Lock()
	defer rt.connsMutex.Unlock()

	// Rimuove la connessione specificata dall'elenco delle connessioni dell'utente
	conns := rt.conns[userID]
	for i, c := range conns {
		if c == conn {
			rt.conns[userID] = append(conns[:i], conns[i+1:]...)
			break
		}
	}
	if len(rt.conns[userID]) == 0 {
		delete(rt.conns, userID)
	}
	_ = conn.Close()
}

func (rt *_router) broadcastMessage(userID int64, msg WebSocketMessage) {
	rt.connsMutex.Lock()
	defer rt.connsMutex.Unlock()

	// Ottiene tutte le connessioni WebSocket per l'utente specificato
	conns, ok := rt.conns[userID]
	if !ok {
		return
	}

	// Invia il messaggio a tutte le connessioni dell'utente
	for _, conn := range conns {
		err := conn.WriteJSON(msg)
		if err != nil {
			rt.baseLogger.WithError(err).Warn("Failed to write to websocket")
			_ = conn.Close()
		}
	}
}

func (rt *_router) notifyConversation(conversationID int64, msg WebSocketMessage) {
	// Esegui la notifica in una goroutine separata per non bloccare la richiesta HTTP principale
	go func() {
		members, err := rt.db.GetConversationMembers(conversationID)
		if err != nil {
			rt.baseLogger.WithError(err).Error("Error getting conversation members for notification")
			return
		}

		for _, member := range members {
			rt.broadcastMessage(member.ID, msg)
		}
	}()
}
