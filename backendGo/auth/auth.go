package auth

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"

	"backendGo/models"
	"backendGo/session"
	"backendGo/utils"

	"database/sql"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

// Hash password
func HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

// Compare passwords
func CheckPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// Generate 2FA key for the account
func Generate2FASecret() (string, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "MyApp",            // Change to your app name
		AccountName: "user@example.com", // Replace with user email
	})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

// Verify 2FA code
func Verify2FACode(secret, code string) bool {
	return totp.Validate(code, secret)
}

// Login Handler
func LoginHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	var loginDetails struct {
		Username  string `json:"Username"`
		Password  string `json:"Password"`
		TwoFACode string `json:"TwoFACode"`
	}
	err := json.NewDecoder(r.Body).Decode(&loginDetails)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	// Query the account by username
	var account models.Account
	err = db.QueryRow("SELECT acc_id, username, email, encrypted_password, twofa_code, is_email_verified FROM accounts WHERE username = $1",
		loginDetails.Username).Scan(&account.AccID, &account.UserName, &account.Email, &account.EncryptedPassword, &account.TwoFACode, &account.IsEmailVerified)
	if err != nil || !CheckPassword(account.EncryptedPassword, loginDetails.Password) {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
		return
	}

	if !account.IsEmailVerified {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, map[string]string{"error": "Email not verified."})
		return
	}

	// Compare TwoFACode directly
	if loginDetails.TwoFACode != account.TwoFACode {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, map[string]string{"error": "Invalid 2FA code"})
		return
	}

	session.GenerateRandomSessions(db, account.AccID)
	utils.WriteJSONResponse(w, http.StatusOK, map[string]string{"message": "Login successful"})
}

// Register Handler
func RegisterHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	var accountDetails struct {
		Username  string `json:"Username"`
		Email     string `json:"Email"`
		Password  string `json:"Password"`
		TwoFACode string `json:"TwoFACode"` // Directly get TwoFACode from user
	}
	err := json.NewDecoder(r.Body).Decode(&accountDetails)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	// Check if username exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM accounts WHERE username = $1", accountDetails.Username).Scan(&count)
	if err != nil || count > 0 {
		utils.WriteJSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Username already taken"})
		return
	}

	// Hash the password
	hashedPassword, err := HashPassword(accountDetails.Password)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Error hashing password"})
		return
	}

	// Insert into the database, including TwoFACode
	var accID uint64
	err = db.QueryRow("INSERT INTO accounts (username, email, encrypted_password, twofa_code) VALUES ($1, $2, $3, $4) RETURNING acc_id",
		accountDetails.Username, accountDetails.Email, hashedPassword, accountDetails.TwoFACode).Scan(&accID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Error creating account"})
		return
	}

	// Generate verification token
	verificationToken := uuid.New().String()
	_, err = db.Exec("INSERT INTO email_verifications (acc_id, verification_token) VALUES ($1, $2)", accID, verificationToken)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Error storing verification data"})
		return
	}

	// Send verification email
	verificationLink := fmt.Sprintf("http://localhost:8080/verify-email?token=%s", verificationToken)
	err = sendVerificationEmail(accountDetails.Email, verificationLink)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Error sending verification email"})
		return
	}

	utils.WriteJSONResponse(w, http.StatusCreated, map[string]string{"message": "Account created. Verify email to activate account."})
}

// Verify Email Handler
func VerifyEmailHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	token := r.URL.Query().Get("token")
	if token == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing verification token"})
		return
	}

	// Retrieve verification data from the database
	var accID uint64
	var secretKey2FA string
	err := db.QueryRow("SELECT acc_id, secret_key_2fa FROM email_verifications WHERE verification_token = $1", token).Scan(&accID, &secretKey2FA)
	if err != nil {
		if err == sql.ErrNoRows {
			utils.WriteJSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid or expired verification token"})
		} else {
			log.Printf("Error querying verification data: %v", err)
			utils.WriteJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Error verifying account"})
		}
		return
	}

	// Mark email as verified and store the 2FA secret
	_, err = db.Exec("UPDATE accounts SET is_email_verified = TRUE, secretkey_2fa = $1 WHERE acc_id = $2", secretKey2FA, accID)
	if err != nil {
		log.Printf("Error updating account during verification: %v", err)
		utils.WriteJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Error verifying account"})
		return
	}

	// Optionally, delete the verification record
	_, err = db.Exec("DELETE FROM email_verifications WHERE verification_token = $1", token)
	if err != nil {
		log.Printf("Error deleting verification record: %v", err)
	}

	// Redirect to login page
	loginURL := "/login" // Or your desired login URL
	http.Redirect(w, r, loginURL, http.StatusFound)
}

func sendVerificationEmail(toEmail, verificationLink string) error {
	from := "namae.wa.khairul@gmail.com" // Replace with your sending email address
	password := "qezf msgy qqlm ygit"    // Replace with your email password (or use app password)
	smtpServer := "smtp.gmail.com"       // Replace with your SMTP server address
	smtpPort := "587"                    // Replace with your SMTP server port

	subject := "Verify Your Account"
	body := fmt.Sprintf("Click the following link to verify your account: %s", verificationLink)

	message := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s\r\n", toEmail, subject, body))

	auth := smtp.PlainAuth("", from, password, smtpServer)

	err := smtp.SendMail(smtpServer+":"+smtpPort, auth, from, []string{toEmail}, message)
	return err
}
