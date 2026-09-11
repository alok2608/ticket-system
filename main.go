package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// server holds the dependencies the handlers need. Passing it explicitly keeps
// package-level mutable state out of the program and lets tests build a server
// against a temporary database.
type server struct {
	db        *sql.DB
	jwtSecret []byte
}

func main() {
	// Values already set in the real environment take priority over the file.
	loadEnvFile(env("ENV_FILE", ".env"))

	db := openDB(env("DB_PATH", "tickets.db"))
	defer db.Close()

	srv := &server{db: db, jwtSecret: resolveJWTSecret()}

	r := srv.routes()

	addr := ":" + env("PORT", "8080")
	log.Printf("ticket-system listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// timestamp returns the current UTC time in RFC3339, the format used for
// created_at / updated_at everywhere.
func timestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// routes builds the router. Kept separate from main so tests can exercise the
// real routing table without starting a server.
func (s *server) routes() *gin.Engine {
	r := gin.Default()

	// Frontend. The API routes below are unchanged.
	r.GET("/", serveIndex)
	r.GET("/style.css", serveAsset("style.css", "text/css; charset=utf-8"))
	r.GET("/app.js", serveAsset("app.js", "text/javascript; charset=utf-8"))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.POST("/auth/register", s.register)
	r.POST("/auth/login", s.login)

	// Every /tickets route requires a valid JWT.
	tickets := r.Group("/tickets", s.authRequired())
	{
		tickets.POST("", s.createTicket)
		tickets.GET("", s.listTickets)
		tickets.GET("/:id", s.getTicket)
		tickets.PATCH("/:id/status", s.updateTicketStatus)
	}

	return r
}
