<script>
// Importazione dei componenti figli per la sidebar, la finestra di chat e le info
import ChatSidebar from '../components/ChatSidebar.vue';
import ChatWindow from '../components/ChatWindow.vue';
import ChatInfoSidebar from '../components/ChatInfoSidebar.vue';

export default {
    components: {
        ChatSidebar,
        ChatWindow,
        ChatInfoSidebar
    },
    data() {
        return {
            // Dati dell'utente corrente recuperati dal localStorage
            myId: parseInt(localStorage.getItem('token')),
            username: localStorage.getItem('username'),

            // Stato dell'interfaccia utente
            showChatInfo: false, // Controlla la visibilità della sidebar delle info
            errormsg: null, // Messaggio di errore globale

            // Dati principali dell'applicazione
            conversations: [], // Lista delle conversazioni attive
            activeChat: null, // Conversazione attualmente selezionata
            chatMessages: [], // Messaggi della conversazione attiva
            groupMembers: [], // Lista dei membri del gruppo (se la chat attiva è un gruppo)

            // Gestione del polling per aggiornare i dati in tempo reale
            socket: null,

            // --- GESTIONE MODALI ---
            
            // Stato e dati per il modale di inoltro messaggi
            showForwardModal: false,
            msgToForward: null,
            forwardSearchQuery: "",
            selectedForwardDestinations: [],
            forwardDestinations: [],

            // Stato ricerca nuovi utenti per inoltro
            forwardUserSearchQuery: "",
            usersFoundForForward: [],

            // Stato e dati per il modale di aggiunta membri al gruppo
            showAddMemberModal: false,
            addMemberSearchQuery: "",
            usersFoundForAdd: [],
            selectedUsersToAdd: [],
        }
    },

    computed: {
        // Filtra la lista delle conversazioni per il modale di inoltro in base alla ricerca
        filteredForwardList() {
            const query = this.forwardSearchQuery.toLowerCase();
            const source = this.forwardDestinations.length ? this.forwardDestinations : this.conversations;
            return source.filter(c => (c.name || 'Chat').toLowerCase().includes(query));
        },
        // Estrae i media (foto) dai messaggi per mostrarli nella sidebar delle info
        sharedMedia() {
            if (!this.chatMessages) return [];
            return this.chatMessages.filter(m => m.body && m.body.photo);
        }
    },

    methods: {
        // Effettua il logout dell'utente
        logout() {
            this.closeWebSocket(); // Chiude il WebSocket
            localStorage.removeItem("token"); // Rimuove il token
            localStorage.removeItem("username"); // Rimuove il nome utente
            this.$router.push("/"); // Reindirizza alla home (login)
        },

        updateUsername(newName) {
            this.username = newName;
        },

        // Aggiorna la lista delle conversazioni chiamando l'API
        async refreshConversations() {
            try {
                let response = await this.$axios.get(`/users/${this.myId}/conversations`);
                this.conversations = response.data.conversations || [];

                // Se c'è una chat attiva, controlla se esiste ancora
                if (this.activeChat) {
                    const stillExists = this.conversations.find(c => c.conversation_id === this.activeChat.conversation_id);
                    if (!stillExists) {
                        // Se la chat è stata cancellata, deselezionala
                        this.activeChat = null;
                        this.chatMessages = [];
                    } else {
                        // Aggiorna l'oggetto activeChat con i dati più recenti
                        this.activeChat = stillExists;
                    }
                }
                
                // Aggiorna la lista per il modale di inoltro se è vuota
                if (this.showForwardModal && this.forwardDestinations.length === 0) {
                    this.forwardDestinations = this.conversations;
                }
            } catch (e) {
                console.error("Errore refresh conversazioni", e);
            }
        },

        // Aggiorna i messaggi della chat attiva
        async refreshChatMessages() {
            if (!this.activeChat) return;
            try {
                let response = await this.$axios.get(`/users/${this.myId}/conversations/${this.activeChat.conversation_id}`);
                this.chatMessages = response.data.messages || [];
                
                // Se è un gruppo, aggiorna anche la lista dei partecipanti
                if (response.data.participants && response.data.participants.users) {
                    this.groupMembers = response.data.participants.users;
                }
                // Aggiorna lo stato is_group se presente nella risposta
                if (response.data.is_group !== undefined) {
                    this.activeChat.is_group = response.data.is_group;
                }
            } catch (e) {
                console.error("Errore polling messaggi:", e);
            }
        },

        // Segna la conversazione attiva come letta
        async markAsRead() {
            if (!this.activeChat) return;
            try {
                await this.$axios.put(`/users/${this.myId}/conversations/${this.activeChat.conversation_id}/seen`);
            } catch (e) {
                console.error("Error marking as read:", e);
            }
        },

        // Inizializza la connessione WebSocket
        initWebSocket() {
            let apiUrl = __API_URL__;
            if (apiUrl.endsWith('/')) {
                apiUrl = apiUrl.slice(0, -1);
            }
            const wsUrl = apiUrl.replace(/^http/, 'ws') + `/stream?token=${this.myId}`;
            
            this.socket = new WebSocket(wsUrl);
            
            this.socket.onopen = () => {
                // console.log("WebSocket connected");
            };
            
            this.socket.onmessage = (event) => {
                try {
                    const msg = JSON.parse(event.data);
                    this.handleWebSocketMessage(msg);
                } catch (e) {
                    console.error("Error parsing WebSocket message:", e);
                }
            };
            
            this.socket.onclose = () => {
                // console.log("WebSocket disconnected");
                // Tentativo di riconnessione dopo 3 secondi se non è stato chiuso intenzionalmente
                if (this.socket) {
                    setTimeout(() => this.initWebSocket(), 3000);
                }
            };
            
            this.socket.onerror = (error) => {
                console.error("WebSocket error:", error);
            };
        },

        handleWebSocketMessage(msg) {
            if (msg.type === 'NEW_MESSAGE') {
                const message = msg.payload;
                // Aggiorna la lista delle conversazioni (per mostrare nuovi messaggi/ordinamento)
                this.refreshConversations();
                
                // Se il messaggio appartiene alla chat attiva, aggiungilo
                if (this.activeChat && this.activeChat.conversation_id == message.conversation_id) {
                    // Verifica duplicati
                    const exists = this.chatMessages.find(m => m.message_id == message.message_id);
                    if (!exists) {
                        this.chatMessages.push(message);
                        // Segna immediatamente come letto se la chat è aperta
                        this.markAsRead();
                    }
                }
            } else if (msg.type === 'CONVERSATION_UPDATED') {
                this.refreshConversations();
                // Se la conversazione aggiornata è quella attiva, ricarica i messaggi
                // (utile per reazioni, cancellazioni, cambi nome gruppo, ecc.)
                if (this.activeChat && msg.payload && msg.payload.conversation_id == this.activeChat.conversation_id) {
                    // Se l'azione è 'messages_read' e non siamo noi il lettore, aggiorniamo lo stato
                    if (msg.payload.action === 'messages_read' && msg.payload.reader_id != this.myId) {
                        this.refreshChatMessages();
                    } else if (msg.payload.action !== 'messages_read') {
                        // Per altre azioni (es. reazioni, cancellazioni), ricarica sempre
                        this.refreshChatMessages();
                    }
                }
            }
        },

        // Chiude la connessione WebSocket
        closeWebSocket() {
            if (this.socket) {
                const ws = this.socket;
                this.socket = null; // Previene riconnessione automatica
                ws.close();
            }
        },

        // Gestisce la selezione di una chat dalla sidebar
        async selectChat(conversation) {
            this.activeChat = conversation;
            this.chatMessages = []; 
            this.errormsg = null;
            this.showChatInfo = false; // Chiude la sidebar info quando si cambia chat

            await this.refreshChatMessages();
        },

        // Invia un messaggio (chiamato da ChatWindow)
        async sendMessage(payload) {
            try {
                await this.$axios.post(`/users/${this.myId}/conversations/${this.activeChat.conversation_id}/messages`, 
                    payload
                );
                
                await this.refreshChatMessages();
                this.refreshConversations(); // Aggiorna per mostrare l'ultimo messaggio nella sidebar

            } catch (e) {
                this.errormsg = "Errore invio: " + e.toString();
            }
        },

        // Elimina un messaggio
        async deleteMessage(msg) {
            if (!confirm("Vuoi eliminare questo messaggio?")) return;
            try {
                await this.$axios.delete(`/users/${this.myId}/conversations/${this.activeChat.conversation_id}/messages/${msg.message_id}`);
                // Rimuove localmente il messaggio per un feedback immediato
                this.chatMessages = this.chatMessages.filter(m => m.message_id !== msg.message_id);
                await this.refreshChatMessages();
                await this.refreshConversations();
            } catch (e) {
                this.errormsg = "Errore cancellazione: " + e.toString();
            }
        },

        // Abbandona o cancella una chat/gruppo
        async leaveGroup() {
            if (!confirm("Vuoi abbandonare/cancellare questa chat?")) return;
            try {
                await this.$axios.delete(`/users/${this.myId}/groups/${this.activeChat.conversation_id}`);
                this.activeChat = null;
                this.chatMessages = [];
                this.showChatInfo = false;
                await this.refreshConversations();
            } catch (e) {
                this.errormsg = "Impossibile abbandonare: " + e.toString();
            }
        },

        // Mostra/Nasconde la sidebar delle informazioni chat
        toggleChatInfo() {
            this.showChatInfo = !this.showChatInfo;
        },

        // Aggiorna le info della chat attiva (es. dopo cambio nome gruppo)
        updateActiveChatInfo(updatedChatInfo) {
            this.activeChat = { ...this.activeChat, ...updatedChatInfo };
            this.refreshConversations();
        },

        // Apre il modale per inoltrare un messaggio
        openForwardModal(msg) {
            this.msgToForward = msg;
            this.forwardSearchQuery = "";
            
            this.forwardUserSearchQuery = "";
            this.usersFoundForForward = [];
            
            this.forwardDestinations = this.conversations;
            this.showForwardModal = true;
            this.selectedForwardDestinations = [];
        },
        
        // Cerca utenti per inoltro
        async searchUsersToForward() {
            if (this.forwardUserSearchQuery.length < 1) {
                this.usersFoundForForward = [];
                return;
            }
            try {
                let response = await this.$axios.get(`/users`, {
                    params: { found_user: this.forwardUserSearchQuery }
                });
                let allUsers = response.data.users || [];
                // Esclude se stesso
                this.usersFoundForForward = allUsers.filter(u => u.user_id != this.myId);
            } catch (e) {
                console.error(e);
            }
        },

        // Seleziona/Deseleziona utente esterno per inoltro
        toggleForwardUserSelection(user) {
            const existingIndex = this.selectedForwardDestinations.findIndex(c => c.is_user_target && c.user_id === user.user_id);
            if (existingIndex !== -1) {
                this.selectedForwardDestinations.splice(existingIndex, 1);
            } else {
                this.selectedForwardDestinations.push({
                    conversation_id: null,
                    name: user.username,
                    photo: user.photo,
                    user_id: user.user_id, 
                    is_user_target: true
                });
            }
        },

        // Seleziona/Deseleziona una chat per l'inoltro
        toggleForwardSelection(conversation) {
            const index = this.selectedForwardDestinations.findIndex(c => !c.is_user_target && c.conversation_id === conversation.conversation_id);
            if (index === -1) {
                this.selectedForwardDestinations.push(conversation);
            } else {
                this.selectedForwardDestinations.splice(index, 1);
            }
        },

        // Esegue l'inoltro del messaggio alle chat selezionate
        performForward() {
            if (this.selectedForwardDestinations.length === 0) return;

            if (!confirm(`Sei sicuro di voler inoltrare questo messaggio a ${this.selectedForwardDestinations.length} chat?`)) {
                return;
            }

            // 1. Cattura i dati necessari (snapshot) per evitare problemi se l'utente cambia contesto
            const destinations = [...this.selectedForwardDestinations];
            const msgId = this.msgToForward.message_id;
            const sourceChatId = this.activeChat.conversation_id;
            
            // 2. Chiudi il modale immediatamente (Priorità al mandante)
            this.showForwardModal = false;
            this.selectedForwardDestinations = [];
            this.msgToForward = null;

            // 3. Elabora in background (Coda ordinata ai riceventi)
            (async () => {
                for (const dest of destinations) {
                    try {
                        let targetConvID = dest.conversation_id;
                        
                        // Se è un utente target, dobbiamo creare/recuperare la conversazione
                        if (dest.is_user_target) {
                             try {
                                 let resp = await this.$axios.put(`/users/${this.myId}/conversations/${dest.user_id}`, { body: {} });
                                 targetConvID = resp.data.conversation_id;
                                 this.refreshConversations();
                             } catch (e) {
                                  console.error(`Errore creazione chat con ${dest.name}:`, e);
                                  continue; // Salta questo
                             }
                        }

                        if (targetConvID) {
                            await this.$axios.post(`/users/${this.myId}/conversations/${sourceChatId}/messages/${msgId}`, 
                                {}, { params: { destination_id: targetConvID } }
                            );
                        }
                    } catch (e) {
                        console.error(`Errore durante l'inoltro a ${dest.name}:`, e);
                    }
                }
                // console.log("Inoltro multiplo completato");
            })();
        },

        // Apre il modale per aggiungere membri
        openAddMemberModal() {
            this.showAddMemberModal = true;
            this.addMemberSearchQuery = "";
            this.usersFoundForAdd = [];
            this.selectedUsersToAdd = [];
        },

        // Cerca utenti da aggiungere al gruppo
        async searchUsersToAdd() {
            if (this.addMemberSearchQuery.length < 1) {
                this.usersFoundForAdd = [];
                return;
            }
            try {
                let response = await this.$axios.get(`/users`, {
                    params: { found_user: this.addMemberSearchQuery }
                });
                let allUsers = response.data.users || [];
                // Filtra utenti già presenti nel gruppo o se stessi
                const currentMemberIds = this.groupMembers.map(m => m.user_id);
                this.usersFoundForAdd = allUsers.filter(u => 
                    u.user_id != this.myId && !currentMemberIds.includes(u.user_id)
                );
            } catch (e) {
                console.error(e);
            }
        },

        // Seleziona/Deseleziona utente da aggiungere
        toggleUserSelectionForAdd(user) {
            const index = this.selectedUsersToAdd.findIndex(u => u.user_id === user.user_id);
            if (index === -1) {
                this.selectedUsersToAdd.push(user);
            } else {
                this.selectedUsersToAdd.splice(index, 1);
            }
        },

        // Aggiunge i membri selezionati al gruppo
        async addMembersToGroup() {
            if (this.selectedUsersToAdd.length === 0) return;
            try {
                const payload = {
                    users: this.selectedUsersToAdd.map(u => ({ user_id: u.user_id }))
                };
                await this.$axios.put(`/users/${this.myId}/groups/${this.activeChat.conversation_id}`, payload);
                
                alert("Membri aggiunti!");
                this.showAddMemberModal = false;
                await this.refreshChatMessages();
            } catch (e) {
                this.errormsg = "Errore aggiunta membri: " + e.toString();
            }
        },
    },
    // Ciclo di vita
    mounted() {
        this.refreshConversations();
        this.initWebSocket();
    },
    beforeUnmount() {
        this.closeWebSocket();
    }
}
</script>

<template>
    <div class="container-fluid vh-100 d-flex flex-column p-0">
        
        <!-- Barra verde superiore decorativa -->
        <div class="green-bar"></div>

        <!-- Corpo principale dell'applicazione -->
        <div class="app-body d-flex shadow-lg mx-auto my-auto position-relative" style="height: 95%; width: 95%; max-width: 1600px; background: white; z-index: 1;">
            
            <!-- Sidebar sinistra con lista chat -->
            <ChatSidebar 
                :conversations="conversations" 
                :active-chat="activeChat" 
                :username="username" 
                :my-id="myId"
                @select-chat="selectChat"
                @logout="logout"
                @refresh-conversations="refreshConversations"
                @username-updated="updateUsername"
            />

            <!-- Finestra centrale della chat -->
            <ChatWindow 
                :active-chat="activeChat" 
                :chat-messages="chatMessages" 
                :my-id="myId"
                @toggle-info="toggleChatInfo"
                @leave-group="leaveGroup"
                @delete-message="deleteMessage"
                @send-message="sendMessage"
                @refresh-messages="refreshChatMessages"
                @open-forward="openForwardModal"
            />

            <!-- Sidebar destra con info chat (opzionale) -->
            <ChatInfoSidebar 
                v-if="showChatInfo && activeChat"
                :active-chat="activeChat"
                :group-members="groupMembers"
                :shared-media="sharedMedia"
                :my-id="myId"
                @close="toggleChatInfo"
                @leave-group="leaveGroup"
                @add-member="openAddMemberModal"
                @update-group="updateActiveChatInfo"
            />
        </div>

        <!-- Modale per Inoltro Messaggi -->
        <div v-if="showForwardModal" class="modal d-block" style="background: rgba(0,0,0,0.5)">
            <div class="modal-dialog modal-dialog-centered">
                <div class="modal-content">
                    <div class="modal-header py-2 bg-success text-white">
                        <h6 class="modal-title">Inoltra messaggio a...</h6>
                        <button class="btn-close btn-close-white" @click="showForwardModal=false"></button>
                    </div>
                    <div class="modal-body p-2">
                        <h6 class="text-secondary small border-bottom pb-1">Chat recenti</h6>
                        <input type="text" class="form-control form-control-sm mb-2" placeholder="Cerca chat..." v-model="forwardSearchQuery">
                        
                        <div class="list-group list-group-flush mb-3" style="max-height: 150px; overflow-y: auto;">
                            <div v-for="c in filteredForwardList" :key="c.conversation_id" 
                                    class="list-group-item list-group-item-action d-flex align-items-center cursor-pointer"
                                    @click="toggleForwardSelection(c)">
                                <input type="checkbox" class="form-check-input me-3" 
                                        :checked="selectedForwardDestinations.some(sel => !sel.is_user_target && sel.conversation_id === c.conversation_id)" readonly>
                                <div class="avatar-circle bg-light border me-2" style="width: 24px; height: 24px; font-size: 0.7rem;">
                                    <span v-if="!c.photo">{{ (c.name || '?').charAt(0).toUpperCase() }}</span>
                                    <img v-else :src="'data:image/jpeg;base64,'+c.photo" class="w-100 h-100 rounded-circle">
                                </div>
                                <span>{{ c.name }}</span>
                            </div>
                        </div>

                        <h6 class="text-secondary small border-bottom pb-1 mt-2">Cerca utenti</h6>
                        <input type="text" class="form-control form-control-sm mb-2" placeholder="Cerca persone..." v-model="forwardUserSearchQuery" @input="searchUsersToForward">
                        <div class="list-group list-group-flush mb-3" style="max-height: 150px; overflow-y: auto;">
                            <div v-for="u in usersFoundForForward" :key="u.user_id" 
                                    class="list-group-item list-group-item-action d-flex align-items-center cursor-pointer"
                                    @click="toggleForwardUserSelection(u)">
                                <input type="checkbox" class="form-check-input me-3" 
                                        :checked="selectedForwardDestinations.some(sel => sel.is_user_target && sel.user_id === u.user_id)" readonly>
                                <div class="avatar-circle bg-light border me-2" style="width: 24px; height: 24px; font-size: 0.7rem;">
                                    <img v-if="u.photo" :src="'data:image/jpeg;base64,'+u.photo" class="w-100 h-100 rounded-circle" style="object-fit: cover;">
                                    <span v-else>{{ (u.username || '?').charAt(0).toUpperCase() }}</span>
                                </div>
                                <span>{{ u.username }}</span>
                            </div>
                            <div v-if="usersFoundForForward.length === 0 && forwardUserSearchQuery.length >= 3" class="text-muted small text-center p-2">
                                Nessun utente trovato.
                            </div>
                        </div>

                        <div class="d-flex justify-content-end border-top pt-2">
                            <button class="btn btn-light btn-sm me-2" @click="showForwardModal=false">Annulla</button>
                            <button class="btn btn-success btn-sm position-relative" @click="performForward" :disabled="selectedForwardDestinations.length===0">
                                Invia ({{ selectedForwardDestinations.length }})
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Modale per Aggiunta Membri -->
        <div v-if="showAddMemberModal" class="modal d-block" style="background: rgba(0,0,0,0.5)">
            <div class="modal-dialog modal-dialog-centered">
                <div class="modal-content">
                    <div class="modal-header py-2 bg-success text-white">
                        <h6 class="modal-title">Aggiungi membri al gruppo</h6>
                        <button class="btn-close btn-close-white" @click="showAddMemberModal=false"></button>
                    </div>
                    <div class="modal-body p-2">
                        <input type="text" class="form-control form-control-sm mb-2" placeholder="Cerca utenti..." v-model="addMemberSearchQuery" @input="searchUsersToAdd">
                        
                        <div class="list-group list-group-flush mb-3" style="max-height: 250px; overflow-y: auto;">
                            <div v-for="u in usersFoundForAdd" :key="u.user_id" 
                                    class="list-group-item list-group-item-action d-flex align-items-center cursor-pointer"
                                    @click="toggleUserSelectionForAdd(u)">
                                <input type="checkbox" class="form-check-input me-3" 
                                        :checked="selectedUsersToAdd.some(sel => sel.user_id === u.user_id)" readonly>
                                <div class="avatar-circle bg-light border me-2" style="width: 30px; height: 30px; font-size: 0.8rem;">
                                    <img v-if="u.photo" :src="'data:image/jpeg;base64,'+u.photo" class="w-100 h-100 rounded-circle" style="object-fit: cover;">
                                    <span v-else>{{ (u.username || '?').charAt(0).toUpperCase() }}</span>
                                </div>
                                <span>{{ u.username }}</span>
                            </div>
                            <div v-if="usersFoundForAdd.length === 0 && addMemberSearchQuery.length >= 3" class="text-muted small text-center p-2">
                                Nessun utente trovato o già presente.
                            </div>
                        </div>

                        <div class="d-flex justify-content-end border-top pt-2">
                            <button class="btn btn-light btn-sm me-2" @click="showAddMemberModal=false">Annulla</button>
                            <button class="btn btn-success btn-sm" @click="addMembersToGroup" :disabled="selectedUsersToAdd.length===0">
                                Aggiungi ({{ selectedUsersToAdd.length }})
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Alert per errori globali -->
        <div v-if="errormsg" class="alert alert-danger fixed-bottom m-3 w-auto d-inline-block shadow">{{ errormsg }} <button class="btn-close float-end ms-2" @click="errormsg=null"></button></div>
    </div>
</template>

<style scoped>
.vh-100 { height: 100vh; }
.green-bar { position: absolute; top:0; left:0; width:100%; height:127px; background: #00a884; z-index:0; }
.app-body { width: 95%; height: 95%; max-width:1600px; background: #fff; z-index:1; position: relative; }
.cursor-pointer { cursor: pointer; }
.avatar-circle { border-radius: 50%; display: flex; justify-content: center; align-items: center; }
</style>