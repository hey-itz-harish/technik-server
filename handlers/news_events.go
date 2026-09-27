package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"technik-server/database"
	"technik-server/dto"

	"github.com/gin-gonic/gin"
)

// GetNewsEvents fetches news and events from PostgreSQL database
func (h *AdminHandler) GetNewsEvents(c *gin.Context) {
	db := database.GetSQLDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	filterType := strings.ToUpper(strings.TrimSpace(c.Query("type")))
	onlyPublished := c.Query("published") == "true"

	query := `SELECT id, type, title, COALESCE(description, ''), COALESCE(date, ''), COALESCE(category, ''), COALESCE(location, ''), COALESCE(image_url, ''), COALESCE(link_url, ''), is_published, created_at, updated_at FROM "NewsEvents" WHERE 1=1`
	var args []interface{}
	argIdx := 1

	if filterType != "" && (filterType == "NEWS" || filterType == "EVENT") {
		query += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, filterType)
		argIdx++
	}

	if onlyPublished {
		query += fmt.Sprintf(" AND is_published = $%d", argIdx)
		args = append(args, true)
		argIdx++
	}

	query += " ORDER BY created_at DESC;"

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to fetch news and events: " + err.Error()})
		return
	}
	defer rows.Close()

	var items []dto.NewsEventDTO
	for rows.Next() {
		var item dto.NewsEventDTO
		err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.Title,
			&item.Description,
			&item.Date,
			&item.Category,
			&item.Location,
			&item.ImageURL,
			&item.LinkURL,
			&item.IsPublished,
			&item.CreatedAt,
			&item.UpdatedAt,
		)
		if err == nil {
			items = append(items, item)
		}
	}

	if items == nil {
		items = []dto.NewsEventDTO{}
	}

	c.JSON(http.StatusOK, dto.NewsEventsResponse{
		Success: true,
		Message: "News and events fetched successfully",
		Total:   len(items),
		Items:   items,
	})
}

// CreateNewsEvent adds a new news article or upcoming event to PostgreSQL
func (h *AdminHandler) CreateNewsEvent(c *gin.Context) {
	db := database.GetSQLDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	var req dto.NewsEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	itemType := strings.ToUpper(strings.TrimSpace(req.Type))
	if itemType != "NEWS" && itemType != "EVENT" {
		itemType = "NEWS"
	}

	idPrefix := "news"
	if itemType == "EVENT" {
		idPrefix = "event"
	}
	newID := fmt.Sprintf("%s-%d", idPrefix, time.Now().UnixNano())

	isPub := true
	if req.IsPublished != nil {
		isPub = *req.IsPublished
	}

	query := `
		INSERT INTO "NewsEvents" ("id", "type", "title", "description", "date", "category", "location", "image_url", "link_url", "is_published", "created_at", "updated_at")
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		RETURNING id, type, title, COALESCE(description, ''), COALESCE(date, ''), COALESCE(category, ''), COALESCE(location, ''), COALESCE(image_url, ''), COALESCE(link_url, ''), is_published, created_at, updated_at;
	`

	var item dto.NewsEventDTO
	err := db.QueryRow(
		query,
		newID,
		itemType,
		req.Title,
		req.Description,
		req.Date,
		req.Category,
		req.Location,
		req.ImageURL,
		req.LinkURL,
		isPub,
	).Scan(
		&item.ID,
		&item.Type,
		&item.Title,
		&item.Description,
		&item.Date,
		&item.Category,
		&item.Location,
		&item.ImageURL,
		&item.LinkURL,
		&item.IsPublished,
		&item.CreatedAt,
		&item.UpdatedAt,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to create news/event: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": fmt.Sprintf("%s created successfully", itemType),
		"item":    item,
	})
}

// UpdateNewsEvent modifies an existing news or event by ID
func (h *AdminHandler) UpdateNewsEvent(c *gin.Context) {
	db := database.GetSQLDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Missing item ID"})
		return
	}

	var req dto.NewsEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	itemType := strings.ToUpper(strings.TrimSpace(req.Type))
	if itemType != "NEWS" && itemType != "EVENT" {
		itemType = "NEWS"
	}

	isPub := true
	if req.IsPublished != nil {
		isPub = *req.IsPublished
	}

	query := `
		UPDATE "NewsEvents"
		SET type = $1, title = $2, description = $3, date = $4, category = $5, location = $6, image_url = $7, link_url = $8, is_published = $9, updated_at = NOW()
		WHERE id = $10
		RETURNING id, type, title, COALESCE(description, ''), COALESCE(date, ''), COALESCE(category, ''), COALESCE(location, ''), COALESCE(image_url, ''), COALESCE(link_url, ''), is_published, created_at, updated_at;
	`

	var item dto.NewsEventDTO
	err := db.QueryRow(
		query,
		itemType,
		req.Title,
		req.Description,
		req.Date,
		req.Category,
		req.Location,
		req.ImageURL,
		req.LinkURL,
		isPub,
		id,
	).Scan(
		&item.ID,
		&item.Type,
		&item.Title,
		&item.Description,
		&item.Date,
		&item.Category,
		&item.Location,
		&item.ImageURL,
		&item.LinkURL,
		&item.IsPublished,
		&item.CreatedAt,
		&item.UpdatedAt,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to update news/event: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("%s updated successfully", itemType),
		"item":    item,
	})
}

// DeleteNewsEvent removes a news or event item by ID
func (h *AdminHandler) DeleteNewsEvent(c *gin.Context) {
	db := database.GetSQLDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Database connection unavailable"})
		return
	}

	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: "Missing item ID"})
		return
	}

	res, err := db.Exec(`DELETE FROM "NewsEvents" WHERE id = $1;`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to delete item: " + err.Error()})
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, dto.APIError{Success: false, Error: "Item not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Item deleted successfully",
		"id":      id,
	})
}
