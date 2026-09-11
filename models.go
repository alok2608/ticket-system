package main

// User is a registered account. The password hash is never sent to clients.
type User struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
	CreatedAt    string `json:"created_at"`
}

// Ticket belongs to exactly one user.
type Ticket struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"user_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// credentials is the body for /auth/register and /auth/login.
// "username" is accepted as an alias for "email" so either field name works.
type credentials struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (c credentials) identifier() string {
	if c.Email != "" {
		return c.Email
	}
	return c.Username
}

type createTicketRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type updateStatusRequest struct {
	Status string `json:"status"`
}
