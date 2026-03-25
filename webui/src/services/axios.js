import axios from "axios";

// Creazione di un'istanza di Axios con configurazione di base
// __API_URL__ viene sostituito da Vite durante la build con l'URL del backend
const instance = axios.create({
	baseURL: __API_URL__,
	timeout: 1000 * 5 // Timeout di 5 secondi per le richieste
});

// Aggiungi un intercettore: prima di ogni richiesta, inserisce il token di autenticazione
instance.interceptors.request.use(
	(config) => {
		// Recupera il token salvato nel localStorage al momento del login
		const token = localStorage.getItem("token");
		if (token) {
			// Aggiunge l'header Authorization: Bearer <token>
			// Questo è il formato standard che il backend si aspetta per autenticare le richieste
			config.headers["Authorization"] = `Bearer ${token}`;
		}
		return config;
	},
	(error) => {
		// Gestione degli errori nella configurazione della richiesta
		return Promise.reject(error);
	}
);

export default instance;