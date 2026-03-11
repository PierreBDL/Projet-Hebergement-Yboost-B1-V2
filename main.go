package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Config de l'application
type Config struct {
	DBType      string
	DBHost      string
	DBPort      string
	DBName      string
	DBUser      string
	DBPass      string
	DatabaseURL string
	Port        string
}

// Session utilisateur
type Session struct {
	UserID    int
	Username  string
	Access    string
	CreatedAt time.Time
}

// ===== MODÈLES GORM =====

// Compte représente un utilisateur
type Compte struct {
	ID       int       `gorm:"primaryKey;column:idcompte" json:"id"`
	Username string    `gorm:"column:identifiant" json:"username"`
	Password []byte    `gorm:"column:motdepasse" json:"password"`
	IV       []byte    `gorm:"column:iv" json:"iv"`
	Key      []byte    `gorm:"column:cle" json:"key"`
	Messages []Message `gorm:"foreignKey:SenderID" json:"-"`
}

func (Compte) TableName() string {
	return "compte"
}

// Contact représente une relation entre deux utilisateurs
type Contact struct {
	ID            int       `gorm:"primaryKey;column:idcontact" json:"id"`
	SenderID      int       `gorm:"column:idpossesseur" json:"senderId"`
	ReceiverID    int       `gorm:"column:iddestinataire" json:"receiverId"`
	Status        string    `gorm:"column:statut" json:"status"`
	DateCreation  time.Time `gorm:"column:date_creation;autoCreateTime:milli" json:"dateCreation"`
}

func (Contact) TableName() string {
	return "contact"
}

// Message représente un message entre deux utilisateurs
type Message struct {
	ID            int       `gorm:"primaryKey;column:idmessage" json:"id"`
	SenderID      int       `gorm:"column:idemetteur" json:"senderId"`
	ReceiverID    int       `gorm:"column:idreceveur" json:"receiverId"`
	Content       string    `gorm:"column:contenu" json:"content"`
	FilePath      *string   `gorm:"column:chemin" json:"filePath"`
	DateCreation  time.Time `gorm:"column:date_creation;autoCreateTime:milli" json:"dateCreation"`
	SenderName    string    `gorm:"-" json:"senderName"`
}

func (Message) TableName() string {
	return "messages"
}

// Types API response
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

type ContactResponse struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

type Invitation struct {
	ID             int    `json:"id"`
	SenderID       int    `json:"senderId"`
	SenderUsername string `json:"senderUsername"`
}

type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

var (
	config Config
	db     *gorm.DB
	sessions = make(map[string]*Session)
)

func main() {
	// Capturer les panics pour mieux debugger
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ PANIC AU DÉMARRAGE: %v", r)
		}
	}()

	// Charger les variables d'environnement
	godotenv.Load(".env.local")
	godotenv.Load(".env.example")

	// Vérifier les env vars critiques
	dbURL := os.Getenv("DATABASE_URL")
	port := os.Getenv("PORT")
	
	log.Printf("====== VARIABLES D'ENVIRONNEMENT ======")
	if dbURL != "" {
		log.Printf("✓ DATABASE_URL: postgresql://****:****@****:****")
	} else {
		log.Printf("✗ DATABASE_URL: NON DÉFINI")
	}
	log.Printf("PORT: %s (par défaut: 8080)", port)
	log.Printf("========================================")

	config = parseConfig()

	// Logging détaillé pour debugging
	log.Printf("====== Configuration Chargée ======")
	log.Printf("Mode DB: %s", config.DBType)
	if config.DBType == "pgsql" {
		log.Printf("DATABASE_URL configuré pour PostgreSQL")
	} else {
		log.Printf("Mode MySQL: DB_HOST=%s, DB_PORT=%s, DB_NAME=%s, DB_USER=%s", 
			config.DBHost, config.DBPort, config.DBName, config.DBUser)
	}
	log.Printf("PORT=%s", config.Port)
	log.Printf("=====================================")

	// Connexion à la base de données
	var err error
	db, err = connectDB()
	if err != nil {
		log.Printf("⚠️  ATTENTION: BD indisponible au démarrage: %v", err)
		log.Printf("💡 L'app continuera sans BD. Les requêtes retourneront une erreur.")
	} else {
		log.Printf("✅ Connecté à la base de données")
		defer func() {
			sqlDB, _ := db.DB()
			if sqlDB != nil {
				sqlDB.Close()
			}
		}()
	}

	// Routes
	http.HandleFunc("/", redirectHome)
	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/signup", handleSignup)
	http.HandleFunc("/dashboard", requireAuth(handleDashboard))
	http.HandleFunc("/api/contacts", requireAuth(handleGetContacts))
	http.HandleFunc("/api/messages", requireAuth(handleGetMessages))
	http.HandleFunc("/api/send-message", requireAuth(handleSendMessage))
	http.HandleFunc("/api/invitations", requireAuth(handleGetInvitations))
	http.HandleFunc("/api/send-invitation", requireAuth(handleSendInvitation))
	http.HandleFunc("/api/invitation-response", requireAuth(handleInvitationResponse))
	http.HandleFunc("/api/logout", handleLogout)
	http.HandleFunc("/api/session", handleGetSession)

	// Servir les fichiers statiques
	fs := http.FileServer(http.Dir("front"))
	http.Handle("/front/", http.StripPrefix("/front/", fs))

	port = config.Port
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 Serveur démarré sur http://0.0.0.0:%s", port)
	log.Printf("💡 Accès: http://localhost:%s (local) ou https://votre-app.onrender.com (production)", port)
	
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("❌ ERREUR CRITIQUE au démarrage du serveur: %v", err)
	}
}

// parseConfig lit la configuration
func parseConfig() Config {
	cfg := Config{
		Port: os.Getenv("PORT"),
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL != "" {
		cfg.DBType = "pgsql"
		cfg.DatabaseURL = databaseURL
		cfg.DBName = "postgres" // Supabase
		log.Printf("✓ DATABASE_URL détecté → Mode PostgreSQL")
	} else {
		log.Printf("✗ DATABASE_URL non trouvé → Mode MySQL")
		cfg.DBType = "mysql"
		cfg.DBHost = getEnv("DB_HOST", "localhost")
		cfg.DBPort = getEnv("DB_PORT", "3306")
		cfg.DBName = getEnv("DB_NAME", "bdd_messagerie")
		cfg.DBUser = getEnv("DB_USER", "root")
		cfg.DBPass = getEnv("DB_PASS", "")
	}

	return cfg
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// connectDB crée la connexion à la base de données avec GORM
func connectDB() (*gorm.DB, error) {
	var dialector gorm.Dialector
	var err error

	if config.DBType == "pgsql" && config.DatabaseURL != "" {
		// PostgreSQL via URL (Supabase/Render)
		dialector = postgres.Open(config.DatabaseURL)
	} else {
		// MySQL local
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true",
			config.DBUser, config.DBPass, config.DBHost, config.DBPort, config.DBName)
		dialector = mysql.Open(dsn)
	}

	db, err = gorm.Open(postgres.New(postgres.Config{
		DSN: config.DatabaseURL,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	return db, err
}

// redirectHome redirige vers login ou dashboard
func redirectHome(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	if session != nil && session.Access == "pass" {
		http.Redirect(w, r, "/front/template/dashboard.html", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/front/template/login.html", http.StatusSeeOther)
	}
}

// handleLogin traite la connexion avec GORM
func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		serveHTML(w, "/front/template/login.html")
		return
	}

	r.ParseForm()
	username := strings.TrimSpace(r.FormValue("identifiant"))
	password := r.FormValue("password")

	if username == "" || password == "" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Identifiants requis",
		})
		return
	}

	if db == nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Base de données non disponible",
		})
		return
	}

	// Récupérer l'utilisateur avec GORM
	var compte Compte
	result := db.Where("identifiant = ?", username).First(&compte)
	
	if result.Error != nil {
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Error:   "Identifiants incorrects",
		})
		return
	}

	// Déchiffrer le mot de passe
	decrypted, err := decryptPass(compte.Password, compte.Key, compte.IV)
	if err != nil || decrypted != password {
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Error:   "Identifiants incorrects",
		})
		return
	}

	// Créer la session
	sessionID := generateSessionID()
	sessions[sessionID] = &Session{
		UserID:    compte.ID,
		Username:  username,
		Access:    "pass",
		CreatedAt: time.Now(),
	}

	// Définir le cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now().Add(24 * time.Hour),
	})

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "Connecté",
	})
}

// handleSignup traite l'inscription avec GORM
func handleSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		serveHTML(w, "/front/template/inscription.html")
		return
	}

	r.ParseForm()
	username := strings.TrimSpace(r.FormValue("identifiant"))
	password := r.FormValue("password")

	if username == "" || password == "" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Tous les champs sont requis",
		})
		return
	}

	// Vérifier si l'utilisateur existe avec GORM
	var count int64
	db.Model(&Compte{}).Where("identifiant = ?", username).Count(&count)
	if count > 0 {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Cet identifiant existe déjà",
		})
		return
	}

	// Chiffrer le mot de passe
	key := make([]byte, 32)
	io.ReadFull(rand.Reader, key)

	iv := make([]byte, 16)
	io.ReadFull(rand.Reader, iv)

	encrypted := encryptPass(password, key, iv)

	// Créer le nouvel utilisateur avec GORM
	newCompte := Compte{
		Username: username,
		Password: encrypted,
		IV:       iv,
		Key:      key,
	}

	if err := db.Create(&newCompte).Error; err != nil {
		log.Printf("Erreur inscription: %v", err)
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Erreur lors de l'inscription",
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "Inscription réussie",
	})
}

// handleDashboard sert la page du tableau de bord
func handleDashboard(w http.ResponseWriter, r *http.Request) {
	serveHTML(w, "/front/template/dashboard.html")
}

// handleGetContacts retourne les contacts avec GORM
func handleGetContacts(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	if session == nil {
		respondJSON(w, http.StatusUnauthorized, Response{Success: false})
		return
	}

	if db == nil {
		respondJSON(w, http.StatusServiceUnavailable, Response{
			Success: false,
			Error:   "Base de données non disponible. Veuillez relancer l'application.",
		})
		return
	}

	var comptes []Compte
	result := db.Where(`
		idcompte IN (
			SELECT iddestinataire FROM contact 
			WHERE idpossesseur = ? AND statut = 'accepte'
		)
	`, session.UserID).Find(&comptes)

	if result.Error != nil {
		respondJSON(w, http.StatusInternalServerError, Response{Success: false})
		return
	}

	// Transformer en ContactResponse
	var contacts []ContactResponse
	for _, c := range comptes {
		contacts = append(contacts, ContactResponse{
			ID:       c.ID,
			Username: c.Username,
		})
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    contacts,
	})
}

// handleGetMessages retourne les messages avec GORM
func handleGetMessages(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	if session == nil {
		respondJSON(w, http.StatusUnauthorized, Response{Success: false})
		return
	}

	if db == nil {
		respondJSON(w, http.StatusServiceUnavailable, Response{
			Success: false,
			Error:   "Base de données non disponible. Veuillez relancer l'application.",
		})
		return
	}

	contactID := r.URL.Query().Get("contact")
	if contactID == "" {
		respondJSON(w, http.StatusBadRequest, Response{Success: false})
		return
	}

	cID, _ := strconv.Atoi(contactID)

	var messages []Message
	result := db.Where(`
		(idemetteur = ? AND idreceveur = ?) OR (idemetteur = ? AND idreceveur = ?)
	`, session.UserID, cID, cID, session.UserID).
		Order("date_creation ASC").
		Find(&messages)

	if result.Error != nil {
		respondJSON(w, http.StatusInternalServerError, Response{Success: false})
		return
	}

	// Charger les noms des émetteurs
	for i, msg := range messages {
		var sender Compte
		db.Select("identifiant").Where("idcompte = ?", msg.SenderID).First(&sender)
		messages[i].SenderName = sender.Username
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    messages,
	})
}

// handleSendMessage envoie un message avec GORM
func handleSendMessage(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	if session == nil {
		respondJSON(w, http.StatusUnauthorized, Response{Success: false})
		return
	}

	if db == nil {
		respondJSON(w, http.StatusServiceUnavailable, Response{
			Success: false,
			Error:   "Base de données non disponible. Veuillez relancer l'application.",
		})
		return
	}

	r.ParseMultipartForm(10 << 20) // 10MB max

	receiverID := r.FormValue("receiverId")
	content := strings.TrimSpace(r.FormValue("message"))

	rID, _ := strconv.Atoi(receiverID)

	var filePath *string
	file, _, err := r.FormFile("file")
	if err == nil {
		defer file.Close()
		
		fileName := time.Now().Format("20060102150405") + "_" + strings.TrimSpace(filepath.Base(r.FormValue("filename")))
		fileType := r.FormValue("fileType") // "image" ou "video"
		
		dir := filepath.Join("uploads", "messages", fileType+"s")
		os.MkdirAll(dir, 0755)

		fPath := filepath.Join(dir, fileName)
		outFile, _ := os.Create(fPath)
		defer outFile.Close()
		io.Copy(outFile, file)

		filePath = &fileName
	}

	if content == "" && filePath == nil {
		respondJSON(w, http.StatusBadRequest, Response{Success: false, Error: "Message vide"})
		return
	}

	if content == "" {
		content = "null"
	}

	// Créer le message avec GORM
	newMessage := Message{
		SenderID:   session.UserID,
		ReceiverID: rID,
		Content:    content,
		FilePath:   filePath,
	}

	if err := db.Create(&newMessage).Error; err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{Success: false})
		return
	}

	respondJSON(w, http.StatusOK, Response{Success: true, Message: "Message envoyé"})
}

// handleGetInvitations retourne les invitations avec GORM
func handleGetInvitations(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	if session == nil {
		respondJSON(w, http.StatusUnauthorized, Response{Success: false})
		return
	}

	if db == nil {
		respondJSON(w, http.StatusServiceUnavailable, Response{
			Success: false,
			Error:   "Base de données non disponible. Veuillez relancer l'application.",
		})
		return
	}

	var contacts []Contact
	result := db.Where("iddestinataire = ? AND statut = 'en_attente'", session.UserID).
		Order("date_creation DESC").
		Find(&contacts)

	if result.Error != nil {
		respondJSON(w, http.StatusInternalServerError, Response{Success: false})
		return
	}

	// Charger les noms des émetteurs
	var invitations []Invitation
	for _, contact := range contacts {
		var sender Compte
		db.Select("identifiant").Where("idcompte = ?", contact.SenderID).First(&sender)
		invitations = append(invitations, Invitation{
			ID:             contact.ID,
			SenderID:       contact.SenderID,
			SenderUsername: sender.Username,
		})
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    invitations,
	})
}

// handleSendInvitation envoie une invitation avec GORM
func handleSendInvitation(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	if session == nil {
		respondJSON(w, http.StatusUnauthorized, Response{Success: false})
		return
	}

	if db == nil {
		respondJSON(w, http.StatusServiceUnavailable, Response{
			Success: false,
			Error:   "Base de données non disponible. Veuillez relancer l'application.",
		})
		return
	}

	r.ParseForm()
	pseudo := strings.TrimSpace(r.FormValue("pseudoDestinataire"))

	if pseudo == "" {
		respondJSON(w, http.StatusBadRequest, Response{Success: false, Error: "Pseudo vide"})
		return
	}

	// Trouver le destinataire avec GORM
	var destCompte Compte
	result := db.Where("identifiant = ?", pseudo).First(&destCompte)
	if result.Error != nil {
		respondJSON(w, http.StatusNotFound, Response{Success: false, Error: "Utilisateur introuvable"})
		return
	}

	// Créer l'invitation avec GORM
	newContact := Contact{
		SenderID:   session.UserID,
		ReceiverID: destCompte.ID,
		Status:     "en_attente",
	}

	if err := db.Create(&newContact).Error; err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{Success: false})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "Invitation envoyée à " + pseudo,
	})
}

// handleInvitationResponse traite la réponse à une invitation avec GORM
func handleInvitationResponse(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	if session == nil {
		respondJSON(w, http.StatusUnauthorized, Response{Success: false})
		return
	}

	if db == nil {
		respondJSON(w, http.StatusServiceUnavailable, Response{
			Success: false,
			Error:   "Base de données non disponible. Veuillez relancer l'application.",
		})
		return
	}

	r.ParseForm()
	senderID := r.FormValue("senderId")
	action := r.FormValue("action")

	sID, _ := strconv.Atoi(senderID)

	if action == "accepter" {
		// Accepter le contact initial
		db.Model(&Contact{}).
			Where("idpossesseur = ? AND iddestinataire = ?", sID, session.UserID).
			Update("statut", "accepte")

		// Vérifier si le contact inverse existe
		var count int64
		db.Model(&Contact{}).
			Where("idpossesseur = ? AND iddestinataire = ?", session.UserID, sID).
			Count(&count)

		// Créer le contact inverse s'il n'existe pas
		if count == 0 {
			reverseContact := Contact{
				SenderID:   session.UserID,
				ReceiverID: sID,
				Status:     "accepte",
			}
			db.Create(&reverseContact)
		}
	} else if action == "refuser" {
		// Supprimer le contact
		db.Where("idpossesseur = ? AND iddestinataire = ?", sID, session.UserID).
			Delete(&Contact{})
	}

	respondJSON(w, http.StatusOK, Response{Success: true})
}

// handleLogout déconnecte l'utilisateur
func handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	respondJSON(w, http.StatusOK, Response{Success: true})
}

// handleGetSession retourne l'info de session
func handleGetSession(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	if session == nil {
		respondJSON(w, http.StatusOK, Response{Success: false})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"id":   session.UserID,
			"user": session.Username,
		},
	})
}

// Fonctions utilitaires

func getSession(r *http.Request) *Session {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return nil
	}
	return sessions[cookie.Value]
}

func generateSessionID() string {
	b := make([]byte, 32)
	io.ReadFull(rand.Reader, b)
	return base64.URLEncoding.EncodeToString(b)
}

func encryptPass(password string, key, iv []byte) []byte {
	block, _ := aes.NewCipher(key)
	stream := cipher.NewCBCEncrypter(block, iv)
	plaintext := []byte(password)
	
	// Ajouter du padding PKCS7
	padlen := aes.BlockSize - (len(plaintext) % aes.BlockSize)
	plaintext = append(plaintext, bytes.Repeat([]byte{byte(padlen)}, padlen)...)
	
	ciphertext := make([]byte, len(plaintext))
	stream.CryptBlocks(ciphertext, plaintext)
	return ciphertext
}

func decryptPass(ciphertext, key, iv []byte) (string, error) {
	block, _ := aes.NewCipher(key)
	stream := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	stream.CryptBlocks(plaintext, ciphertext)
	
	// Retirer le padding PKCS7
	padlen := int(plaintext[len(plaintext)-1])
	plaintext = plaintext[:len(plaintext)-padlen]
	
	return string(plaintext), nil
}

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := getSession(r)
		if session == nil {
			http.Redirect(w, r, "/front/template/login.html", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func serveHTML(w http.ResponseWriter, filePath string) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "404 - Fichier non trouvé")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
