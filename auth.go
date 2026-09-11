package main

import (
	"crypto/rand"
	"database/sql"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const tokenTTL = 24 * time.Hour

// resolveJWTSecret returns the configured signing key. When JWT_SECRET is not
// set it generates a random one instead of falling back to a fixed string, so
// no usable signing key is ever shipped in the source or the image.
//
// The trade-off is that tokens issued before a restart stop validating after
// it, which is why a real deployment must set JWT_SECRET.
func resolveJWTSecret() []byte {
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		return []byte(secret)
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		log.Fatalf("generate JWT secret: %v", err)
	}
	log.Println("WARNING: JWT_SECRET is not set - using a random key for this run; tokens will stop working after a restart")
	return key
}

// issueToken signs a JWT that carries the user id as the subject.
func issueToken(userID int64) (string, error) {
	claims := jwt.MapClaims{
		"sub":     strconv.FormatInt(userID, 10),
		"user_id": userID,
		"exp":     time.Now().Add(tokenTTL).Unix(),
		"iat":     time.Now().Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtSecret)
}

// POST /auth/register
func register(c *gin.Context) {
	var body credentials
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	email, password := body.identifier(), body.Password
	if email == "" || password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password are required"})
		return
	}

	var exists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE email = ?`, email).Scan(&exists); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not register user"})
		return
	}
	if exists > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "user already exists"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not register user"})
		return
	}

	now := timestamp()
	res, err := db.Exec(
		`INSERT INTO users (email, password_hash, created_at) VALUES (?, ?, ?)`,
		email, string(hash), now,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not register user"})
		return
	}
	id, _ := res.LastInsertId()

	token, err := issueToken(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":         id,
		"user_id":    id,
		"email":      email,
		"created_at": now,
		// A token is returned here too so clients can skip a separate login call.
		"token":        token,
		"access_token": token,
	})
}

// POST /auth/login
func login(c *gin.Context) {
	var body credentials
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	email, password := body.identifier(), body.Password
	if email == "" || password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password are required"})
		return
	}

	var user User
	err := db.QueryRow(
		`SELECT id, email, password_hash FROM users WHERE email = ?`, email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not log in"})
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := issueToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":        token,
		"access_token": token,
		"token_type":   "Bearer",
		"user_id":      user.ID,
		"email":        user.Email,
	})
}
