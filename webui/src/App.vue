<script setup>
import { RouterLink, RouterView } from 'vue-router'
</script>

<template>
    <!-- Contenitore principale dell'applicazione -->
    <main class="app-container">
        <!-- RouterView renderizza il componente corrispondente alla rotta attuale -->
        <RouterView />
    </main>
</template>

<style>
    /* Stili globali per assicurare che l'app occupi tutta l'altezza della finestra */
	html, body {
		height: 100%;
		margin: 0;
		padding: 0;
		overflow: hidden; /* Previene lo scroll della pagina intera */
	}
	.app-container {
		height: 100%;
		display: flex;
		flex-direction: column;
	}
</style>
