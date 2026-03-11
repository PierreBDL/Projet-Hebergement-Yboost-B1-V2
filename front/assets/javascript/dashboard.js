// =========================
// État global
// =========================
let currentUserId = null;
let currentContactId = null;

// =========================
// Initialisation
// =========================
async function initDashboard() {
    try {
        const response = await fetch('/api/session');
        const data = await response.json();
        
        if (!data.success) {
            window.location.href = '/front/template/login.html';
            return;
        }

        currentUserId = data.data.id;
        document.getElementById('username').textContent = data.data.user;
        document.getElementById('content').style.display = 'block';
        
        loadContacts();
        loadInvitations();
    } catch (error) {
        console.error('Erreur initialisation:', error);
        window.location.href = '/front/template/login.html';
    }
}

// =========================
// Charger les contacts
// =========================
async function loadContacts() {
    try {
        const response = await fetch('/api/contacts');
        const data = await response.json();
        
        const contactsList = document.getElementById('contactsList');
        
        if (!data.data || data.data.length === 0) {
            contactsList.innerHTML = '<p class="contactVide">Aucun contact pour le moment !</p>';
            return;
        }

        contactsList.innerHTML = data.data.map(contact => `
            <a href="#" onclick="selectContact(${contact.id}); return false;" 
               class="contactItem ${currentContactId === contact.id ? 'active' : ''}">
                <img src="../assets/images/avatar.jpg" alt="avatar">
                <span>${escapeHtml(contact.username)}</span>
            </a>
        `).join('');
    } catch (error) {
        console.error('Erreur chargement contacts:', error);
    }
}

// =========================
// Charger les invitations
// =========================
async function loadInvitations() {
    try {
        const response = await fetch('/api/invitations');
        const data = await response.json();
        
        const invitationsList = document.getElementById('invitationsList');
        
        if (!data.data || data.data.length === 0) {
            invitationsList.innerHTML = '<p class="aucuneInvitation">Aucune invitation</p>';
            return;
        }

        invitationsList.innerHTML = data.data.map(invitation => `
            <div class="invitation-item">
                <div class="invitation-info">
                    <img src="../assets/images/avatar.jpg" alt="avatar">
                    <span>${escapeHtml(invitation.senderUsername)}</span>
                </div>
                <div class="invitation-actions">
                    <button onclick="respondInvitation(${invitation.senderId}, 'accepter')" class="btn-accepter">✓ Accepter</button>
                    <button onclick="respondInvitation(${invitation.senderId}, 'refuser')" class="btn-refuser">✕ Refuser</button>
                </div>
            </div>
        `).join('');
    } catch (error) {
        console.error('Erreur chargement invitations:', error);
    }
}

// =========================
// Sélectionner un contact
// =========================
async function selectContact(contactId) {
    currentContactId = contactId;
    document.querySelectorAll('.contactItem').forEach(item => item.classList.remove('active'));
    event.target.closest('.contactItem')?.classList.add('active');
    
    document.getElementById('messageForm').style.display = 'block';
    loadMessages(contactId);
}

// =========================
// Charger les messages
// =========================
async function loadMessages(contactId) {
    try {
        const response = await fetch(`/api/messages?contact=${contactId}`);
        const data = await response.json();
        
        const messagesList = document.getElementById('messagesList');
        
        if (!data.data || data.data.length === 0) {
            messagesList.innerHTML = '<h3 class="messagesVide">Aucun message pour le moment !</h3>';
        } else {
            messagesList.innerHTML = '<h2>Discussion</h2>' + data.data.map(msg => `
                <div class="message ${msg.sender === currentUserId ? 'emetteur' : 'recepteur'}">
                    <div class="message-content">
                        ${msg.filepath ? `
                            <div class="pj-container">
                                ${msg.filepath.match(/\.(jpg|jpeg|png|webp)$/i) ? `
                                    <img src="/uploads/messages/images/${escapeHtml(msg.filepath)}" alt="Image">
                                ` : `
                                    <video controls>
                                        <source src="/uploads/messages/videos/${escapeHtml(msg.filepath)}" type="video/mp4">
                                    </video>
                                `}
                            </div>
                        ` : ''}
                        ${msg.content && msg.content !== 'null' ? `
                            <h3 class="messagesRemplis">${escapeHtml(msg.content)}</h3>
                        ` : ''}
                    </div>
                    <h5 class="date">${new Date(msg.timestamp).toLocaleString('fr-FR')}</h5>
                </div>
            `).join('');
        }
        
        // Auto-scroll
        setTimeout(() => {
            messagesList.scrollTop = messagesList.scrollHeight;
        }, 100);
    } catch (error) {
        console.error('Erreur chargement messages:', error);
    }
}

// =========================
// Envoyer un message
// =========================
// =========================
// Envoyer un message
// =========================
document.addEventListener('DOMContentLoaded', () => {
    document.getElementById('sendMessageForm')?.addEventListener('submit', async (e) => {
        e.preventDefault();
        
        if (!currentContactId) {
            Swal.fire('Erreur', 'Veuillez sélectionner un contact', 'error');
            return;
        }

        const formData = new FormData();
        formData.append('receiverId', currentContactId);
        formData.append('message', document.getElementById('messageRediger').value.trim());
        
        const imageFile = document.getElementById('uploadImage').files[0];
        const videoFile = document.getElementById('uploadVideo').files[0];
        
        if (imageFile) {
            formData.append('file', imageFile);
            formData.append('fileType', 'image');
            formData.append('filename', imageFile.name);
        } else if (videoFile) {
            formData.append('file', videoFile);
            formData.append('fileType', 'video');
            formData.append('filename', videoFile.name);
        }

        try {
            const response = await fetch('/api/send-message', {
                method: 'POST',
                body: formData
            });
            
            const data = await response.json();
            
            if (data.success) {
                document.getElementById('messageRediger').value = '';
                document.getElementById('uploadImage').value = '';
                document.getElementById('uploadVideo').value = '';
                
                Swal.fire({
                    icon: 'success',
                    title: 'Message envoyé',
                    timer: 1500,
                    showConfirmButton: false
                });
                
                loadMessages(currentContactId);
            } else {
                Swal.fire('Erreur', data.error || 'Erreur lors de l\'envoi', 'error');
            }
        } catch (error) {
            console.error('Erreur envoi message:', error);
            Swal.fire('Erreur', 'Erreur serveur', 'error');
        }
    });

    // =========================
    // Modal "Nouveau contact"
    // =========================
    const dialog = document.getElementById('favDialog');
    const showButton = document.getElementById('showDialog');
    const cancelBtn = document.getElementById('cancelBtn');
    const formInvitation = document.getElementById('formInvitation');
    const inputContact = document.getElementById('nomNouveauContact');

    showButton?.addEventListener('click', () => {
        dialog.showModal();
        inputContact.focus();
    });

    cancelBtn?.addEventListener('click', (e) => {
        e.preventDefault();
        dialog.close();
        inputContact.value = '';
    });

    dialog?.addEventListener('click', (e) => {
        if (e.target === dialog) {
            dialog.close();
        }
    });

    formInvitation?.addEventListener('submit', async (e) => {
        e.preventDefault();
        
        const pseudo = inputContact.value.trim();
        
        if (!pseudo) {
            Swal.fire('Champ vide', 'Veuillez entrer un pseudo', 'warning');
            return;
        }

        try {
            const response = await fetch('/api/send-invitation', {
                method: 'POST',
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                body: `pseudoDestinataire=${encodeURIComponent(pseudo)}`
            });
            
            const data = await response.json();
            
            if (data.success) {
                Swal.fire('Invitation envoyée', data.message, 'success');
                dialog.close();
                inputContact.value = '';
                loadContacts();
            } else {
                Swal.fire('Erreur', data.error, 'error');
            }
        } catch (error) {
            console.error('Erreur invitation:', error);
            Swal.fire('Erreur', 'Erreur serveur', 'error');
        }
    });

    // =========================
    // Menu pièces jointes
    // =========================
    const btnPieceJointe = document.getElementById('piece-jointe');
    const menuPieceJointe = document.querySelector('.piece-jointe-menu');

    btnPieceJointe?.addEventListener('click', () => {
        menuPieceJointe.classList.toggle('piece-jointe-menu-ouvert');
    });

    // =========================
    // Auto-resize textarea
    // =========================
    const textarea = document.getElementById('messageRediger');
    if (textarea) {
        textarea.addEventListener('input', () => {
            textarea.style.height = 'auto';
            textarea.style.height = textarea.scrollHeight + 'px';
        });
    }

    // =========================
    // Détecter le changement sur les inputs file
    // =========================
    document.querySelectorAll('input[type="file"]').forEach(input => {
        input.addEventListener('change', function() {
            if (this.files && this.files[0]) {
                const btn = document.getElementById('piece-jointe');
                btn.style.backgroundColor = '#4caf50';

                Swal.fire({
                    icon: 'info',
                    title: 'Fichier sélectionné',
                    text: this.files[0].name,
                    timer: 1500,
                    showConfirmButton: false
                });
            }
        });
    });
});

// =========================
// Répondre à une invitation
// =========================
async function respondInvitation(senderId, action) {
    try {
        const response = await fetch('/api/invitation-response', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: `senderId=${senderId}&action=${action}`
        });
        
        const data = await response.json();
        
        if (data.success) {
            loadInvitations();
            loadContacts();
        }
    } catch (error) {
        console.error('Erreur réponse invitation:', error);
    }
}

// =========================
// Déconnexion
// =========================
async function deconnexion() {
    try {
        await fetch('/api/logout', { method: 'POST' });
        window.location.href = '/front/template/login.html';
    } catch (error) {
        console.error('Erreur déconnexion:', error);
        window.location.href = '/front/template/login.html';
    }
}

// =========================
// Utilitaires
// =========================
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// =========================
// Modal "Nouveau contact"
// =========================
const dialog = document.getElementById("favDialog");
const showButton = document.getElementById("showDialog");
const cancelBtn = document.getElementById("cancelBtn");
const inputContact = document.getElementById("nomNouveauContact");

showButton?.addEventListener("click", () => {
    dialog.showModal();
    inputContact.focus();
});

cancelBtn?.addEventListener("click", (e) => {
    e.preventDefault();
    dialog.close();
    inputContact.value = "";
});

// Fermer en cliquant hors modale
dialog?.addEventListener("click", (e) => {
    if (e.target === dialog) {
        dialog.close();
    }
});

// =========================
// Auto-scroll messages
// =========================
const messagesBox = document.querySelector(".messages");
if (messagesBox) {
    messagesBox.scrollTop = messagesBox.scrollHeight;
}

// =========================
// Auto-resize textarea
// =========================
const textarea = document.getElementById("messageRediger");

textarea?.addEventListener("input", () => {
    textarea.style.height = "auto";
    textarea.style.height = textarea.scrollHeight + "px";
});


// =========================
// Envoie de l'invitation
// =========================

document.getElementById("formInvitation").addEventListener("submit", function (e) {
    e.preventDefault();

    const pseudo = document.getElementById("nomNouveauContact").value.trim();

    if (pseudo === "") {
        Swal.fire({
            icon: "warning",
            title: "Champ vide",
            text: "Veuillez entrer un pseudo"
        });
        return;
    }

    fetch("./ajax/invitation.inc.php", {
        method: "POST",
        headers: {
            "Content-Type": "application/x-www-form-urlencoded"
        },
        body: new URLSearchParams({
            pseudoDestinataire: pseudo
        })
    })
    .then(r => r.json())
    .then(data => {
        if (data.success) {
            Swal.fire("Invitation envoyée", data.message, "success");
            dialog.close();
        } else {
            Swal.fire("Erreur", data.error, "error");
        }
    })
    .catch(err => console.error(err));
});


// =========================
// Menu pièces jointes
// =========================

const btnPieceJointe = document.getElementById("piece-jointe");
const menuPieceJointe = document.querySelector(".piece-jointe-menu");

btnPieceJointe.addEventListener("click", () => {
    menuPieceJointe.classList.toggle("piece-jointe-menu-ouvert");
});


// =========================
// Détecter le changement sur les inputs file
// =========================

document.querySelectorAll('input[type="file"]').forEach(input => {
    input.addEventListener('change', function() {
        if (this.files && this.files[0]) {
            const btn = document.getElementById('piece-jointe');
            btn.style.backgroundColor = '#4caf50';

            Swal.fire({
                icon: 'info',
                title: 'Fichier sélectionné',
                text: this.files[0].name,
                timer: 1500,
                showConfirmButton: false
            });
        }
    });
});