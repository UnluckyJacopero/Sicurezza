<script>
export default {
	data: function() {
		return {
			errormsg: null, // Messaggio di errore da mostrare all'utente
			loading: false, // Stato di caricamento per disabilitare il pulsante durante la richiesta
			username: "", // Variabile collegata all'input utente tramite v-model
		}
	},
	methods: {
		// Funzione asincrona per gestire il login
		async doLogin() {
			this.loading = true;
			this.errormsg = null;

			// Validazione lato client: il nome utente deve essere valido
			if (this.username.length < 3) {
				this.errormsg = "Il nome utente deve essere di almeno 3 caratteri.";
				this.loading = false;
				return;
			}

			try {
				// Chiamata all'API POST /session definita nel backend per creare una nuova sessione
				let response = await this.$axios.post("/session", {
					name: this.username
				});
				
				// Salva l'ID utente (token) e il nome nel localStorage del browser
				// Questo permette di mantenere la sessione attiva anche dopo il refresh della pagina
				localStorage.setItem("token", response.data.id);
				localStorage.setItem("username", this.username);

				// Reindirizza l'utente alla dashboard principale
				this.$router.push("/dashboard");
				
			} catch (e) {
				// Gestione degli errori restituiti dal server
				if (e.response && e.response.status === 400) {
					this.errormsg = "Richiesta non valida o nome troppo corto.";
				} else {
					this.errormsg = e.toString();
				}
			}
			this.loading = false;
		},
	},
}
</script>

<template>
	<!-- Container principale centrato verticalmente e orizzontalmente -->
	<div class="d-flex justify-content-center align-items-center vh-100">
		<div class="card p-4" style="width: 300px;">
			<h3 class="text-center mb-3">Login WasaTxt</h3>
			
			<div class="mb-3">
				<label for="username" class="form-label">Username</label>
				<!-- Input collegato alla variabile 'username' -->
				<!-- @keyup.enter permette di fare login premendo Invio -->
				<input 
					type="text" 
					id="username" 
					class="form-control" 
					v-model="username" 
					placeholder="Inserisci il tuo nome"
					@keyup.enter="doLogin"
				>
			</div>

			<!-- Pulsante di login, disabilitato durante il caricamento -->
			<button 
				type="button" 
				class="btn btn-primary w-100" 
				@click="doLogin"
				:disabled="loading"
			>
				<!-- Spinner di caricamento visibile solo se loading è true -->
				<span v-if="loading" class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span>
				Entra
			</button>

			<!-- Alert per mostrare eventuali errori -->
			<div v-if="errormsg" class="alert alert-danger mt-3" role="alert">
				{{ errormsg }}
			</div>
		</div>
	</div>
</template>

<style>
/* Classe di utilità per impostare l'altezza al 100% della viewport */
.vh-100 {
	height: 100vh;
}
</style>