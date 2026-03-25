<script>
export default {
    // Props: chat attiva, lista messaggi, ID utente corrente
    props: ['activeChat', 'chatMessages', 'myId'],
    // Eventi per comunicare azioni al padre
    emits: ['toggle-info', 'leave-group', 'delete-message', 'send-message', 'refresh-messages', 'open-forward'],
    data() {
        return {
            newMessageText: "", // Testo del messaggio da inviare
            selectedPhoto: null, // Foto selezionata (base64)
            selectedPhotoPreview: null, // Anteprima foto
            replyingTo: null, // Messaggio a cui si sta rispondendo
            showScrollButton: false, // Visibilità pulsante scroll-to-bottom
            scrollInterval: null, // Intervallo per controllare lo scroll
            
            // Gestione reazioni personalizzate
            customEmojiInput: "",
            customReactionMsgId: null
        }
    },
    mounted() {
        // Controlla periodicamente se mostrare il pulsante di scroll
        this.scrollInterval = setInterval(this.handleScroll, 300);
    },
    beforeUnmount() {
        if (this.scrollInterval) clearInterval(this.scrollInterval);
    },
    methods: {
        formatTime(isoString) {
            if (!isoString) return '';
            return new Date(isoString).toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'});
        },
        // Scorre la chat fino all'ultimo messaggio
        scrollToBottom() {
            this.showScrollButton = false;
            this.$nextTick(() => {
                const container = this.$refs.messageContainer;
                if (container) container.scrollTop = container.scrollHeight;
            });
        },
        // Logica per mostrare/nascondere il pulsante "vai giù"
        handleScroll() {
            const container = this.$refs.messageContainer;
            if (container) {
                const isScrollable = container.scrollHeight > container.clientHeight;
                // Se l'utente è salito di più di 50px dal fondo
                const isNotAtBottom = (container.scrollHeight - container.scrollTop - container.clientHeight) > 50;
                this.showScrollButton = isScrollable && isNotAtBottom;
            }
        },
        // Apre il selettore file nascosto
        triggerFileInput() {
            this.$refs.fileInput.click();
        },
        // Gestisce il caricamento di un'immagine da allegare
        handleFileUpload(event) {
            const file = event.target.files[0];
            if (!file) return;
            const reader = new FileReader();
            reader.onload = (e) => {
                this.selectedPhotoPreview = e.target.result;
                this.selectedPhoto = e.target.result.split(',')[1];
            };
            reader.readAsDataURL(file);
        },
        // Annulla l'allegato
        cancelAttachment() {
            this.selectedPhoto = null;
            this.selectedPhotoPreview = null;
            if (this.$refs.fileInput) this.$refs.fileInput.value = null;
        },
        // Annulla la risposta a un messaggio
        cancelReply() {
            this.replyingTo = null;
        },
        // Helper per ottenere anteprima del messaggio citato
        getReplySnippet(replyId) {
            const targetMsg = this.chatMessages.find(m => m.message_id == replyId);
            if (!targetMsg) return "Messaggio non disponibile";
            return targetMsg.body.caption || targetMsg.body.text || "";
        },
        getReplyMessage(replyId) {
            return this.chatMessages.find(m => m.message_id == replyId);
        },
        getReplySender(replyId) {
            const targetMsg = this.chatMessages.find(m => m.message_id == replyId);
            if (!targetMsg) return "Utente";
            return targetMsg.sender_id == this.myId ? "Tu" : (targetMsg.sender_name || "Utente");
        },
        // Raggruppa le reazioni per tipo (emoji)
        getGroupedReactions(msg) {
            if (!msg.reactions || msg.reactions.length === 0) return [];
            const groups = {};
            msg.reactions.forEach(r => {
                if (!groups[r.emoticon]) {
                    groups[r.emoticon] = { emoji: r.emoticon, count: 0, userReacted: false, reactionId: null };
                }
                groups[r.emoticon].count++;
                if (r.user_id == this.myId) {
                    groups[r.emoticon].userReacted = true;
                    groups[r.emoticon].reactionId = r.reaction_id;
                }
            });
            return Object.values(groups);
        },
        // Aggiunge o rimuove una reazione
        async toggleReaction(msg, emoji) {
            if (msg.sender_id == this.myId) {
                alert("Non puoi reagire ai tuoi messaggi!");
                return;
            }

            // Cerca se l'utente ha già messo questa reazione
            const myReaction = msg.reactions?.find(r => r.user_id == this.myId && r.emoticon == emoji);

            try {
                if (myReaction) {
                    // Se la reazione esiste, usa DELETE per rimuoverla
                    await this.$axios.delete(`/users/${this.myId}/conversations/${this.activeChat.conversation_id}/messages/${msg.message_id}/reactions/${myReaction.reaction_id}`);
                } else {
                    // Altrimenti usa PUT per aggiungerla (o aggiornarla se diversa)
                    await this.$axios.put(`/users/${this.myId}/conversations/${this.activeChat.conversation_id}/messages/${msg.message_id}/reactions`, { emoticon: emoji });
                }
                this.$emit('refresh-messages');
            } catch (e) {
                if (e.response && e.response.status === 403) alert("Non puoi reagire ai tuoi messaggi!");
                else alert("Errore reazione: " + e.toString());
            }
        },
        // Gestione input reazione personalizzata
        openCustomReactionInput(msgId) {
            this.customReactionMsgId = msgId;
            this.customEmojiInput = "";
        },
        confirmCustomReaction(msg) {
            if (this.customEmojiInput && this.customEmojiInput.trim()) {
                this.toggleReaction(msg, this.customEmojiInput.trim());
            }
            this.customReactionMsgId = null;
            this.customEmojiInput = "";
        },
        // Invia il messaggio (testo e/o foto)
        async sendMessage() {
            if (!this.newMessageText.trim() && !this.selectedPhoto) return;
            
            let messageBody = {};
            if (this.selectedPhoto) {
                messageBody = { photo: this.selectedPhoto, caption: this.newMessageText };
            } else {
                messageBody = { text: this.newMessageText };
            }

            let payload = { body: messageBody };
            if (this.replyingTo) payload.reply_to = this.replyingTo.message_id;

            this.$emit('send-message', payload);
            
            this.newMessageText = "";
            this.cancelAttachment();
            this.cancelReply();
        }
    },
    watch: {
        chatMessages() {
            // Scroll automatico in basso all'arrivo di nuovi messaggi
            // Solo se l'utente non sta guardando la cronologia (pulsante scroll non visibile)
            if (!this.showScrollButton) {
                this.scrollToBottom();
            }
        }
    }
}
</script>

<template>
    <div class="main-chat d-flex flex-column flex-grow-1 position-relative bg-chat">
        
        <div v-if="!activeChat" class="h-100 d-flex flex-column align-items-center justify-content-center text-muted">
            <h3>WasaTxt Web</h3>
            <p>Seleziona una chat per iniziare</p>
        </div>

        <div v-else class="d-flex flex-column h-100 position-relative">
            <div class="chat-header p-2 px-3 bg-light border-bottom d-flex align-items-center justify-content-between" style="height: 60px;">
                <div class="d-flex align-items-center">
                    <button v-if="activeChat.is_group" class="btn btn-light btn-sm me-2" @click="$emit('toggle-info')" title="Impostazioni Gruppo">
                        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" fill="currentColor" viewBox="0 0 16 16">
                            <path d="M9.405 1.05c-.413-1.4-2.397-1.4-2.81 0l-.1.34a1.464 1.464 0 0 1-2.105.872l-.31-.17c-1.283-.698-2.686.705-1.987 1.987l.169.311c.446.82.023 1.841-.872 2.105l-.34.1c-1.4.413-1.4 2.397 0 2.81l.34.1a1.464 1.464 0 0 1 .872 2.105l-.17.31c-.698 1.283.705 2.686 1.987 1.987l.311-.169a1.464 1.464 0 0 1 2.105.872l.1.34c.413 1.4 2.397 1.4 2.81 0l.1-.34a1.464 1.464 0 0 1 2.105-.872l.31.17c1.283.698 2.686-.705 1.987-1.987l-.169-.311a1.464 1.464 0 0 1 .872-2.105l.34-.1c1.4-.413 1.4-2.397 0-2.81l-.34-.1a1.464 1.464 0 0 1-.872-2.105l.17-.31c.698-1.283-.705-2.686-1.987-1.987l-.311.169a1.464 1.464 0 0 1-2.105-.872l-.1-.34zM8 10.93a2.929 2.929 0 1 1 0-5.86 2.929 2.929 0 0 1 0 5.858z"/>
                        </svg>
                    </button>
                    <div class="d-flex align-items-center cursor-pointer" @click="$emit('toggle-info')">
                        <div class="avatar-circle bg-secondary text-white me-3" style="width: 40px; height: 40px;">
                            <img v-if="activeChat.photo" :src="'data:image/png;base64,'+activeChat.photo" class="rounded-circle w-100 h-100" style="object-fit: cover;">
                            <span v-else>{{ activeChat.name ? activeChat.name.charAt(0).toUpperCase() : '?' }}</span>
                        </div>
                        <div>
                            <h6 class="m-0">{{ activeChat.name || 'Chat' }}</h6>
                        </div>
                    </div>
                </div>
                <button class="btn btn-sm text-danger" @click="$emit('leave-group')">
                    <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" fill="currentColor" viewBox="0 0 16 16">
                        <path d="M5.5 5.5A.5.5 0 0 1 6 6v6a.5.5 0 0 1-1 0V6a.5.5 0 0 1 .5-.5zm2.5 0a.5.5 0 0 1 .5.5v6a.5.5 0 0 1-1 0V6a.5.5 0 0 1 .5-.5zm3 .5a.5.5 0 0 0-1 0v6a.5.5 0 0 0 1 0V6z"/>
                        <path fill-rule="evenodd" d="M14.5 3a1 1 0 0 1-1 1H13v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V4h-.5a1 1 0 0 1-1-1V2a1 1 0 0 1 1-1H6a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1h3.5a1 1 0 0 1 1 1v1zM4.118 4 4 4.059V13a1 1 0 0 0 1 1h6a1 1 0 0 0 1-1V4.059L11.882 4H4.118zM2.5 3V2h11v1h-11z"/>
                    </svg>
                    Elimina
                </button>
            </div>

            <div class="flex-grow-1 overflow-auto p-4 d-flex flex-column" ref="messageContainer" @scroll="handleScroll">
                <div v-for="msg in chatMessages" :key="msg.message_id" class="d-flex mb-3 w-100 position-relative group-message" 
                    :class="msg.sender_id == myId ? 'justify-content-end' : 'justify-content-start'">
                    
                    <div style="max-width: 70%; min-width: 120px;">
                        
                        <div class="message-bubble shadow-sm p-1 rounded position-relative d-flex flex-column" 
                            :class="msg.sender_id == myId ? 'msg-sent' : 'msg-received'">
                            
                            <div class="px-2 pt-1">
                                <div v-if="msg.forwarded" class="text-muted fst-italic mb-1 small">
                                    <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" fill="currentColor" viewBox="0 0 16 16">
                                        <path d="M5.921 11.9 1.353 8.62a.719.719 0 0 1 0-1.238L5.921 4.1A.716.716 0 0 1 7 4.719V6c1.5 0 6 0 7 8-2.5-4.5-7-4-7-4v1.281c0 .56-.606.898-1.079.62z"/>
                                    </svg>
                                    Inoltrato
                                </div>
                                <div v-if="msg.reply_to" class="reply-container p-1 mb-1 rounded border-start border-4 border-success bg-light text-start small cursor-pointer">
                                    <div class="d-flex gap-2 align-items-center">
                                        <div v-if="getReplyMessage(msg.reply_to) && getReplyMessage(msg.reply_to).body.photo" style="width: 50px; height: 50px;" class="flex-shrink-0">
                                            <img :src="'data:image/jpeg;base64,'+getReplyMessage(msg.reply_to).body.photo" class="w-100 h-100 rounded" style="object-fit: cover;">
                                        </div>
                                        <div class="fw-bold text-success">{{ getReplySender(msg.reply_to) }}</div>
                                    </div>
                                    <div class="text-truncate text-secondary" v-if="getReplySnippet(msg.reply_to)">{{ getReplySnippet(msg.reply_to) }}</div>
                                </div>
                                <div v-if="msg.sender_id != myId" class="text-danger small fw-bold">{{ msg.sender_name }}</div>
                            </div>

                            <div v-if="msg.body.photo" class="p-1">
                                <img :src="'data:image/jpeg;base64,'+msg.body.photo" class="img-fluid rounded">
                            </div>

                            <div v-if="msg.body.text || msg.body.caption" 
                                class="message-text px-2 pb-1 pt-1" 
                                style="white-space: pre-wrap; text-align: left;">
                                {{ msg.body.text || msg.body.caption }}
                            </div>

                            <div class="d-flex justify-content-end align-items-center pe-2 pb-1" style="font-size: 0.65rem;">
                                <span class="text-muted me-1">{{ formatTime(msg.send_time) }}</span>
                                <span v-if="msg.sender_id == myId" class="fw-bold" style="font-size: 0.85rem;" :class="msg.status === 'read' ? 'text-primary' : 'text-secondary'">                                    <!-- Logica visualizzazione spunte:
                                        - Inviato/Ricevuto (default): 1 spunta grigia (✓)
                                        - Letto (msg.status === 'read'): 2 spunte blu (✓✓)
                                    -->
                                    {{ msg.status === 'read' ? '✓✓' : '✓' }}
                                </span>
                            </div>

                            <div class="msg-actions dropdown position-absolute top-0 end-0 mt-1 me-1">
                                <button class="btn btn-sm p-0 text-secondary lh-1 bg-white rounded-circle shadow-sm px-1" data-bs-toggle="dropdown">⌄</button>
                                <ul class="dropdown-menu shadow">
                                    <li><a class="dropdown-item" href="#" @click.prevent="replyingTo = msg">Rispondi</a></li>
                                    <li><a class="dropdown-item" href="#" @click.prevent="$emit('open-forward', msg)">Inoltra</a></li>
                                    <li v-if="msg.sender_id == myId"><hr class="dropdown-divider"></li>
                                    <li v-if="msg.sender_id == myId"><a class="dropdown-item text-danger" href="#" @click.prevent="$emit('delete-message', msg)">Elimina</a></li>
                                </ul>
                            </div>
                        </div>

                        <div class="d-flex flex-wrap gap-1 mt-1" :class="msg.sender_id == myId ? 'justify-content-end' : 'justify-content-start'">
                            <button v-for="group in getGroupedReactions(msg)" :key="group.emoji"
                                    class="btn btn-sm py-0 px-2 border rounded-pill shadow-sm bg-white"
                                    :class="{'selected-reaction': group.userReacted}"
                                    @click="toggleReaction(msg, group.emoji)"
                                    title="Clicca per rimuovere/aggiungere">
                                {{ group.emoji }} <span v-if="group.count > 1" class="small fw-bold">{{ group.count }}</span>
                            </button>
                            
                            <div class="dropdown" v-if="msg.sender_id != myId">
                                <button class="btn btn-sm py-0 px-2 border rounded-pill bg-light text-muted shadow-sm add-react-btn" data-bs-toggle="dropdown">+</button>
                                <div class="dropdown-menu p-2 shadow" style="min-width: auto;">
                                    <div class="d-flex gap-2" v-if="customReactionMsgId !== msg.message_id">
                                        <button class="btn btn-light btn-sm fs-5 p-1" @click="toggleReaction(msg, '👍')">👍</button>
                                        <button class="btn btn-light btn-sm fs-5 p-1" @click="toggleReaction(msg, '❤️')">❤️</button>
                                        <button class="btn btn-light btn-sm fs-5 p-1" @click="toggleReaction(msg, '😂')">😂</button>
                                        <button class="btn btn-light btn-sm fs-5 p-1" @click="toggleReaction(msg, '😮')">😮</button>
                                        <button class="btn btn-light btn-sm fs-5 p-1" @click="toggleReaction(msg, '😢')">😢</button>
                                        <button class="btn btn-light btn-sm fs-5 p-1" @click.stop="openCustomReactionInput(msg.message_id)" title="Reazione personalizzata">➕</button>
                                    </div>
                                    <div v-else class="d-flex gap-1 align-items-center">
                                        <input type="text" class="form-control form-control-sm" 
                                            style="width: 80px;" 
                                            v-model="customEmojiInput" 
                                            @click.stop
                                            @keyup.enter="confirmCustomReaction(msg)"
                                            placeholder="Emoji"
                                            autofocus>
                                        <button class="btn btn-success btn-sm" @click.stop="confirmCustomReaction(msg)">OK</button>
                                        <button class="btn btn-outline-secondary btn-sm" @click.stop="customReactionMsgId = null">✖</button>
                                    </div>
                                </div>
                            </div>
                        </div>

                    </div>
                </div>
            </div>

            <button v-if="showScrollButton" 
                    class="btn btn-light shadow rounded-circle position-absolute d-flex align-items-center justify-content-center" 
                    style="bottom: 80px; right: 20px; z-index: 100; width: 40px; height: 40px;"
                    @click="scrollToBottom"
                    title="Vai all'ultimo messaggio">
                <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" fill="currentColor" viewBox="0 0 16 16">
                    <path fill-rule="evenodd" d="M8 1a.5.5 0 0 1 .5.5v11.793l3.146-3.147a.5.5 0 0 1 .708.708l-4 4a.5.5 0 0 1-.708 0l-4-4a.5.5 0 0 1 .708-.708L7.5 13.293V1.5A.5.5 0 0 1 8 1z"/>
                </svg>
            </button>

            <div class="p-2 bg-light border-top flex-shrink-0 position-relative">
                <div v-if="replyingTo" class="d-flex align-items-center mb-2">
                    <div class="flex-grow-1 p-1 rounded border-start border-4 border-success bg-light text-start small me-2">
                        <div class="d-flex gap-2 align-items-center">
                            <div v-if="replyingTo.body.photo" style="width: 50px; height: 50px;" class="flex-shrink-0">
                                <img :src="'data:image/jpeg;base64,'+replyingTo.body.photo" class="w-100 h-100 rounded" style="object-fit: cover;">
                            </div>
                            <div class="fw-bold text-success">{{ replyingTo.sender_id == myId ? 'Tu' : replyingTo.sender_name }}</div>
                        </div>
                        <div class="text-truncate text-secondary" v-if="replyingTo.body.text || replyingTo.body.caption">{{ replyingTo.body.text || replyingTo.body.caption }}</div>
                    </div>
                    <button class="btn-close" @click="cancelReply"></button>
                </div>
                
                <div v-if="selectedPhotoPreview" class="position-relative d-inline-block mb-2">
                    <img :src="selectedPhotoPreview" height="60" class="border rounded">
                    <button class="btn-close position-absolute top-0 end-0 bg-white" @click="cancelAttachment"></button>
                </div>

                <div class="input-group">
                    <button class="btn btn-link text-secondary" @click="triggerFileInput">📎</button>
                    <input type="file" ref="fileInput" hidden accept="image/*" @change="handleFileUpload">
                    <input type="text" class="form-control rounded-pill border-0 mx-1" placeholder="Scrivi un messaggio..." v-model="newMessageText" @keyup.enter="sendMessage">
                    <button class="btn btn-success rounded-circle" @click="sendMessage">➤</button>
                </div>
            </div>
        </div>
    </div>
</template>

<style scoped>
.bg-chat { background-color: #efeae2; background-image: url("https://user-images.githubusercontent.com/15075759/28719144-86dc0f70-73b1-11e7-911d-60d70fcded21.png"); }
.avatar-circle { width:40px; height:40px; border-radius:50%; display:flex; justify-content:center; align-items:center; }
.message-bubble { max-width: 100%; min-width: 150px; font-size: 14.2px; line-height: 19px; }
.msg-sent { background: #d9fdd3; border-radius: 7.5px 0 7.5px 7.5px; }
.msg-received { background: #fff; border-radius: 0 7.5px 7.5px 7.5px; }
.selected-reaction { border-color: #25d366 !important; background-color: #d9fdd3 !important; color: #000; }
.reaction-btn:hover, .add-react-btn:hover { background-color: #f0f0f0; }
.add-react-btn { opacity: 0; transition: opacity 0.2s; }
.group-message:hover .add-react-btn { opacity: 1; }
.msg-actions { opacity: 0; transition: opacity 0.2s; }
.message-bubble:hover .msg-actions { opacity: 1; }
.cursor-pointer { cursor: pointer; }
</style>