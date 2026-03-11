// =========================
// État global
// =========================
let currentUserId = null;
let currentContactId = null;

// =========================
// Initialisation au chargement
// =========================
document.addEventListener('DOMContentLoaded', () => {
    initDashboard();
    setupEventListeners();
});

async function initDashboard() {
    try {
        const response = await fetch('/api/session');
        const data = await response.json();
        
        if (!data.success) {
            window.location.href = '/front/template/login.html';
            return;
        }

        currentUserId = data.data.id;
        // Correction de l'affichage du nom
        document.getElementById('username').textContent = data.data.user;
        document.getElementById('content').style.display = 'flex'; // 'flex' pour respecter ton CSS
        
        loadContacts();
        loadInvitations();
    } catch (error) {
        console.error('Erreur initialisation:', error);
        window.location.href = '/front/template/login.html';
    }
}

// =========================
// Gestionnaires d'événements
// =========================
function setupEventListeners() {
    // Envoi de message
    document.getElementById('sendMessageForm')?.addEventListener('submit', handleSendMessage);

    // Modal Nouveau Contact
    const dialog = document.getElementById('favDialog');
    document.getElementById('showDialog')?.addEventListener('click', () => dialog.showModal());
    document.getElementById('cancelBtn')?.addEventListener('click', () => dialog.close());
    document.getElementById('formInvitation')?.addEventListener('submit', handleSendInvitation);

    // Menu pièces jointes
    const btnPJ = document.getElementById('piece-jointe');
    const menuPJ = document.querySelector('.piece-jointe-menu');
    btnPJ?.addEventListener('click', () => menuPJ.classList.toggle('piece-jointe-menu-ouvert'));

    // Auto-resize textarea
    const textarea = document.getElementById('messageRediger');
    textarea?.addEventListener('input', function() {
        this.style.height = 'auto';
        this.style.height = (this.scrollHeight) + 'px';
    });

    // Feedback visuel upload
    document.querySelectorAll('input[type="file"]').forEach(input => {
        input.addEventListener('change', function() {
            if (this.files && this.files[0]) {
                document.getElementById('piece-jointe').style.backgroundColor = '#4caf50';
                Swal.fire({ icon: 'info', title: 'Fichier prêt', text: this.files[0].name, timer: 1000, showConfirmButton: false });
            }
        });
    });
}

// =========================
// Fonctions API
// =========================

async function loadContacts() {
    try {
        const response = await fetch('/api/contacts');
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
    } catch (e) { console.error(e); }
}

async function selectContact(contactId, element) {
    currentContactId = contactId;
    document.querySelectorAll('.contactItem').forEach(i => i.classList.remove('active'));
    element.classList.add('active');
    document.getElementById('messageForm').style.display = 'block';
    loadMessages(contactId);
}

async function loadMessages(contactId) {
    try {
        const response = await fetch(`/api/messages?contact=${contactId}`);
        const data = await response.json();
        const list = document.getElementById('messagesList');
        
        list.innerHTML = '<h2>Discussion</h2>' + (data.data || []).map(msg => `
            <div class="message ${msg.sender === currentUserId ? 'emetteur' : 'recepteur'}">
                <div class="message-content">
                    ${msg.filepath ? renderMedia(msg.filepath) : ''}
                    ${msg.content && msg.content !== 'null' ? `<h3 class="messagesRemplis">${escapeHtml(msg.content)}</h3>` : ''}
                </div>
                <span class="date">${new Date(msg.timestamp).toLocaleString('fr-FR')}</span>
            </div>
        `).join('');
        list.scrollTop = list.scrollHeight;
    } catch (e) { console.error(e); }
}

async function handleSendMessage(e) {
    e.preventDefault();
    const formData = new FormData(e.target);
    formData.append('receiverId', currentContactId);

    const img = document.getElementById('uploadImage').files[0];
    const vid = document.getElementById('uploadVideo').files[0];
    if (img) { formData.append('file', img); formData.append('fileType', 'image'); }
    else if (vid) { formData.append('file', vid); formData.append('fileType', 'video'); }

    const res = await fetch('/api/send-message', { method: 'POST', body: formData });
    const data = await res.json();
    if (data.success) {
        e.target.reset();
        document.getElementById('piece-jointe').style.backgroundColor = '';
        loadMessages(currentContactId);
    }
}

async function handleSendInvitation(e) {
    e.preventDefault();
    const pseudo = document.getElementById('nomNouveauContact').value.trim();
    const res = await fetch('/api/send-invitation', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: `pseudoDestinataire=${encodeURIComponent(pseudo)}`
    });
    const data = await res.json();
    if (data.success) {
        Swal.fire('Succès', 'Invitation envoyée', 'success');
        document.getElementById('favDialog').close();
        e.target.reset();
    } else {
        Swal.fire('Erreur', data.error, 'error');
    }
}

// Helpers
function escapeHtml(t) {
    const d = document.createElement('div');
    d.textContent = t;
    return d.innerHTML;
}

function renderMedia(path) {
    if (path.match(/\.(jpg|jpeg|png|webp)$/i)) {
        return `<div class="pj-container"><img src="/uploads/messages/images/${path}" alt="Image"></div>`;
    }
    return `<div class="pj-container"><video controls><source src="/uploads/messages/videos/${path}" type="video/mp4"></video></div>`;
}

// Ces fonctions doivent être globales pour les onclick du HTML
window.selectContact = selectContact;
window.respondInvitation = async (senderId, action) => {
    await fetch('/api/invitation-response', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: `senderId=${senderId}&action=${action}`
    });
    loadInvitations();
    loadContacts();
};
window.deconnexion = async () => {
    await fetch('/api/logout', { method: 'POST' });
    window.location.href = '/front/template/login.html';
};