import {createApp, reactive} from 'vue'
import App from './App.vue'
import router from './router'
import axios from './services/axios.js';
import ErrorMsg from './components/ErrorMsg.vue'
import LoadingSpinner from './components/LoadingSpinner.vue'

// Importazione dei file CSS globali per lo stile dell'applicazione
import './assets/dashboard.css'
import './assets/main.css'

// Creazione dell'istanza dell'applicazione Vue
const app = createApp(App)

// Configurazione di Axios come proprietà globale per poterlo utilizzare in tutti i componenti con this.$axios
app.config.globalProperties.$axios = axios;

// Registrazione dei componenti globali che possono essere usati ovunque senza doverli importare ogni volta
app.component("ErrorMsg", ErrorMsg);
app.component("LoadingSpinner", LoadingSpinner);

// Utilizzo del router per la gestione della navigazione
app.use(router)

// Montaggio dell'applicazione sull'elemento con id 'app' nel DOM
app.mount('#app')
