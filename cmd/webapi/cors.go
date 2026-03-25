package main

import (
	"github.com/gorilla/handlers"
	"net/http"
)

// applyCORSHandler applica una politica CORS al router.
// Questo permette al frontend (che gira su una porta diversa, es. 8080)
// di comunicare con questo backend (che gira su porta 3000).
func applyCORSHandler(h http.Handler) http.Handler {
	return handlers.CORS(
		// 1. Header permessi:
		// "Content-Type" serve per inviare JSON.
		// "Authorization" serve per inviare il token Bearer (login).
		handlers.AllowedHeaders([]string{
			"Content-Type",
			"Authorization",
		}),

		// 2. Metodi permessi:
		// Specifica quali verbi HTTP il frontend può usare.
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),

		// 3. Origini permesse:
		// "*" permette richieste da qualsiasi sito.
		// In produzione, dovresti mettere l'URL specifico del tuo frontend (es. "http://localhost:8080")
		// per maggiore sicurezza, ma "*" va benissimo per lo sviluppo e i test.
		handlers.AllowedOrigins([]string{"*"}),

		// 4. MaxAge:
		// Indica al browser per quanto tempo (in secondi) può ricordare la risposta CORS
		// senza dover chiedere di nuovo conferma al server.
		handlers.MaxAge(1),
	)(h)
}
