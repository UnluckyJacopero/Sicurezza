/*
Package reqcontext contiene il contesto della richiesta. Ogni richiesta avrà la propria istanza di RequestContext riempita dal
codice middleware in api-context-wrapper.go (pacchetto genitore).

Ogni valore qui dovrebbe essere considerato valido solo per richiesta, con alcune eccezioni come il logger.
*/
package reqcontext

import (
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
)

// RequestContext è il contesto della richiesta, per parametri dipendenti dalla richiesta
type RequestContext struct {
	// ReqUUID è l'ID univoco della richiesta
	ReqUUID uuid.UUID

	// UserID è l'ID dell'utente autenticato
	UserID int64

	// Logger è un logger personalizzato per la richiesta
	Logger logrus.FieldLogger
}
