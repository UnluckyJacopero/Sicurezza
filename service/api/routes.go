package api

import (
	"net/http"
)

// Handler restituisce un'istanza di httprouter.Router che gestisce tutte le API registrate in questo pacchetto.
// Configura le rotte per sessioni, utenti, conversazioni, messaggi, reazioni e gruppi.
func (rt *_router) Handler() http.Handler {
	// Registra le rotte
	rt.router.GET("/liveness", rt.liveness)

	// Stream WebSocket
	rt.router.GET("/stream", rt.wrap(rt.subscribeToStream))

	// Sessione
	// POST /session - Effettua il login e crea una nuova sessione
	rt.router.POST("/session", rt.wrap(rt.doLogin))

	// Users
	// GET /users - Cerca utenti (es. per nome)
	rt.router.GET("/users", rt.wrap(rt.findUsers))
	// PUT /users/{user}/username - Imposta il nome utente
	rt.router.PUT("/users/:user/username", rt.wrap(rt.setMyUserName))
	// PUT /users/{user}/photo - Imposta la foto profilo
	rt.router.PUT("/users/:user/photo", rt.wrap(rt.setMyPhoto))

	// Conversations
	// GET /users/{user}/conversations - Ottiene la lista delle conversazioni dell'utente
	rt.router.GET("/users/:user/conversations", rt.wrap(rt.getMyConversations))
	// PUT /users/{user}/conversations/{conversation_id} - Crea una nuova conversazione
	rt.router.PUT("/users/:user/conversations/:conversation_id", rt.wrap(rt.createConversation))
	// GET /users/{user}/conversations/{conversation_id} - Ottiene i dettagli di una conversazione specifica
	rt.router.GET("/users/:user/conversations/:conversation_id", rt.wrap(rt.getConversation))
	// PUT /users/{user}/conversations/{conversation_id}/seen - Segna la conversazione come letta
	rt.router.PUT("/users/:user/conversations/:conversation_id/seen", rt.wrap(rt.setConversationSeen))

	// Messages
	// POST /users/{user}/conversations/{conversation_id}/messages - Invia un nuovo messaggio
	rt.router.POST("/users/:user/conversations/:conversation_id/messages", rt.wrap(rt.sendMessage))
	// POST /users/{user}/conversations/{conversation_id}/messages/{message} - Inoltra un messaggio esistente
	rt.router.POST("/users/:user/conversations/:conversation_id/messages/:message", rt.wrap(rt.forwardMessage))
	// DELETE /users/{user}/conversations/{conversation_id}/messages/{message} - Elimina un messaggio
	rt.router.DELETE("/users/:user/conversations/:conversation_id/messages/:message", rt.wrap(rt.deleteMessage))

	// Comments / Reactions
	// PUT /users/{user}/conversations/{conversation_id}/messages/{message}/reactions - Aggiunge una reazione a un messaggio
	rt.router.PUT("/users/:user/conversations/:conversation_id/messages/:message/reactions", rt.wrap(rt.commentMessage))
	// DELETE /users/{user}/conversations/{conversation_id}/messages/{message}/reactions/{reaction} - Rimuove una reazione
	rt.router.DELETE("/users/:user/conversations/:conversation_id/messages/:message/reactions/:reaction", rt.wrap(rt.uncommentMessage))

	// Groups
	// POST /users/{user}/groups - Crea un nuovo gruppo
	rt.router.POST("/users/:user/groups", rt.wrap(rt.createGroup))
	// PUT /users/{user}/groups/{group} - Aggiunge un membro al gruppo
	rt.router.PUT("/users/:user/groups/:group", rt.wrap(rt.addToGroup))
	// DELETE /users/{user}/groups/{group} - Lascia il gruppo
	rt.router.DELETE("/users/:user/groups/:group", rt.wrap(rt.leaveGroup))
	// PUT /users/{user}/groups/{group}/groupname - Imposta il nome del gruppo
	rt.router.PUT("/users/:user/groups/:group/groupname", rt.wrap(rt.setGroupName))
	// PUT /users/{user}/groups/{group}/groupphoto - Imposta la foto del gruppo
	rt.router.PUT("/users/:user/groups/:group/groupphoto", rt.wrap(rt.setGroupPhoto))

	return rt.router
}
