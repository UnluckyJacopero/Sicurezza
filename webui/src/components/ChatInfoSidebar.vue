<script>
export default {
    // Props: chat attiva, membri del gruppo, media condivisi
    props: ['activeChat', 'groupMembers', 'sharedMedia', 'myId'],
    emits: ['close', 'leave-group', 'add-member', 'update-group'],
    data() {
        return {
            editGroupName: "" // Variabile per la modifica del nome gruppo
        }
    },
    watch: {
        // Aggiorna il nome modificabile quando cambia la chat attiva
        activeChat: {
            immediate: true,
            handler(newVal) {
                if (newVal) this.editGroupName = newVal.name;
            }
        }
    },
    methods: {
        // Salva il nuovo nome del gruppo
        async saveGroupSettings() {
            try {
                await this.$axios.put(`/users/${this.myId}/groups/${this.activeChat.conversation_id}/groupname`, { name: this.editGroupName });
                // Notifica il padre dell'aggiornamento
                this.$emit('update-group', { ...this.activeChat, name: this.editGroupName });
                alert("Gruppo aggiornato");
            } catch (e) {
                alert("Errore: " + e.toString());
            }
        },
        // Gestisce il caricamento e aggiornamento della foto del gruppo
        async handleGroupPhotoUpload(event) {
            const file = event.target.files[0];
            if (!file) return;
            const reader = new FileReader();
            reader.onload = async (e) => {
                const photoBase64 = e.target.result.split(',')[1];
                try {
                    await this.$axios.put(`/users/${this.myId}/groups/${this.activeChat.conversation_id}/groupphoto`, { photo: photoBase64 });
                    this.$emit('update-group', { ...this.activeChat, photo: photoBase64 });
                    alert("Foto gruppo aggiornata");
                } catch (error) {
                    alert("Errore caricamento foto gruppo: " + error.toString());
                }
                // Resetta l'input file per permettere di ricaricare lo stesso file se necessario
                event.target.value = '';
            };
            reader.readAsDataURL(file);
        },
        // Rimuove la foto del gruppo
        async removeGroupPhoto() {
            if (!confirm("Sei sicuro di voler rimuovere la foto del gruppo?")) return;
            try {
                await this.$axios.put(`/users/${this.myId}/groups/${this.activeChat.conversation_id}/groupphoto`, { photo: "" });
                this.$emit('update-group', { ...this.activeChat, photo: "" });
                alert("Foto gruppo rimossa");
            } catch (error) {
                alert("Errore rimozione foto gruppo: " + error.toString());
            }
        }
    }
}
</script>

<template>
    <div class="sidebar-right bg-white border-start overflow-auto" style="width: 25%; min-width: 250px;">
        <div class="p-3 border-bottom d-flex align-items-center bg-light justify-content-between">
            <span class="fw-bold">Info Contatto / Gruppo</span>
            <button class="btn-close btn-sm" @click="$emit('close')"></button>
        </div>
        
        <div class="p-4 text-center border-bottom">
            <div class="avatar-circle mx-auto mb-3 border bg-secondary text-white shadow-sm" style="width: 120px; height: 120px; font-size: 3rem;">
                <img v-if="activeChat.photo" :src="'data:image/jpeg;base64,'+activeChat.photo" class="w-100 h-100 rounded-circle" style="object-fit: cover;">
                <span v-else>{{ (activeChat.name||'?').charAt(0) }}</span>
            </div>
            <h4 class="mb-1">{{ activeChat.name }}</h4>
            
            <div v-if="activeChat.is_group" class="mt-3 text-start">
                <label class="form-label small fw-bold text-success">Modifica Nome (Gruppo)</label>
                <div class="input-group input-group-sm mb-2">
                    <input type="text" class="form-control" v-model="editGroupName">
                    <button class="btn btn-outline-secondary" @click="saveGroupSettings">Salva</button>
                </div>
                <label class="btn btn-sm btn-outline-primary w-100 mt-1">
                    Cambia Foto Gruppo 
                    <input type="file" hidden accept="image/png, image/jpeg, image/jpg" @change="handleGroupPhotoUpload">
                </label>
                <button class="btn btn-sm btn-outline-danger w-100 mt-1" @click="removeGroupPhoto">
                    Rimuovi Foto Gruppo
                </button>
            </div>
        </div>

        <div class="p-3 border-bottom">
            <h6 class="text-muted small mb-3">MEDIA CONDIVISI</h6>
            <div class="d-flex flex-wrap gap-1">
                <div v-for="(m, i) in sharedMedia" :key="i" style="width: 31%; aspect-ratio: 1;">
                    <img :src="'data:image/jpeg;base64,'+m.body.photo" class="w-100 h-100 rounded border" style="object-fit: cover;">
                </div>
                <div v-if="sharedMedia.length === 0" class="text-muted small w-100 text-center py-2">Nessun media</div>
            </div>
        </div>

        <div v-if="activeChat.is_group" class="p-3 border-bottom">
            <div class="d-flex justify-content-between align-items-center mb-3">
                <h6 class="text-muted small m-0">PARTECIPANTI ({{ groupMembers.length }})</h6>
                <button class="btn btn-sm btn-outline-success" @click="$emit('add-member')">+ Aggiungi</button>
            </div>
            <div class="list-group list-group-flush">
                <div v-for="member in groupMembers" :key="member.user_id" class="list-group-item px-0 d-flex align-items-center border-0">
                    <div class="avatar-circle bg-light border me-2" style="width: 30px; height: 30px; font-size: 0.8rem;">
                        <img v-if="member.photo" :src="'data:image/jpeg;base64,'+member.photo" class="w-100 h-100 rounded-circle" style="object-fit: cover;">
                        <span v-else>{{ (member.username || '?').charAt(0).toUpperCase() }}</span>
                    </div>
                    <span class="small">{{ member.username }}</span>
                    <span v-if="member.user_id === myId" class="badge bg-secondary ms-auto">Tu</span>
                </div>
            </div>
        </div>

        <div class="p-3">
            <button class="btn btn-outline-danger w-100" @click="$emit('leave-group')">
                <i class="bi bi-box-arrow-right me-2"></i> {{ activeChat.is_group ? 'Abbandona Gruppo' : 'Elimina Chat' }}
            </button>
        </div>
    </div>
</template>

<style scoped>
.avatar-circle { width:40px; height:40px; border-radius:50%; display:flex; justify-content:center; align-items:center; }
</style>