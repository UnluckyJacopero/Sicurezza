<script>
export default {
    // Props ricevute dal componente padre (DashboardView)
    props: ['conversations', 'activeChat', 'username', 'myId'],
    // Eventi emessi verso il componente padre
    emits: ['select-chat', 'logout', 'refresh-conversations'],
    mounted() {
        this.fetchMyProfile();
    },
    data() {
        return {
            // Stato per la gestione delle impostazioni profilo
            showProfileSettings: false,
            
            // Stato per la creazione di un nuovo gruppo
            isCreatingGroup: false,
            
            // Variabili temporanee per le modifiche al profilo
            newUsername: "",
            newProfilePhoto: null,
            newProfilePhotoPreview: null,
            myPhoto: null,
            
            // Variabili per la creazione del gruppo
            newGroupName: "",
            userSearchQuery: "",
            usersFound: [],
            selectedUsersForGroup: [],
            
            errormsg: null
        }
    },
    methods: {
        // Attiva/Disattiva la modalità modifica profilo
        toggleProfileSettings() {
            this.showProfileSettings = !this.showProfileSettings;
            if (this.showProfileSettings) {
                // Inizializza i campi con i valori attuali
                this.newUsername = this.username;
                this.newProfilePhotoPreview = null;
                this.newProfilePhoto = null;
                this.fetchMyProfile();
            }
        },
        // Recupera il profilo corrente (per mostrare la foto attuale)
        async fetchMyProfile() {
            try {
                let response = await this.$axios.get(`/users`, {
                    params: { found_user: this.username }
                });
                let users = response.data.users || [];
                let me = users.find(u => u.user_id == this.myId);
                if (me && me.photo) {
                    this.myPhoto = 'data:image/jpeg;base64,' + me.photo;
                    if (this.showProfileSettings) {
                        this.newProfilePhotoPreview = this.myPhoto;
                    }
                } else {
                    this.myPhoto = null;
                }
            } catch (e) {
                console.error("Error fetching profile", e);
            }
        },
        // Attiva/Disattiva la modalità creazione gruppo
        toggleCreateGroupMode() {
            this.isCreatingGroup = !this.isCreatingGroup;
            // Resetta i campi
            this.selectedUsersForGroup = [];
            this.newGroupName = "";
            this.userSearchQuery = "";
            this.usersFound = [];
        },
        // Formatta l'orario per visualizzarlo nella lista chat
        formatTime(isoString) {
            if (!isoString) return '';
            return new Date(isoString).toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'});
        },
        // Cerca utenti nel sistema (per iniziare chat o aggiungere a gruppo)
        async searchUsers() {
            if (this.userSearchQuery.length < 1) {
                this.usersFound = [];
                return;
            }
            try {
                let response = await this.$axios.get(`/users`, {
                    params: { found_user: this.userSearchQuery }
                });
                let allUsers = response.data.users || [];
                // Filtra se stessi dai risultati
                this.usersFound = allUsers.filter(u => u.user_id != this.myId);
            } catch (e) {
                console.error(e);
            }
        },
        // Avvia una chat singola con un utente
        async startChat(otherUser) {
            try {
                // PUT /users/:id/conversations/:otherId crea o recupera la chat
                await this.$axios.put(`/users/${this.myId}/conversations/${otherUser.user_id}`, { body: {} });
                this.userSearchQuery = "";
                this.usersFound = [];
                // Chiede al padre di aggiornare la lista conversazioni
                this.$emit('refresh-conversations');
            } catch (e) {
                alert("Errore creazione chat: " + e.toString());
            }
        },
        // Seleziona/Deseleziona utenti per il nuovo gruppo
        toggleUserSelection(user) {
            const index = this.selectedUsersForGroup.findIndex(u => u.user_id === user.user_id);
            if (index === -1) {
                this.selectedUsersForGroup.push(user);
            } else {
                this.selectedUsersForGroup.splice(index, 1);
            }
        },
        // Crea un nuovo gruppo
        async createGroup() {
            if (!this.newGroupName.trim() || this.selectedUsersForGroup.length === 0) return;
            try {
                const membersPayload = {
                    users: this.selectedUsersForGroup.map(u => ({ user_id: u.user_id }))
                };
                await this.$axios.post(`/users/${this.myId}/groups`, {
                    name: this.newGroupName,
                    members: membersPayload
                });
                this.toggleCreateGroupMode();
                this.$emit('refresh-conversations');
                alert("Gruppo creato!");
            } catch (e) {
                alert("Errore creazione gruppo: " + e.toString());
            }
        },
        // Gestisce il caricamento della foto profilo
        handleProfilePhotoUpload(event) {
            const file = event.target.files[0];
            if (!file) return;
            const reader = new FileReader();
            reader.onload = (e) => {
                this.newProfilePhotoPreview = e.target.result;
                // Estrae la parte base64
                this.newProfilePhoto = e.target.result.split(',')[1];
                // Resetta l'input file
                event.target.value = '';
            };
            reader.readAsDataURL(file);
        },
        // Rimuove la foto profilo
        removeProfilePhoto() {
            this.newProfilePhoto = "";
            this.newProfilePhotoPreview = "";
        },
        // Salva le modifiche al profilo (nome e foto)
        async saveProfile() {
            try {
                if (this.newUsername !== this.username) {
                    await this.$axios.put(`/users/${this.myId}/username`, { name: this.newUsername });
                    localStorage.setItem('username', this.newUsername);
                    this.$emit('username-updated', this.newUsername);
                }
                if (this.newProfilePhoto !== null) {
                    await this.$axios.put(`/users/${this.myId}/photo`, { photo: this.newProfilePhoto });
                    if (this.newProfilePhoto) {
                        this.myPhoto = 'data:image/jpeg;base64,' + this.newProfilePhoto;
                    } else {
                        this.myPhoto = null;
                    }
                }
                alert("Profilo aggiornato! Ricarica la pagina per vedere le modifiche.");
                this.toggleProfileSettings();
            } catch (e) {
                if (e.response && e.response.status === 409) {
                    alert("Errore: Il nome utente è già in uso. Scegline un altro.");
                } else {
                    alert("Errore: " + e.toString());
                }
            }
        }
    }
}
</script>

<template>
    <div class="sidebar d-flex flex-column border-end bg-white position-relative" style="width: 30%; min-width: 300px;">
        <div class="sidebar-header d-flex justify-content-between align-items-center p-3 bg-light border-bottom" style="height: 60px;">
            <div class="d-flex align-items-center cursor-pointer" @click="toggleProfileSettings">
                <div class="avatar-circle bg-secondary text-white me-2">
                    <img v-if="myPhoto" :src="myPhoto" class="w-100 h-100 rounded-circle" style="object-fit: cover;">
                    <span v-else>{{ username.charAt(0).toUpperCase() }}</span>
                </div>
                <span class="fw-bold text-truncate">{{ username }}</span>
            </div>
            <div>
                <button class="btn btn-link text-success p-0 me-3 fs-4 text-decoration-none" title="Nuovo Gruppo" @click="toggleCreateGroupMode"><strong>+</strong></button>
                <button class="btn btn-link text-secondary p-0 me-3" @click="toggleProfileSettings">⚙️</button>
                <button class="btn btn-link text-danger p-0" @click="$emit('logout')">🚪</button>
            </div>
        </div>

        <div v-if="showProfileSettings" class="p-4 flex-grow-1 overflow-auto bg-white">
            <h5 class="mb-3 text-success">Il tuo Profilo</h5>
            <div class="text-center mb-3">
                <div class="avatar-circle mx-auto mb-2 border" style="width: 80px; height: 80px; font-size: 2rem;">
                    <img v-if="newProfilePhotoPreview" :src="newProfilePhotoPreview" class="w-100 h-100 rounded-circle" style="object-fit: cover;">
                    <span v-else>{{ username.charAt(0).toUpperCase() }}</span>
                </div>
                <label class="btn btn-sm btn-outline-primary">
                    Cambia Foto <input type="file" hidden accept="image/png, image/jpeg, image/jpg" @change="handleProfilePhotoUpload">
                </label>
                <button class="btn btn-sm btn-outline-danger ms-2" @click="removeProfilePhoto">Rimuovi Foto</button>
            </div>
            <div class="mb-3">
                <label class="form-label text-success small fw-bold">Nome Utente</label>
                <input type="text" class="form-control" v-model="newUsername">
            </div>
            <button class="btn btn-success w-100" @click="saveProfile">Salva</button>
            <button class="btn btn-link w-100 text-secondary" @click="toggleProfileSettings">Annulla</button>
        </div>

        <div v-else-if="isCreatingGroup" class="d-flex flex-column flex-grow-1 bg-white overflow-hidden">
            <div class="p-3 border-bottom bg-light">
                <h6 class="text-success mb-2">Nuovo Gruppo</h6>
                <input type="text" class="form-control mb-2" placeholder="Nome gruppo" v-model="newGroupName">
                <input type="text" class="form-control form-control-sm" placeholder="Cerca persone..." v-model="userSearchQuery" @input="searchUsers">
            </div>
            
            <div class="flex-grow-1 overflow-auto p-2">
                <div v-for="u in usersFound" :key="u.user_id" class="d-flex align-items-center p-2 border-bottom">
                    <input type="checkbox" class="form-check-input me-3" 
                        :checked="selectedUsersForGroup.some(sel => sel.user_id === u.user_id)"
                        @change="toggleUserSelection(u)">
                    <div class="avatar-circle bg-light border me-2" style="width: 30px; height: 30px; font-size: 0.8rem;">
                        <img v-if="u.photo" :src="'data:image/jpeg;base64,'+u.photo" class="w-100 h-100 rounded-circle" style="object-fit: cover;">
                        <span v-else>{{ (u.username || '?').charAt(0).toUpperCase() }}</span>
                    </div>
                    <div>{{ u.username }}</div>
                </div>
                
                <div v-if="selectedUsersForGroup.length > 0" class="mt-3 border-top pt-2">
                    <small class="fw-bold">Selezionati ({{selectedUsersForGroup.length}})</small>
                    <div v-for="sel in selectedUsersForGroup" :key="'sel-'+sel.user_id">
                        {{ sel.username }} <button class="btn-close btn-sm float-end" @click="toggleUserSelection(sel)"></button>
                    </div>
                </div>
            </div>
            
            <div class="p-2 border-top d-flex gap-2">
                <button class="btn btn-light w-50" @click="toggleCreateGroupMode">Annulla</button>
                <button class="btn btn-success w-50" @click="createGroup">Crea</button>
            </div>
        </div>

        <div v-else class="d-flex flex-column flex-grow-1 overflow-hidden">
            <div class="p-2 border-bottom bg-white">
                <input type="text" class="form-control form-control-sm" placeholder="Cerca utente..." v-model="userSearchQuery" @input="searchUsers">
                <div v-if="usersFound.length > 0" class="list-group position-absolute w-100 shadow mt-1 start-0" style="z-index: 100;">
                    <button v-for="u in usersFound" :key="u.user_id" class="list-group-item list-group-item-action d-flex align-items-center" @click="startChat(u)">
                        <div class="avatar-circle bg-light border me-2" style="width: 30px; height: 30px; font-size: 0.8rem;">
                            <img v-if="u.photo" :src="'data:image/jpeg;base64,'+u.photo" class="w-100 h-100 rounded-circle" style="object-fit: cover;">
                            <span v-else>{{ (u.username || '?').charAt(0).toUpperCase() }}</span>
                        </div>
                        {{ u.username }}
                    </button>
                </div>
            </div>

            <div class="flex-grow-1 overflow-auto">
                <div v-if="conversations.length === 0" class="text-center text-muted p-4 small">Nessuna chat attiva</div>
                <div v-for="c in conversations" :key="c.conversation_id" 
                        class="chat-item p-3 border-bottom cursor-pointer d-flex align-items-center"
                        :class="{ 'active-chat': activeChat && activeChat.conversation_id == c.conversation_id }"
                        @click="$emit('select-chat', c)">
                    <div class="avatar-circle bg-light border me-3 flex-shrink-0">
                        <img v-if="c.photo" :src="'data:image/jpeg;base64,'+c.photo" class="w-100 h-100 rounded-circle" style="object-fit: cover;">
                        <span v-else>{{ (c.name || '?').charAt(0).toUpperCase() }}</span>
                    </div>
                    <div class="flex-grow-1 overflow-hidden">
                        <div class="d-flex justify-content-between">
                            <span class="fw-bold text-truncate">{{ c.name }}</span>
                            <small class="text-muted" style="font-size: 0.7rem" v-if="c.last_msg">{{ formatTime(c.last_msg.send_time) }}</small>
                        </div>
                        <div class="text-truncate small text-muted">
                            <span v-if="c.last_msg && c.last_msg.body && (c.last_msg.body.text || c.last_msg.body.photo)">
                                <span v-if="c.last_msg.sender_id == myId">Tu: </span>
                                {{ c.last_msg.body.text ? c.last_msg.body.text : '📷 Foto' }}
                            </span>
                            <span v-else>Nessun messaggio</span>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<style scoped>
.cursor-pointer { cursor: pointer; }
.avatar-circle { width:40px; height:40px; border-radius:50%; display:flex; justify-content:center; align-items:center; }
.active-chat { background: #f0f2f5; }
</style>