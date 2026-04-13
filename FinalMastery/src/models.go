package main

// Album represents an album in the system
type Album struct {
	AlbumID     string `dynamodbav:"album_id" json:"album_id"`
	Title       string `dynamodbav:"title" json:"title"`
	Description string `dynamodbav:"description" json:"description"`
	Owner       string `dynamodbav:"owner" json:"owner"`
	NextSeq     int    `dynamodbav:"next_seq" json:"-"` // Hidden from API
}

// Photo represents a photo in an album
type Photo struct {
	PhotoID string `dynamodbav:"photo_id" json:"photo_id"`
	AlbumID string `dynamodbav:"album_id" json:"album_id"`
	Seq     int    `dynamodbav:"seq" json:"seq"`
	Status  string `dynamodbav:"status" json:"status"` // "processing", "completed", "failed"
	URL     string `dynamodbav:"url,omitempty" json:"url,omitempty"`
}

// PhotoUploadResponse is the response for photo upload
type PhotoUploadResponse struct {
	PhotoID string `json:"photo_id"`
	Seq     int    `json:"seq"`
	Status  string `json:"status"`
}
