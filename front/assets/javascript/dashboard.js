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
            <a href="#" onclick="selectContact(${c.id}, this); return false;" class="contactItem ${currentContactId === c.id ? 'active' : ''}">
                <img src="/front/assets/images/avatar.jpg" alt="avatar">
                <span>${escapeHtml(c.username)}</span>
            </a>
        `).join('');
    } catch (e) {
        console.error('Erreur contacts:', e);
    }
}

async function selectContact(contactId, element) {
    currentContactId = contactId;
    document.querySelectorAll('.contactItem').forEach(i => i.classList.remove('active'));
    element.classList.add('active');
    document.getElementById('messageForm').style.display = 'block';
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

        list.innerHTML = '<h2>Discussion</h2>' + messages.map(msg => {
            const isMe = Number(msg.sender) === currentUserId;
            return `
            <div class="message ${isMe ? 'emetteur' : 'recepteur'}">
                <div class="message-content">
                    ${msg.filepath ? renderMedia(msg.filepath) : ''}
                    ${msg.content ? `<h3 class="messagesRemplis">${escapeHtml(msg.content)}</h3>` : ''}
                </div>
                <span class="date">${new Date(msg.timestamp).toLocaleString('fr-FR')}</span>
            </div>
        `}).join('');
        list.scrollTop = list.scrollHeight;
    } catch (e) {
        console.error('Erreur messages:', e);
    }
}

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
    } catch (e) {
        console.error('Erreur invitations:', e);
    }
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