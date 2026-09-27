package dto

import "time"

type NewsEventRequest struct {
	Type        string `json:"type" binding:"required"` // "NEWS" or "EVENT"
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Date        string `json:"date"`
	Category    string `json:"category"`
	Location    string `json:"location"`
	ImageURL    string `json:"imageUrl"`
	LinkURL     string `json:"linkUrl"`
	IsPublished *bool  `json:"isPublished"`
}

type NewsEventDTO struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Date        string    `json:"date"`
	Category    string    `json:"category"`
	Location    string    `json:"location"`
	ImageURL    string    `json:"imageUrl"`
	LinkURL     string    `json:"linkUrl"`
	IsPublished bool      `json:"isPublished"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type NewsEventsResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Total   int            `json:"total"`
	Items   []NewsEventDTO `json:"items"`
}
