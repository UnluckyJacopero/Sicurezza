/*
Webapi is the executable for the main web server.
It builds a web server around APIs from `service/api`.
Webapi connects to external resources needed (database) and starts two web servers: the API web server, and the debug.
Everything is served via the API web server, except debug variables (/debug/vars) and profiler infos (pprof).

Usage:

	webapi [flags]

Flags and configurations are handled automatically by the code in `load-configuration.go`.

Return values (exit codes):

	0
		The program ended successfully (no errors, stopped by signal)

	> 0
		The program ended due to an error

Note that this program will update the schema of the database to the latest version available (embedded in the
executable during the build).
*/
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"wasaTxt/service/api"
	"wasaTxt/service/database"

	"github.com/ardanlabs/conf/v3"
	"github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
)

// main è il punto di ingresso del programma. L'unico scopo di questa funzione è chiamare run() e impostare il codice di uscita se c'è
// qualche errore
func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error: ", err)
		os.Exit(1)
	}
}

// run esegue il programma. Il corpo di questa funzione dovrebbe eseguire i seguenti passaggi:
// * legge la configurazione
// * crea e configura il logger
// * si connette a qualsiasi risorsa esterna (come database, autenticatori, ecc.)
// * crea un'istanza del pacchetto service/api
// * avvia il server web principale (usando service/api.Router.Handler() per gli handler HTTP)
// * attende qualsiasi evento di terminazione: segnale SIGTERM (UNIX), errore server non recuperabile, ecc.
// * chiude il server web principale
func run() error {
	// Carica Configurazione e default
	cfg, err := loadConfiguration()
	if err != nil {
		if errors.Is(err, conf.ErrHelpWanted) {
			return nil
		}
		return err
	}

	// Inizializza logging
	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	if cfg.Debug {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	logger.Infof("application initializing")

	// Avvia Database
	logger.Println("initializing database support")
	dbconn, err := sql.Open("sqlite", cfg.DB.Filename)
	if err != nil {
		logger.WithError(err).Error("error opening SQLite DB")
		return fmt.Errorf("opening SQLite: %w", err)
	}
	defer func() {
		logger.Debug("database stopping")
		_ = dbconn.Close()
	}()
	db, err := database.New(dbconn)
	if err != nil {
		logger.WithError(err).Error("error creating AppDatabase")
		return fmt.Errorf("creating AppDatabase: %w", err)
	}

	// Avvia server API (principale)
	logger.Info("initializing API server")

	// Crea un canale per ascoltare un segnale di interruzione o terminazione dal SO.
	// Usa un canale bufferizzato perché il pacchetto signal lo richiede.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Crea un canale per ascoltare errori provenienti dal listener. Usa un
	// canale bufferizzato in modo che la goroutine possa uscire se non raccogliamo questo errore.
	serverErrors := make(chan error, 1)

	// Crea il router API
	apirouter, err := api.New(api.Config{
		Logger:   logger,
		Database: db,
	})
	if err != nil {
		logger.WithError(err).Error("error creating the API server instance")
		return fmt.Errorf("creating the API server instance: %w", err)
	}
	router := apirouter.Handler()

	router, err = registerWebUI(router)
	if err != nil {
		logger.WithError(err).Error("error registering web UI handler")
		return fmt.Errorf("registering web UI handler: %w", err)
	}

	// Applica policy CORS
	router = applyCORSHandler(router)

	// Crea il server API
	apiserver := http.Server{
		Addr:              cfg.Web.APIHost,
		Handler:           router,
		ReadTimeout:       cfg.Web.ReadTimeout,
		ReadHeaderTimeout: cfg.Web.ReadTimeout,
		WriteTimeout:      cfg.Web.WriteTimeout,
	}

	// Avvia il servizio in ascolto delle richieste in una goroutine separata
	go func() {
		logger.Infof("API listening on %s", apiserver.Addr)
		serverErrors <- apiserver.ListenAndServe()
		logger.Infof("stopping API server")
	}()

	// Attende segnale di shutdown o segnali POSIX
	select {
	case err := <-serverErrors:
		// Errore server non recuperabile
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		logger.Infof("signal %v received, start shutdown", sig)

		// Chiede al server API di spegnersi e scaricare il carico.
		err := apirouter.Close()
		if err != nil {
			logger.WithError(err).Warning("graceful shutdown of apirouter error")
		}

		// Dà alle richieste in sospeso una scadenza per il completamento.
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Web.ShutdownTimeout)
		defer cancel()

		// Chiede al listener di spegnersi e scaricare il carico.
		err = apiserver.Shutdown(ctx)
		if err != nil {
			logger.WithError(err).Warning("error during graceful shutdown of HTTP server")
			err = apiserver.Close()
		}

		// Logga lo stato di questo shutdown.
		if err != nil {
			return fmt.Errorf("could not stop server gracefully: %w", err)
		}
	}

	return nil
}
