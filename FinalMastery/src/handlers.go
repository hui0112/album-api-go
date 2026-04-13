package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handlers holds dependencies for HTTP handlers
type Handlers struct {
	store     AlbumStore
	processor *PhotoProcessor
}

// NewHandlers creates a new handlers instance
func NewHandlers(store AlbumStore, processor *PhotoProcessor) *Handlers {
	return &Handlers{
		store:     store,
		processor: processor,
	}
}

// HealthCheck handles the health check endpoint
func (h *Handlers) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// PutAlbum handles PUT /albums/:album_id
func (h *Handlers) PutAlbum(c *gin.Context) {
	albumID := c.Param("album_id")

	var req Album
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Invalid request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Validate album_id matches path
	if req.AlbumID != albumID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "album_id in body must match path parameter"})
		return
	}

	// Validate required fields
	if req.Title == "" || req.Owner == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title and owner are required"})
		return
	}

	isNew, err := h.store.PutAlbum(&req)
	if err != nil {
		log.Printf("Failed to put album: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	status := http.StatusOK
	if isNew {
		status = http.StatusCreated
	}

	c.JSON(status, req)
}

// GetAlbum handles GET /albums/:album_id
func (h *Handlers) GetAlbum(c *gin.Context) {
	albumID := c.Param("album_id")

	album, err := h.store.GetAlbum(albumID)
	if err != nil {
		log.Printf("Album not found: %s", albumID)
		c.JSON(http.StatusNotFound, gin.H{"error": "album not found"})
		return
	}

	c.JSON(http.StatusOK, album)
}

// ListAlbums handles GET /albums
func (h *Handlers) ListAlbums(c *gin.Context) {
	albums, err := h.store.ListAlbums()
	if err != nil {
		log.Printf("Failed to list albums: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	c.JSON(http.StatusOK, albums)
}

// UploadPhoto handles POST /albums/:album_id/photos
func (h *Handlers) UploadPhoto(c *gin.Context) {
	albumID := c.Param("album_id")

	// Check album exists
	album, err := h.store.GetAlbum(albumID)
	if err != nil {
		log.Printf("Album not found for photo upload: %s", albumID)
		c.JSON(http.StatusNotFound, gin.H{"error": "album not found"})
		return
	}

	log.Printf("Uploading photo to album: %s (title: %s)", albumID, album.Title)

	// Get uploaded file
	file, err := c.FormFile("photo")
	if err != nil {
		log.Printf("Missing photo field: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing photo field in form data"})
		return
	}

	log.Printf("Received file: %s, size: %d bytes", file.Filename, file.Size)

	// Atomically increment sequence number
	seq, err := h.store.IncrementSeq(albumID)
	if err != nil {
		log.Printf("Failed to increment sequence: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sequence error"})
		return
	}

	// Generate photo ID
	photoID := uuid.New().String()

	// Create photo record (status=processing)
	photo := &Photo{
		PhotoID: photoID,
		AlbumID: albumID,
		Seq:     seq,
		Status:  "processing",
	}
	if err := h.store.CreatePhoto(photo); err != nil {
		log.Printf("Failed to create photo record: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	// Launch background processing
	go h.processor.ProcessPhoto(photoID, albumID, file)

	// Return 202 immediately
	c.JSON(http.StatusAccepted, PhotoUploadResponse{
		PhotoID: photoID,
		Seq:     seq,
		Status:  "processing",
	})
}

// GetPhoto handles GET /albums/:album_id/photos/:photo_id
func (h *Handlers) GetPhoto(c *gin.Context) {
	albumID := c.Param("album_id")
	photoID := c.Param("photo_id")

	photo, err := h.store.GetPhoto(albumID, photoID)
	if err != nil {
		log.Printf("Photo not found: album=%s, photo=%s", albumID, photoID)
		c.JSON(http.StatusNotFound, gin.H{"error": "photo not found"})
		return
	}

	// Verify photo belongs to the album
	if photo.AlbumID != albumID {
		c.JSON(http.StatusNotFound, gin.H{"error": "photo not found in this album"})
		return
	}

	c.JSON(http.StatusOK, photo)
}

// ListPhotos handles GET /albums/:album_id/photos
func (h *Handlers) ListPhotos(c *gin.Context) {
	albumID := c.Param("album_id")

	// Check album exists
	_, err := h.store.GetAlbum(albumID)
	if err != nil {
		log.Printf("Album not found for photo list: %s", albumID)
		c.JSON(http.StatusNotFound, gin.H{"error": "album not found"})
		return
	}

	photos, err := h.store.ListPhotos(albumID)
	if err != nil {
		log.Printf("Failed to list photos: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	c.JSON(http.StatusOK, photos)
}

// DeletePhoto handles DELETE /albums/:album_id/photos/:photo_id
func (h *Handlers) DeletePhoto(c *gin.Context) {
	albumID := c.Param("album_id")
	photoID := c.Param("photo_id")

	// Get photo to find S3 URL
	photo, err := h.store.GetPhoto(albumID, photoID)
	if err != nil {
		log.Printf("Photo not found for deletion: album=%s, photo=%s", albumID, photoID)
		c.JSON(http.StatusNotFound, gin.H{"error": "photo not found"})
		return
	}

	// Verify photo belongs to the album
	if photo.AlbumID != albumID {
		c.JSON(http.StatusNotFound, gin.H{"error": "photo not found in this album"})
		return
	}

	log.Printf("Deleting photo: %s from album: %s", photoID, albumID)

	// Delete in parallel
	errCh := make(chan error, 2)

	// Delete from DynamoDB
	go func() {
		errCh <- h.store.DeletePhoto(photoID)
	}()

	// Delete from S3 (only if URL exists)
	go func() {
		if photo.URL != "" {
			errCh <- h.processor.s3Client.DeleteObject(photo.URL)
		} else {
			errCh <- nil
		}
	}()

	// Wait for both operations
	var hasError bool
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			log.Printf("Delete error: %v", err)
			hasError = true
		}
	}

	if hasError {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete photo"})
		return
	}

	log.Printf("Photo deleted successfully: %s", photoID)
	c.Status(http.StatusNoContent)
}
