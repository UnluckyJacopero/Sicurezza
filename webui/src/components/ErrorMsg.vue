<script>
export default {
	// Prop 'msg' che contiene il testo dell'errore da visualizzare
	props: ['msg']
}
</script>

<template>
	<!-- Componente riutilizzabile per mostrare messaggi di errore in stile alert -->
	<div class="alert alert-danger" role="alert">
		{{ msg }}
	</div>
</template>

<style>
</style>
