package main

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Supported ticket statuses.
const (
	StatusOpen       = "open"
	StatusInProgress = "in_progress"
	StatusClosed     = "closed"
)

func isValidStatus(s string) bool {
	return s == StatusOpen || s == StatusInProgress || s == StatusClosed
}

// canTransition implements the required flow:
//
//	open -> in_progress -> closed
//	closed is final: it can never move back to open or in_progress.
//
// Moving backwards (in_progress -> open) is rejected as well.
func canTransition(from, to string) bool {
	switch from {
	case StatusOpen:
		return to == StatusOpen || to == StatusInProgress || to == StatusClosed
	case StatusInProgress:
		return to == StatusInProgress || to == StatusClosed
	default: // closed
		return false
	}
}

// POST /tickets
func (s *server) createTicket(c *gin.Context) {
	var body createTicketRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	body.Title = strings.TrimSpace(body.Title)
	if body.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}

	userID := currentUserID(c)
	now := timestamp()
	res, err := s.db.Exec(
		`INSERT INTO tickets (user_id, title, description, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		userID, body.Title, body.Description, StatusOpen, now, now,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create ticket"})
		return
	}
	id, err := res.LastInsertId()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create ticket"})
		return
	}

	c.JSON(http.StatusCreated, Ticket{
		ID:          id,
		UserID:      userID,
		Title:       body.Title,
		Description: body.Description,
		Status:      StatusOpen,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
}

// GET /tickets - only the logged-in user's tickets.
func (s *server) listTickets(c *gin.Context) {
	rows, err := s.db.Query(
		`SELECT id, user_id, title, description, status, created_at, updated_at
		 FROM tickets WHERE user_id = ? ORDER BY id`,
		currentUserID(c),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list tickets"})
		return
	}
	defer rows.Close()

	tickets := []Ticket{} // never nil, so the response is [] and not null
	for rows.Next() {
		var t Ticket
		if err := rows.Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list tickets"})
			return
		}
		tickets = append(tickets, t)
	}

	c.JSON(http.StatusOK, tickets)
}

// GET /tickets/:id - a ticket owned by the logged-in user.
func (s *server) getTicket(c *gin.Context) {
	id, ok := ticketIDParam(c)
	if !ok {
		return
	}

	ticket, err := s.findOwnedTicket(id, currentUserID(c))
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "ticket not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch ticket"})
		return
	}

	c.JSON(http.StatusOK, ticket)
}

// PATCH /tickets/:id/status
func (s *server) updateTicketStatus(c *gin.Context) {
	id, ok := ticketIDParam(c)
	if !ok {
		return
	}

	var body updateStatusRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	next := strings.TrimSpace(body.Status)
	if !isValidStatus(next) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be one of: open, in_progress, closed"})
		return
	}

	ticket, err := s.findOwnedTicket(id, currentUserID(c))
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "ticket not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch ticket"})
		return
	}

	if !canTransition(ticket.Status, next) {
		message := "invalid status transition from " + ticket.Status + " to " + next
		if ticket.Status == StatusClosed {
			message = "a closed ticket cannot be reopened"
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": message})
		return
	}

	now := timestamp()
	if _, err := s.db.Exec(
		`UPDATE tickets SET status = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
		next, now, ticket.ID, ticket.UserID,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update ticket"})
		return
	}

	ticket.Status = next
	ticket.UpdatedAt = now
	c.JSON(http.StatusOK, ticket)
}

// findOwnedTicket returns the ticket only if it belongs to the given user,
// so another user's ticket is indistinguishable from one that does not exist.
func (s *server) findOwnedTicket(id, userID int64) (Ticket, error) {
	var t Ticket
	err := s.db.QueryRow(
		`SELECT id, user_id, title, description, status, created_at, updated_at
		 FROM tickets WHERE id = ? AND user_id = ?`,
		id, userID,
	).Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

// ticketIDParam parses :id and writes the error response itself when invalid.
func ticketIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return 0, false
	}
	return id, true
}
