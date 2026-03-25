<script>
export default {
	// Prop 'loading' che determina se mostrare lo spinner o il contenuto
	props: ["loading"]
}
</script>

<template>
	<!-- Se loading è true, mostra lo spinner di caricamento -->
	<div v-if="loading">
		<div style="text-align: center">
			<div class="spinner-border" role="status">
				<span class="visually-hidden">Loading...</span>
			</div>
		</div>
	</div>
	<!-- Se loading è false, mostra il contenuto racchiuso nel componente (slot) -->
	<div v-if="!loading"><slot /></div>
</template>

<style></style>
