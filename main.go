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
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ===== STRUCTURES ET MODÈLES =====

type Config struct {
	DBType      string
	DatabaseURL string
	DBHost      string
	DBPort      string
	DBName      string
	DBUser      string
	DBPass      string
	Port        string
}

type Session struct {
	UserID    int
	Username  string
	Access    string
	CreatedAt time.Time
}

type Compte struct {
	ID       int    `gorm:"primaryKey;column:idcompte" json:"id"`
	Username string `gorm:"column:identifiant" json:"username"`
	Password []byte `gorm:"column:motdepasse" json:"password"`
	IV       []byte `gorm:"column:iv" json:"iv"`
	Key      []byte `gorm:"column:cle" json:"cle"`
}

func (Compte) TableName() string { return "compte" }

type Contact struct {
	ID           int       `gorm:"primaryKey;column:idcontact" json:"id"`
	SenderID     int       `gorm:"column:idpossesseur" json:"senderId"`
	ReceiverID   int       `gorm:"column:iddestinataire" json:"receiverId"`
	Status       string    `gorm:"column:statut" json:"status"`
	DateCreation time.Time `gorm:"column:date_creation;autoCreateTime:milli" json:"dateCreation"`
}

func (Contact) TableName() string { return "contact" }

type Message struct {
	ID           int       `gorm:"primaryKey;column:idmessage" json:"id"`
	SenderID     int       `gorm:"column:idemetteur" json:"senderId"`
	ReceiverID   int       `gorm:"column:idreceveur" json:"receiverId"`
	Content      string    `gorm:"column:contenu" json:"content"`
	FilePath     *string   `gorm:"column:chemin" json:"filePath"`
	DateCreation time.Time `gorm:"column:date_creation;autoCreateTime:milli" json:"dateCreation"`
	SenderName   string    `gorm:"-" json:"senderName"`
}

func (Message) TableName() string { return "messages" }

// Types pour les réponses API
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
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ===== VARIABLES GLOBALES =====

var (
	db        *gorm.DB
	config    Config
	sessions  = make(map[string]*Session)
	masterKey = []byte("ma_cle_secrete_32_caracteres_!!!") // 32 octets pour AES-256
)

// ===== INITIALISATION ET CONNEXION =====

func main() {
	// Chargement des envs
	godotenv.Load(".env.local")
	godotenv.Load(".env.example")

	config = parseConfig()

	// Connexion DB
	var err error
	db, err = connectDB()
	if err != nil {
		log.Printf("⚠️ Erreur DB: %v", err)
	} else {
		log.Println("✅ Base de données connectée")
	}

	// Routes Statiques
	fs := http.FileServer(http.Dir("front"))
	http.Handle("/front/", http.StripPrefix("/front/", fs))

	// Routes Pages
	http.HandleFunc("/", redirectHome)
	http.HandleFunc("/dashboard", requireAuth(handleDashboard))

	// Routes API Authentification
	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/signup", handleSignup)
	http.HandleFunc("/api/logout", handleLogout)
	http.HandleFunc("/api/session", handleGetSession)

	// Routes API Fonctionnalités
	http.HandleFunc("/api/contacts", requireAuth(handleGetContacts))
	http.HandleFunc("/api/messages", requireAuth(handleGetMessages))
	http.HandleFunc("/api/send-message", requireAuth(handleSendMessage))
	http.HandleFunc("/api/invitations", requireAuth(handleGetInvitations))
	http.HandleFunc("/api/send-invitation", requireAuth(handleSendInvitation))
	http.HandleFunc("/api/invitation-response", requireAuth(handleInvitationResponse))

	port := config.Port
	if port == "" { port = "8080" }
	log.Printf("🚀 Serveur sur port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func parseConfig() Config {
	cfg := Config{
		Port: os.Getenv("PORT"),
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		cfg.DBType = "pgsql"
		cfg.DatabaseURL = dbURL
	} else {
		cfg.DBType = "mysql"
		cfg.DBHost = getEnv("DB_HOST", "localhost")
		cfg.DBPort = getEnv("DB_PORT", "3306")
		cfg.DBName = getEnv("DB_NAME", "bdd_messagerie")
		cfg.DBUser = getEnv("DB_USER", "root")
		cfg.DBPass = getEnv("DB_PASS", "")
	}
	return cfg
}

func connectDB() (*gorm.DB, error) {
	var dialector gorm.Dialector

	if config.DBType == "pgsql" {
		// Version pour Render/Supabase
		dialector = postgres.New(postgres.Config{
			DSN:                  config.DatabaseURL,
			PreferSimpleProtocol: true,
		})
	} else {
		// Version MySQL locale
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true",
			config.DBUser, config.DBPass, config.DBHost, config.DBPort, config.DBName)
		dialector = mysql.Open(dsn)
	}

	return gorm.Open(dialector, &gorm.Config{})
}

// ===== HANDLERS AUTH =====

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
        serveHTML(w, r, "front/template/login.html")
        return
    }

	username := r.FormValue("identifiant")
	password := r.FormValue("password")

	var compte Compte
	if err := db.Where("identifiant = ?", username).First(&compte).Error; err != nil {
		respondJSON(w, http.StatusUnauthorized, Response{Success: false, Error: "Identifiants incorrects"})
		return
	}

	decrypted, _ := decryptPass(compte.Password, compte.Key, compte.IV)
	if decrypted != password {
		respondJSON(w, http.StatusUnauthorized, Response{Success: false, Error: "Identifiants incorrects"})
		return
	}

	sessionID := generateSessionID()
	sessions[sessionID] = &Session{
		UserID:    compte.ID,
		Username:  username,
		Access:    "pass",
		CreatedAt: time.Now(),
	}

	http.SetCookie(w, &http.Cookie{
		Name: "session_id", Value: sessionID, Path: "/", HttpOnly: true,
	})

	respondJSON(w, http.StatusOK, Response{Success: true})
}

func handleSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		serveHTML(w, r, "front/template/inscription.html")
		return
	}

	username := r.FormValue("identifiant")
	password := r.FormValue("password")

	var count int64
	db.Model(&Compte{}).Where("identifiant = ?", username).Count(&count)
	if count > 0 {
		respondJSON(w, http.StatusBadRequest, Response{Success: false, Error: "Existe déjà"})
		return
	}

	key := make([]byte, 32)
	rand.Read(key)
	iv := make([]byte, 16)
	rand.Read(iv)
	encrypted := encryptPass(password, key, iv)

	newCompte := Compte{Username: username, Password: encrypted, IV: iv, Key: key}
	if err := db.Create(&newCompte).Error; err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{Success: false, Error: "Erreur création"})
		return
	}

	respondJSON(w, http.StatusOK, Response{Success: true})
}

// ===== HANDLERS MESSAGERIE =====

func handleGetContacts(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	var comptes []Compte
	db.Raw("SELECT * FROM compte WHERE idcompte IN (SELECT iddestinataire FROM contact WHERE idpossesseur = ? AND statut = 'accepte')", session.UserID).Scan(&comptes)

	var contacts []ContactResponse
	for _, c := range comptes {
		contacts = append(contacts, ContactResponse{ID: c.ID, Username: c.Username})
	}
	respondJSON(w, http.StatusOK, Response{Success: true, Data: contacts})
}

func handleGetMessages(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	contactID := r.URL.Query().Get("contact")
	cID, _ := strconv.Atoi(contactID)

	var messages []Message
	db.Where("(idemetteur = ? AND idreceveur = ?) OR (idemetteur = ? AND idreceveur = ?)", 
		session.UserID, cID, cID, session.UserID).Order("date_creation ASC").Find(&messages)

	for i, msg := range messages {
		var sender Compte
		db.Select("identifiant").Where("idcompte = ?", msg.SenderID).First(&sender)
		messages[i].SenderName = sender.Username
	}
	respondJSON(w, http.StatusOK, Response{Success: true, Data: messages})
}

func handleSendMessage(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	r.ParseMultipartForm(10 << 20)
	
	receiverID, _ := strconv.Atoi(r.FormValue("receiverId"))
	content := r.FormValue("message")
	
	var filePath *string
	file, header, err := r.FormFile("file")
	if err == nil {
		defer file.Close()
		fileName := fmt.Sprintf("%d_%s", time.Now().Unix(), header.Filename)
		dir := filepath.Join("uploads", "messages")
		os.MkdirAll(dir, 0755)
		path := filepath.Join(dir, fileName)
		out, _ := os.Create(path)
		defer out.Close()
		io.Copy(out, file)
		filePath = &fileName
	}

	msg := Message{SenderID: session.UserID, ReceiverID: receiverID, Content: content, FilePath: filePath}
	db.Create(&msg)
	respondJSON(w, http.StatusOK, Response{Success: true})
}

// ===== INVITATIONS =====

func handleGetInvitations(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	var contacts []Contact
	db.Where("iddestinataire = ? AND statut = 'en_attente'", session.UserID).Find(&contacts)

	var invits []Invitation
	for _, c := range contacts {
		var u Compte
		db.First(&u, c.SenderID)
		invits = append(invits, Invitation{ID: c.ID, SenderID: c.SenderID, SenderUsername: u.Username})
	}
	respondJSON(w, http.StatusOK, Response{Success: true, Data: invits})
}

func handleSendInvitation(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	pseudo := r.FormValue("pseudoDestinataire")
	
	var dest Compte
	if err := db.Where("identifiant = ?", pseudo).First(&dest).Error; err != nil {
		respondJSON(w, http.StatusNotFound, Response{Success: false, Error: "Inconnu"})
		return
	}

	db.Create(&Contact{SenderID: session.UserID, ReceiverID: dest.ID, Status: "en_attente"})
	respondJSON(w, http.StatusOK, Response{Success: true})
}

func handleInvitationResponse(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	senderID, _ := strconv.Atoi(r.FormValue("senderId"))
	action := r.FormValue("action")

	if action == "accepter" {
		db.Model(&Contact{}).Where("idpossesseur = ? AND iddestinataire = ?", senderID, session.UserID).Update("statut", "accepte")
		// Créer le lien retour
		db.FirstOrCreate(&Contact{}, Contact{SenderID: session.UserID, ReceiverID: senderID, Status: "accepte"})
	} else {
		db.Where("idpossesseur = ? AND iddestinataire = ?", senderID, session.UserID).Delete(&Contact{})
	}
	respondJSON(w, http.StatusOK, Response{Success: true})
}

// ===== UTILS =====

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok { return value }
	return fallback
}

func getSession(r *http.Request) *Session {
	c, err := r.Cookie("session_id")
	if err != nil { return nil }
	return sessions[c.Value]
}

func generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func encryptPass(pass string, key, iv []byte) []byte {
	block, _ := aes.NewCipher(key)
	stream := cipher.NewCBCEncrypter(block, iv)
	plaintext := []byte(pass)
	pad := aes.BlockSize - (len(plaintext) % aes.BlockSize)
	plaintext = append(plaintext, bytes.Repeat([]byte{byte(pad)}, pad)...)
	res := make([]byte, len(plaintext))
	stream.CryptBlocks(res, plaintext)
	return res
}

func decryptPass(cipherText, key, iv []byte) (string, error) {
	block, _ := aes.NewCipher(key)
	stream := cipher.NewCBCDecrypter(block, iv)
	plain := make([]byte, len(cipherText))
	stream.CryptBlocks(plain, cipherText)
	pad := int(plain[len(plain)-1])
	return string(plain[:len(plain)-pad]), nil
}

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if getSession(r) == nil {
			http.Redirect(w, r, "/front/template/login.html", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func serveHTML(w http.ResponseWriter, path string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeFile(w, r, path)
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "session_id", Value: "", Path: "/", MaxAge: -1})
	respondJSON(w, http.StatusOK, Response{Success: true})
}

func handleGetSession(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	if s == nil { respondJSON(w, http.StatusOK, Response{Success: false}); return }
	respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{"id": s.UserID, "user": s.Username}})
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	serveHTML(w, r, "front/template/dashboard.html")
}

func redirectHome(w http.ResponseWriter, r *http.Request) {
	if getSession(r) != nil {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}