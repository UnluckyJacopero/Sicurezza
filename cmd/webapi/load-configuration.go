package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ardanlabs/conf"
	"gopkg.in/yaml.v2"
)

// WebAPIConfiguration descrive la configurazione della web API. Questa struttura viene analizzata automaticamente da
// loadConfiguration e verranno caricati i valori da flag, variabili d'ambiente o file di configurazione.
type WebAPIConfiguration struct {
	Config struct {
		Path string `conf:"default:/conf/config.yml"`
	}
	Web struct {
		APIHost         string        `conf:"default:0.0.0.0:3000"`
		DebugHost       string        `conf:"default:0.0.0.0:4000"`
		ReadTimeout     time.Duration `conf:"default:5s"`
		WriteTimeout    time.Duration `conf:"default:5s"`
		ShutdownTimeout time.Duration `conf:"default:5s"`
	}
	Debug bool
	DB    struct {
		Filename string `conf:"default:/tmp/decaf.db"`
	}
}

// loadConfiguration crea una WebAPIConfiguration partendo da flag, variabili d'ambiente e file di configurazione.
// Funziona caricando prima le variabili d'ambiente, poi aggiorna la configurazione usando i flag della riga di comando, infine caricando il
// file di configurazione (specificato in WebAPIConfiguration.Config.Path).
// Quindi, i parametri CLI sovrascriveranno l'ambiente, e il file di configurazione sovrascriverà tutto.
// Nota che il file di configurazione può essere specificato solo via CLI o variabile d'ambiente.
func loadConfiguration() (WebAPIConfiguration, error) {
	var cfg WebAPIConfiguration

	// Prova a caricare la configurazione da variabili d'ambiente e switch della riga di comando
	if err := conf.Parse(os.Args[1:], "CFG", &cfg); err != nil {
		if errors.Is(err, conf.ErrHelpWanted) {
			usage, err := conf.Usage("CFG", &cfg)
			if err != nil {
				return cfg, fmt.Errorf("generazione usage config: %w", err)
			}
			fmt.Println(usage) //nolint:forbidigo
			return cfg, conf.ErrHelpWanted
		}
		return cfg, fmt.Errorf("parsing config: %w", err)
	}

	// Sovrascrive i valori da YAML se specificato e se esiste (utile in k8s/compose)
	fp, err := os.Open(cfg.Config.Path)
	if err != nil && !os.IsNotExist(err) {
		return cfg, fmt.Errorf("impossibile leggere il file di config, anche se esiste: %w", err)
	} else if err == nil {
		yamlFile, err := io.ReadAll(fp)
		if err != nil {
			return cfg, fmt.Errorf("impossibile leggere il file di config: %w", err)
		}
		err = yaml.Unmarshal(yamlFile, &cfg)
		if err != nil {
			return cfg, fmt.Errorf("impossibile fare unmarshal del file di config: %w", err)
		}
		_ = fp.Close()
	}

	return cfg, nil
}
