// =========================
// État global
// =========================
let currentUserId = null;
let currentContactId = null;

// =========================
// Utilitaire auth
// =========================
async function fetchWithAuth(url, options = {}) {
    const token = localStorage.getItem('session_token');
    options.headers = {
        'Authorization': token || '',
        ...(options.headers || {})
    };
    const response = await fetch(url, options);
    if (response.status === 401) {
        localStorage.removeItem('session_token');
        window.location.href = '/front/template/login.html';
        return;
    }
    return response;
}

// =========================
// Initialisation
// =========================
document.addEventListener('DOMContentLoaded', () => {
    const token = localStorage.getItem('session_token');
    if (!token) {
        window.location.href = '/front/template/login.html';
        return;
    }
    initDashboard();
    setupEventListeners();
});

async function initDashboard() {
    try {
        const response = await fetchWithAuth('/api/session');
        const data = await response.json();
        if (!data.success) {
            window.location.href = '/front/template/login.html';
            return;
        }
        currentUserId = Number(data.data.id);
        document.getElementById('username').textContent = data.data.user;
        document.getElementById('content').style.display = 'flex';
        loadContacts();
        loadInvitations();
        jokeFunc();
    } catch (error) {
        console.error('Erreur initialisation:', error);
    }
}

// =========================
// Event Listeners
// =========================
function setupEventListeners() {
    document.getElementById('showDialog').addEventListener('click', () => {
        document.getElementById('favDialog').showModal();
    });
    document.getElementById('cancelBtn').addEventListener('click', () => {
        document.getElementById('favDialog').close();
    });
    document.getElementById('formInvitation').addEventListener('submit', handleSendInvitation);
    document.getElementById('sendMessageForm').addEventListener('submit', handleSendMessage);
    document.getElementById('piece-jointe').addEventListener('click', () => {
        const menu = document.querySelector('.piece-jointe-menu');
        menu.style.display = menu.style.display === 'flex' ? 'none' : 'flex';
    });
}

// =========================
// Navigation mobile
// =========================
function goBackToContacts() {
    document.getElementById('content').classList.remove('contact-selected');
    currentContactId = null;
    document.getElementById('messageForm').style.display = 'none';
}

// =========================
// Contacts
// =========================
async function loadContacts() {
    try {
        const response = await fetchWithAuth('/api/contacts');
        const data = await response.json();
        const list = document.getElementById('contactsList');
        if (!data.data || data.data.length === 0) {
            list.innerHTML = '<p class="contactVide">Aucun contact</p>';
            return;
        }
        list.innerHTML = data.data.map(c => `
            <a href="#" onclick="selectContact(${c.id}, '${escapeHtml(c.username)}', this); return false;" class="contactItem ${currentContactId === c.id ? 'active' : ''}">
                <img src="/front/assets/images/avatar.jpg" alt="avatar">
                <span>${escapeHtml(c.username)}</span>
            </a>
        `).join('');
    } catch (e) { console.error('Erreur contacts:', e); }
}

async function selectContact(contactId, contactName, element) {
    currentContactId = contactId;
    document.querySelectorAll('.contactItem').forEach(i => i.classList.remove('active'));
    element.classList.add('active');
    document.getElementById('messageForm').style.display = 'block';
    const title = document.getElementById('discussionTitle');
    if (title) title.textContent = contactName;
    document.getElementById('content').classList.add('contact-selected');
    loadMessages(contactId);
}

// =========================
// Messages
// =========================
async function loadMessages(contactId) {
    try {
        const response = await fetchWithAuth(`/api/messages?contact=${contactId}`);
        const data = await response.json();
        const list = document.getElementById('messagesList');
        const messages = data.data || [];

        list.innerHTML = messages.map(msg => {
            const isMe = Number(msg.sender) === currentUserId;
            const actions = isMe ? `
                <div class="message-actions">
                    <button class="btn-edit-msg" onclick="editMessage(${msg.id}, this)" title="Modifier">✏️</button>
                    <button class="btn-delete-msg" onclick="deleteMessage(${msg.id})" title="Supprimer">🗑️</button>
                </div>` : '';
            return `
            <div class="message ${isMe ? 'emetteur' : 'recepteur'}" data-id="${msg.id}">
                <div class="message-content">
                    ${msg.filepath ? renderMedia(msg.filepath) : ''}
                    ${msg.content ? `<h3 class="messagesRemplis" data-content="${escapeHtml(msg.content)}">${escapeHtml(msg.content)}</h3>` : ''}
                </div>
                ${actions}
                <span class="date">${new Date(msg.timestamp).toLocaleString('fr-FR')}</span>
            </div>`;
        }).join('');
        list.scrollTop = list.scrollHeight;
    } catch (e) { console.error('Erreur messages:', e); }
}

// ===== MODIFIER UN MESSAGE =====
async function editMessage(messageId, btn) {
    const messageDiv = btn.closest('.message');
    const bubble = messageDiv.querySelector('.messagesRemplis');
    if (!bubble) return;

    const originalContent = bubble.dataset.content;

    // Remplace la bulle par un input
    bubble.style.display = 'none';
    const editArea = document.createElement('textarea');
    editArea.className = 'edit-textarea';
    editArea.value = originalContent;
    messageDiv.querySelector('.message-content').appendChild(editArea);
    editArea.focus();

    // Remplace les boutons action par Valider/Annuler
    const actions = messageDiv.querySelector('.message-actions');
    const originalActions = actions.innerHTML;
    actions.innerHTML = `
        <button class="btn-confirm-edit" onclick="confirmEdit(${messageId}, this)">✅</button>
        <button class="btn-cancel-edit" onclick="cancelEdit(this)">❌</button>
    `;

    // Stocke les données originales pour annulation
    actions.dataset.originalActions = originalActions;
    editArea.dataset.originalContent = originalContent;
}

async function confirmEdit(messageId, btn) {
    const messageDiv = btn.closest('.message');
    const editArea = messageDiv.querySelector('.edit-textarea');
    const newContent = editArea.value.trim();

    if (!newContent) {
        Swal.fire('Erreur', 'Le message ne peut pas être vide', 'error');
        return;
    }

    const res = await fetchWithAuth('/api/edit-message', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: `messageId=${messageId}&content=${encodeURIComponent(newContent)}`
    });
    const data = await res.json();

    if (data.success) {
        loadMessages(currentContactId);
    } else {
        Swal.fire('Erreur', data.error || 'Impossible de modifier', 'error');
    }
}

function cancelEdit(btn) {
    const messageDiv = btn.closest('.message');
    const editArea = messageDiv.querySelector('.edit-textarea');
    const bubble = messageDiv.querySelector('.messagesRemplis');
    const actions = messageDiv.querySelector('.message-actions');

    // Restaure l'état original
    editArea.remove();
    bubble.style.display = '';
    actions.innerHTML = actions.dataset.originalActions;
}

// ===== SUPPRIMER UN MESSAGE =====
async function deleteMessage(messageId) {
    const result = await Swal.fire({
        title: 'Supprimer ce message ?',
        text: 'Cette action est irréversible.',
        icon: 'warning',
        showCancelButton: true,
        confirmButtonColor: '#f44336',
        cancelButtonColor: '#aaa',
        confirmButtonText: 'Supprimer',
        cancelButtonText: 'Annuler'
    });

    if (!result.isConfirmed) return;

    const res = await fetchWithAuth('/api/delete-message', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: `messageId=${messageId}`
    });
    const data = await res.json();

    if (data.success) {
        loadMessages(currentContactId);
    } else {
        Swal.fire('Erreur', data.error || 'Impossible de supprimer', 'error');
    }
}

// =========================
// Envoi de message
// =========================
async function handleSendMessage(e) {
    e.preventDefault();
    if (!currentContactId) return;
    const formData = new FormData(e.target);
    formData.append('receiverId', currentContactId);
    const img = document.getElementById('uploadImage').files[0];
    const vid = document.getElementById('uploadVideo').files[0];
    if (img) formData.append('file', img);
    else if (vid) formData.append('file', vid);
    const res = await fetchWithAuth('/api/send-message', { method: 'POST', body: formData });
    const data = await res.json();
    if (data.success) {
        e.target.reset();
        document.querySelector('.piece-jointe-menu').style.display = 'none';
        document.getElementById('piece-jointe').style.backgroundColor = '';
        loadMessages(currentContactId);
    }
}

// =========================
// Invitations
// =========================
async function loadInvitations() {
    try {
        const res = await fetchWithAuth('/api/invitations');
        const data = await res.json();
        const list = document.getElementById('invitationsList');
        if (!list) return;
        const invitations = data.data || [];
        if (invitations.length === 0) {
            list.innerHTML = '<p class="contactVide">Aucune invitation</p>';
            return;
        }
        list.innerHTML = invitations.map(inv => `
            <div class="invitation-item">
                <div class="invitation-info">
                    <img src="/front/assets/images/avatar.jpg" alt="avatar">
                    <span>${escapeHtml(inv.senderUsername)}</span>
                </div>
                <div class="invitation-actions">
                    <button class="btn-accepter" onclick="respondInvitation(${inv.senderId}, 'accepter')">✓</button>
                    <button class="btn-refuser" onclick="respondInvitation(${inv.senderId}, 'refuser')">✗</button>
                </div>
            </div>
        `).join('');
    } catch (e) { console.error('Erreur invitations:', e); }
}

async function handleSendInvitation(e) {
    e.preventDefault();
    const pseudo = document.getElementById('nomNouveauContact').value.trim();
    if (!pseudo) return;
    const res = await fetchWithAuth('/api/send-invitation', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: `pseudoDestinataire=${encodeURIComponent(pseudo)}`
    });
    const data = await res.json();
    if (data.success) {
        Swal.fire('Succès', 'Invitation envoyée !', 'success');
        document.getElementById('favDialog').close();
        e.target.reset();
    } else {
        Swal.fire('Erreur', data.error || 'Utilisateur introuvable', 'error');
    }
}

async function respondInvitation(senderId, action) {
    const res = await fetchWithAuth('/api/invitation-response', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: `senderId=${senderId}&action=${encodeURIComponent(action)}`
    });
    const data = await res.json();
    if (data.success) {
        loadInvitations();
        loadContacts();
    }
}

// =========================
// Déconnexion
// =========================
window.deconnexion = async () => {
    await fetchWithAuth('/api/logout', { method: 'POST' });
    localStorage.removeItem('session_token');
    window.location.href = '/front/template/login.html';
};

// =========================
// Utilitaires
// =========================
function renderMedia(filepath) {
    const ext = filepath.split('.').pop().toLowerCase();
    const url = `/uploads/messages/${filepath}`;
    if (['jpg', 'jpeg', 'png', 'webp', 'gif'].includes(ext)) {
        return `<img src="${url}" alt="image" style="max-width:250px; border-radius:8px;">`;
    } else if (['mp4', 'webm'].includes(ext)) {
        return `<video src="${url}" controls style="max-width:250px; border-radius:8px;"></video>`;
    }
    return `<a href="${url}" target="_blank">📎 Fichier joint</a>`;
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.appendChild(document.createTextNode(text));
    return div.innerHTML;
}

// =========================
// Blague
// =========================
async function jokeFunc() {
    const jokeDiv = document.getElementById('jokeText');
    const res = await fetch('/api/joke');
    const data = await res.json();
    jokeDiv.innerHTML = `<p>${escapeHtml(data.data)}</p>`;
}