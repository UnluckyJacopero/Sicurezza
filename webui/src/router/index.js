import {createRouter, createWebHashHistory} from 'vue-router'
import HomeView from '../views/HomeView.vue'
import DashboardView from '../views/DashboardView.vue'

// Creazione del router utilizzando la history hash mode
// Questo permette di gestire la navigazione senza ricaricare la pagina
const router = createRouter({
	history: createWebHashHistory(import.meta.env.BASE_URL),
	routes: [
		// Rotta per la pagina di login (Home)
		{path: '/', component: HomeView},
		// Rotta per la dashboard principale dell'applicazione
		{path: '/dashboard', component: DashboardView},
	]
})

// Controlla se l'utente è autenticato prima di accedere a pagine protette
router.beforeEach((to, from, next) => {
	// Pagine accessibili a tutti (es. login)
	const publicPages = ['/'];
	const authRequired = !publicPages.includes(to.path);
	const loggedIn = localStorage.getItem('token');

	// Se la pagina richiede auth e non c'è il token, rimanda al login
	if (authRequired && !loggedIn) {
		return next('/');
	}

	next();
});

export default router