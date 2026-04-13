package main

import (
	"fmt"
	"log"
	"mime/multipart"
)

// PhotoProcessor handles async photo processing
type PhotoProcessor struct {
	store     AlbumStore
	s3Client  *S3Client
}

// NewPhotoProcessor creates a new photo processor
func NewPhotoProcessor(store AlbumStore, s3Client *S3Client) *PhotoProcessor {
	return &PhotoProcessor{
		store:    store,
		s3Client: s3Client,
	}
}

// ProcessPhoto processes a photo upload asynchronously
func (p *PhotoProcessor) ProcessPhoto(photoID, albumID string, fileHeader *multipart.FileHeader) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Photo processing panic for %s: %v", photoID, r)
			if err := p.store.UpdatePhotoStatus(photoID, "failed", ""); err != nil {
				log.Printf("Failed to update photo status after panic: %v", err)
			}
		}
	}()

	log.Printf("Processing photo: %s for album: %s", photoID, albumID)

	// Open file
	file, err := fileHeader.Open()
	if err != nil {
		log.Printf("Failed to open file for photo %s: %v", photoID, err)
		p.store.UpdatePhotoStatus(photoID, "failed", "")
		return
	}
	defer file.Close()

	// Upload to S3
	s3Key := fmt.Sprintf("albums/%s/photos/%s", albumID, photoID)
	url, err := p.s3Client.UploadFile(s3Key, file, fileHeader.Size)
	if err != nil {
		log.Printf("S3 upload failed for photo %s: %v", photoID, err)
		p.store.UpdatePhotoStatus(photoID, "failed", "")
		return
	}

	// Update status to completed
	if err := p.store.UpdatePhotoStatus(photoID, "completed", url); err != nil {
		log.Printf("Status update failed for photo %s: %v", photoID, err)
		return
	}

	log.Printf("Photo processing completed: %s -> %s", photoID, url)
}
