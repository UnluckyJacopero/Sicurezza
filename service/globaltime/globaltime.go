package globaltime

import "time"

// FixedTime rappresenta un momento fisso nel tempo. Imposta questa variabile a qualsiasi valore diverso dal default per
// time.Time e il valore verrà restituito nella funzione Now() al posto dell'ora corrente
var FixedTime time.Time

// Now restituisce l'ora corrente (time.Now()) se non è stato impostato FixedTime. Altrimenti, restituisce FixedTime.
// Usa questo al posto di time.Now() per permettere test con orari personalizzati.
func Now() time.Time {
	if FixedTime.After(time.Time{}) {
		return FixedTime
	}
	return time.Now()
}

// Since restituisce il tempo trascorso dal parametro tm.
func Since(tm time.Time) time.Duration {
	return Now().Sub(tm)
}
