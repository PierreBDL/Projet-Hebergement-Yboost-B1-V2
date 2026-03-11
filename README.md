<div align="center">

# 💬 ChatApp

**Application de messagerie instantanée**

*Développée en Go · Déployée sur Render · Base de données Supabase*

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-Supabase-3ECF8E?style=flat&logo=supabase&logoColor=white)](https://supabase.com/)
[![Render](https://img.shields.io/badge/Déployé_sur-Render-46E3B7?style=flat&logo=render&logoColor=white)](https://render.com/)

[🌐 **Ouvrir l'application**](https://projet-hebergement-yboost-b1-v2-1.onrender.com/front/template/login.html)

</div>

---

## 🛠️ Stack technique

| Couche | Technologie |
|---|---|
| **Backend** | Go (net/http) |
| **ORM** | GORM |
| **Base de données** | PostgreSQL via Supabase |
| **Déploiement** | Render |
| **Frontend** | HTML / CSS / JavaScript vanilla |
| **API externe** | [icanhazdadjoke.com](https://icanhazdadjoke.com/) |

---

## ✨ Fonctionnalités

| | Fonctionnalité |
|---|---|
| 🔐 | Inscription et connexion sécurisées (chiffrement AES-CBC) |
| 🎟️ | Gestion de session via token en `localStorage` |
| 👥 | Ajout de contacts par invitation (accepter / refuser) |
| 💬 | Messagerie instantanée entre contacts |
| 📎 | Envoi de fichiers (images, vidéos) |
| ✏️ | Modification et suppression de ses propres messages (CRUD complet) |
| 😄 | Widget blague du jour via API externe |
| 📱 | Interface responsive (mobile, tablette, desktop) |

---

## 🔐 Variables d'environnement

Copier `.env.example` en `.env.local` et remplir les valeurs :

```bash
cp .env.example .env.local
```

| Variable | Description | Requis |
|---|---|---|
| `DATABASE_URL` | URL complète PostgreSQL Supabase (`postgresql://...`) | ✅ Si Supabase |
| `DB_HOST` | Hôte MySQL | ✅ Si MySQL |
| `DB_PORT` | Port MySQL (défaut : `3306`) | ✅ Si MySQL |
| `DB_NAME` | Nom de la base de données | ✅ Si MySQL |
| `DB_USER` | Utilisateur MySQL | ✅ Si MySQL |
| `DB_PASS` | Mot de passe MySQL | ✅ Si MySQL |
| `PORT` | Port du serveur (défaut : `8081`) | ➖ Optionnel |

> ⚠️ `.env.local` est dans le `.gitignore` — ne jamais committer de vraies credentials

---

## 📡 Routes API

<details>
<summary><strong>🔑 Authentification</strong></summary>
<br>

| Méthode | Route | Description |
|---|---|---|
| `GET` | `/login` | Page de connexion |
| `POST` | `/login` | Connexion (retourne un token) |
| `GET` | `/signup` | Page d'inscription |
| `POST` | `/signup` | Création de compte |
| `POST` | `/api/logout` | Déconnexion |
| `GET` | `/api/session` | Vérifie la session courante |

</details>

<details>
<summary><strong>👥 Contacts</strong></summary>
<br>

| Méthode | Route | Description |
|---|---|---|
| `GET` | `/api/contacts` | Liste des contacts acceptés |
| `GET` | `/api/invitations` | Invitations en attente |
| `POST` | `/api/send-invitation` | Envoyer une invitation |
| `POST` | `/api/invitation-response` | Accepter ou refuser une invitation |

</details>

<details>
<summary><strong>💬 Messages</strong></summary>
<br>

| Méthode | Route | Description |
|---|---|---|
| `GET` | `/api/messages?contact={id}` | Récupérer les messages avec un contact |
| `POST` | `/api/send-message` | Envoyer un message (texte + fichier) |
| `POST` | `/api/edit-message` | Modifier un message |
| `POST` | `/api/delete-message` | Supprimer un message |

</details>

<details>
<summary><strong>😄 API externe</strong></summary>
<br>

| Méthode | Route | Description |
|---|---|---|
| `GET` | `/api/joke` | Récupère une blague via icanhazdadjoke |

</details>

---

## 🗄️ Structure du projet

```
.
├── main.go                  # Serveur Go, routes et handlers
├── go.mod / go.sum          # Dépendances Go
├── .env.example             # Template des variables d'environnement
├── .env.local               # Variables locales (ignoré par git)
├── bdd/
│   └── bdd.sql              # Schéma de la base de données locale
├── front/
│   ├── assets/
│   │   ├── css/
│   │   │   ├── commun/
│   │   │   │   ├── header.css           # Styles communs pour l'en-tête
│   │   │   │   └── footer.css           # Styles communs pour le pied de page
│   │   │   ├── dashboard.css            # Styles spécifiques au dashboard
│   │   │   └── inscriptionLogin.css     # Styles connexion et inscription
│   │   ├── images/                      # Images et icônes
│   │   └── javascript/
│   │       └── dashboard.js             # Logique frontend du dashboard
│   └── template/
│       ├── login.html                   # Page de connexion
│       ├── inscription.html             # Page d'inscription
│       └── dashboard.html               # Interface principale après connexion
└── uploads/
    └── messages/                        # Fichiers uploadés par les utilisateurs
```

---

<div align="center">

**Pierre** — Projet réalisé dans le cadre du cours *Yboost*, B1 - informatique.

</div>