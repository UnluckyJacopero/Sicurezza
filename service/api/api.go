/*
Package api exposes the main API engine. All HTTP APIs are handled here - so-called "business logic" should be here, or
in a dedicated package (if that logic is complex enough).

To use this package, you should create a new instance with New() passing a valid Config. The resulting Router will have
the Router.Handler() function that returns a handler that can be used in a http.Server (or in other middlewares).

Example:

	// Create the API router
	apirouter, err := api.New(api.Config{
		Logger:   logger,
		Database: appdb,
	})
	if err != nil {
		logger.WithError(err).Error("error creating the API server instance")
		return fmt.Errorf("error creating the API server instance: %w", err)
	}
	router := apirouter.Handler()

	// ... other stuff here, like middleware chaining, etc.

	// Create the API server
	apiserver := http.Server{
		Addr:              cfg.Web.APIHost,
		Handler:           router,
		ReadTimeout:       cfg.Web.ReadTimeout,
		ReadHeaderTimeout: cfg.Web.ReadTimeout,
		WriteTimeout:      cfg.Web.WriteTimeout,
	}

	// Start the service listening for requests in a separate goroutine
	apiserver.ListenAndServe()

See the `main.go` file inside the `cmd/webapi` for a full usage example.
*/
package api

import (
	"errors"
	"net/http"
	"sync"
	"wasaTxt/service/database"

	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
)

// Config is the configuration structure needed to initialize the API router.
type Config struct {
	// Logger where log entries are sent
	Logger logrus.FieldLogger

	// Database is the instance of database.AppDatabase where data are saved
	Database database.AppDatabase
}

// Router is the package API interface representing an API handler builder
type Router interface {
	// Handler returns an HTTP handler for APIs provided in this package
	Handler() http.Handler

	// Close terminates any resource used in the package
	Close() error
}

// New crea e inizializza una nuova istanza del router API con la configurazione specificata.
// Parametri:
// - cfg: Oggetto Config contenente le dipendenze necessarie (Logger, Database).
// Ritorna:
// - Router: L'istanza del router inizializzata.
// - error: Eventuale errore di configurazione (es. dipendenze mancanti).
func New(cfg Config) (Router, error) {
	// Verifica che la configurazione sia valida e completa.
	if cfg.Logger == nil {
		return nil, errors.New("logger is required")
	}
	if cfg.Database == nil {
		return nil, errors.New("database is required")
	}

	// Crea un nuovo router httprouter dove verranno registrati gli endpoint HTTP.
	// Le opzioni RedirectTrailingSlash e RedirectFixedPath sono disabilitate per un controllo preciso dei percorsi.
	router := httprouter.New()
	router.RedirectTrailingSlash = false
	router.RedirectFixedPath = false

	// Restituisce l'implementazione privata _router che soddisfa l'interfaccia Router.
	return &_router{
		router:     router,
		baseLogger: cfg.Logger,
		db:         cfg.Database,
		conns:      make(map[int64][]*websocket.Conn),
	}, nil
}

// _router è l'implementazione concreta dell'interfaccia Router.
// Mantiene i riferimenti al router HTTP sottostante, al logger di base e al database.
type _router struct {
	router *httprouter.Router

	// baseLogger is a logger for non-requests contexts, like goroutines or background tasks not started by a request.
	// Use context logger if available (e.g., in requests) instead of this logger.
	baseLogger logrus.FieldLogger

	db database.AppDatabase

	// Connessioni WebSocket
	conns      map[int64][]*websocket.Conn
	connsMutex sync.Mutex
}
