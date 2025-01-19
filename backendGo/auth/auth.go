package auth

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"

	"backendGo/models"
	"backendGo/session"
	"backendGo/utils"

	"database/sql"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

// Load the environment variables
func loadEnvVariables() error {
	err := godotenv.Load("./.env")
	if err != nil {
		return fmt.Errorf("error loading .env file: %v", err)
	}
	fmt.Println("Environment variables loaded successfully.")
	fmt.Println("SMTP_FROM:", os.Getenv("SMTP_FROM"))
	fmt.Println("SMTP_PASSWORD:", os.Getenv("SMTP_PASSWORD"))
	fmt.Println("SMTP_SERVER:", os.Getenv("SMTP_SERVER"))
	fmt.Println("SMTP_PORT:", os.Getenv("SMTP_PORT"))

	return nil
}

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
	// Extract login details
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
	err = db.QueryRow("SELECT acc_id, username, email, encrypted_password, secretkey_2fa, is_email_verified FROM accounts WHERE username = $1", loginDetails.Username).Scan(
		&account.AccID, &account.UserName, &account.Email, &account.EncryptedPassword, &account.SecretKey2FA, &account.IsEmailVerified,
	)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
		return
	}

	// Check if email is verified
	if !account.IsEmailVerified {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, map[string]string{"error": "Email not verified. Please check your email."})
		return
	}

	// Check password
	if !CheckPassword(account.EncryptedPassword, loginDetails.Password) {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
		return
	}

	// Verify 2FA code
	if !Verify2FACode(account.SecretKey2FA, loginDetails.TwoFACode) {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, map[string]string{"error": "Invalid 2FA code"})
		return
	}

	// Generate session for the account if it doesn't already have one
	session.GenerateRandomSessions(db, account.AccID)

	// Respond with success message
	utils.WriteJSONResponse(w, http.StatusOK, map[string]string{"message": "Login successful"})
}

// Register Handler
func RegisterHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	// Extract account details
	var accountDetails struct {
		Username string `json:"Username"`
		Email    string `json:"Email"`
		Password string `json:"Password"`
	}
	err := json.NewDecoder(r.Body).Decode(&accountDetails)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	// Check if username already exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM accounts WHERE username = $1", accountDetails.Username).Scan(&count)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Error checking username"})
		return
	}

	if count > 0 {
		utils.WriteJSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Username already taken"})
		return
	}

	// Hash password
	hashedPassword, err := HashPassword(accountDetails.Password)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Error hashing password"})
		return
	}

	// Generate 2FA secret (but don't store it in the account yet)
	secret, _, err := Generate2FASecret()
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Error generating 2FA secret"})
		return
	}

	// Generate unique verification token
	verificationToken := uuid.New().String()

	// Insert new account into the database (without secretkey_2fa, is_email_verified = false)
	var accID uint64
	err = db.QueryRow("INSERT INTO accounts (username, email, encrypted_password) VALUES ($1, $2, $3) RETURNING acc_id", accountDetails.Username, accountDetails.Email, hashedPassword).Scan(&accID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Error creating account"})
		return
	}

	// Store verification token and secret temporarily (using database)
	_, err = db.Exec("INSERT INTO email_verifications (acc_id, verification_token, secret_key_2fa) VALUES ($1, $2, $3)", accID, verificationToken, secret)
	if err != nil {
		log.Printf("Error storing verification data: %v", err)
		utils.WriteJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Error storing verification data"})
		return
	}

	// Send verification email
	verificationLink := fmt.Sprintf("http://localhost:8080/verify-email?token=%s", verificationToken)
	err = sendVerificationEmail(accountDetails.Email, verificationLink)
	if err != nil {
		log.Printf("Error sending verification email: %v", err)
		// Consider how to handle this - maybe allow resending or manual verification
		utils.WriteJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Error sending verification email"})
		return
	}

	// Respond with success message
	utils.WriteJSONResponse(w, http.StatusCreated, map[string]string{"message": "Account created. Please check your email to verify your account."})
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
	// Load environment variables
	err := loadEnvVariables()
	if err != nil {
		return fmt.Errorf("error loading environment variables: %v", err)
	}

	from := os.Getenv("SMTP_FROM") // Read from the .env file
	password := os.Getenv("SMTP_PASSWORD")
	smtpServer := os.Getenv("SMTP_SERVER")
	smtpPort := os.Getenv("SMTP_PORT")

	subject := "Verify Your Account"
	body := fmt.Sprintf("Click the following link to verify your account: %s", verificationLink)

	message := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s\r\n", toEmail, subject, body))

	auth := smtp.PlainAuth("", from, password, smtpServer)

	err = smtp.SendMail(smtpServer+":"+smtpPort, auth, from, []string{toEmail}, message)
	return err
}
