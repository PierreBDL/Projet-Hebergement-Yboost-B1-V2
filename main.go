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
	ID        string    `gorm:"primaryKey;column:idsession"`
	UserID    int       `gorm:"column:idcompte"`
	Username  string    `gorm:"column:identifiant"`
	CreatedAt time.Time `gorm:"column:date_creation"`
}

func (Session) TableName() string { return "sessions" }

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
	SenderID     int       `gorm:"column:idemetteur" json:"sender"`
	ReceiverID   int       `gorm:"column:idreceveur" json:"receiver"`
	Content      string    `gorm:"column:contenu" json:"content"`
	FilePath     *string   `gorm:"column:chemin" json:"filepath"`
	DateCreation time.Time `gorm:"column:date_creation;autoCreateTime:milli" json:"timestamp"`
	SenderName   string    `gorm:"-" json:"senderName"`
}

func (Message) TableName() string { return "messages" }

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

var (
	db     *gorm.DB
	config Config
)

func main() {
	godotenv.Load(".env.local")
	godotenv.Load(".env.example")

	config = parseConfig()

	var err error
	db, err = connectDB()
	if err != nil {
		log.Printf("⚠️ Erreur DB: %v", err)
	} else {
		log.Println("✅ Base de données connectée")
		db.AutoMigrate(&Session{})
	}

	fs := http.FileServer(http.Dir("front"))
	http.Handle("/front/", http.StripPrefix("/front/", fs))
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	http.HandleFunc("/", redirectHome)
	http.HandleFunc("/dashboard", handleDashboard)
	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/signup", handleSignup)
	http.HandleFunc("/api/logout", handleLogout)
	http.HandleFunc("/api/session", handleGetSession)
	http.HandleFunc("/api/contacts", handleGetContacts)
	http.HandleFunc("/api/messages", handleGetMessages)
	http.HandleFunc("/api/send-message", handleSendMessage)
	http.HandleFunc("/api/invitations", handleGetInvitations)
	http.HandleFunc("/api/send-invitation", handleSendInvitation)
	http.HandleFunc("/api/invitation-response", handleInvitationResponse)

	port := config.Port
	if port == "" {
		port = "8080"
	}
	log.Printf("🚀 Serveur lancé sur le port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// ===== AUTH & SESSION (CORRIGÉ POUR LOCALSTORAGE) =====

func getSession(r *http.Request) *Session {
	// CORRECTION : On lit le token depuis le header Authorization envoyé par le JS
	token := r.Header.Get("Authorization")
	if token == "" {
		return nil
	}

	var s Session
	if err := db.Where("idsession = ?", token).First(&s).Error; err != nil {
		return nil
	}
	return &s
}

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
	newSession := Session{
		ID:        sessionID,
		UserID:    compte.ID,
		Username:  username,
		CreatedAt: time.Now(),
	}

	if err := db.Create(&newSession).Error; err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{Success: false, Error: "Erreur lors de la création de session"})
		return
	}

	// CORRECTION : On renvoie le token au JS pour qu'il le mette dans localStorage
	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    map[string]string{"token": sessionID},
	})
}

// ===== HANDLERS API =====

func handleGetSession(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	if s == nil {
		respondJSON(w, http.StatusUnauthorized, Response{Success: false})
		return
	}
	respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{"id": s.UserID, "user": s.Username}})
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if token != "" {
		db.Where("idsession = ?", token).Delete(&Session{})
	}
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
		respondJSON(w, http.StatusBadRequest, Response{Success: false, Error: "Ce nom d'utilisateur existe déjà"})
		return
	}
	key := make([]byte, 32)
	rand.Read(key)
	iv := make([]byte, 16)
	rand.Read(iv)
	encrypted := encryptPass(password, key, iv)
	newCompte := Compte{Username: username, Password: encrypted, IV: iv, Key: key}
	if err := db.Create(&newCompte).Error; err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{Success: false, Error: "Erreur création compte"})
		return
	}
	respondJSON(w, http.StatusOK, Response{Success: true})
}

func handleGetContacts(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	if session == nil {
		respondJSON(w, http.StatusUnauthorized, Response{Success: false})
		return
	}
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
	if session == nil {
		respondJSON(w, http.StatusUnauthorized, Response{Success: false})
		return
	}
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
	if session == nil {
		respondJSON(w, http.StatusUnauthorized, Response{Success: false})
		return
	}
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

func handleGetInvitations(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	if session == nil {
		respondJSON(w, http.StatusUnauthorized, Response{Success: false})
		return
	}
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
	if session == nil {
		respondJSON(w, http.StatusUnauthorized, Response{Success: false})
		return
	}
	pseudo := r.FormValue("pseudoDestinataire")
	var dest Compte
	if err := db.Where("identifiant = ?", pseudo).First(&dest).Error; err != nil {
		respondJSON(w, http.StatusNotFound, Response{Success: false, Error: "Utilisateur inconnu"})
		return
	}
	db.Create(&Contact{SenderID: session.UserID, ReceiverID: dest.ID, Status: "en_attente"})
	respondJSON(w, http.StatusOK, Response{Success: true})
}

func handleInvitationResponse(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	if session == nil {
		respondJSON(w, http.StatusUnauthorized, Response{Success: false})
		return
	}
	senderID, _ := strconv.Atoi(r.FormValue("senderId"))
	action := r.FormValue("action")
	if action == "accepter" {
		db.Model(&Contact{}).Where("idpossesseur = ? AND iddestinataire = ?", senderID, session.UserID).Update("statut", "accepte")
		db.FirstOrCreate(&Contact{}, Contact{SenderID: session.UserID, ReceiverID: senderID, Status: "accepte"})
	} else {
		db.Where("idpossesseur = ? AND iddestinataire = ?", senderID, session.UserID).Delete(&Contact{})
	}
	respondJSON(w, http.StatusOK, Response{Success: true})
}

// ===== UTILITAIRES =====

func parseConfig() Config {
	cfg := Config{
		Port:   os.Getenv("PORT"),
		DBType: os.Getenv("DB_TYPE"),
	}
	if cfg.Port == "" {
		cfg.Port = "8081"
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		cfg.DBType = "pgsql"
		cfg.DatabaseURL = dbURL
	} else {
		cfg.DBType = "mysql"
		cfg.DBHost = os.Getenv("DB_HOST")
		cfg.DBPort = os.Getenv("DB_PORT")
		cfg.DBName = os.Getenv("DB_NAME")
		cfg.DBUser = os.Getenv("DB_USER")
		cfg.DBPass = os.Getenv("DB_PASS")
	}
	return cfg
}

func connectDB() (*gorm.DB, error) {
	var dialector gorm.Dialector
	if config.DBType == "pgsql" {
		dialector = postgres.New(postgres.Config{
			DSN:                  config.DatabaseURL,
			PreferSimpleProtocol: true,
		})
	} else {
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true",
			config.DBUser, config.DBPass, config.DBHost, config.DBPort, config.DBName)
		dialector = mysql.Open(dsn)
	}
	return gorm.Open(dialector, &gorm.Config{})
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

func serveHTML(w http.ResponseWriter, r *http.Request, path string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeFile(w, r, path)
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	serveHTML(w, r, "front/template/dashboard.html")
}

func redirectHome(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/front/template/login.html", http.StatusSeeOther)
}