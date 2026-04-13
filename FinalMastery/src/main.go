package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
)

func main() {
	// Get configuration from environment
	albumsTable := os.Getenv("ALBUMS_TABLE")
	photosTable := os.Getenv("PHOTOS_TABLE")
	s3Bucket := os.Getenv("S3_BUCKET")
	awsRegion := os.Getenv("AWS_REGION")
	port := os.Getenv("PORT")

	// Set defaults
	if albumsTable == "" {
		albumsTable = "albums"
	}
	if photosTable == "" {
		photosTable = "photos"
	}
	if s3Bucket == "" {
		s3Bucket = "album-photos"
	}
	if awsRegion == "" {
		awsRegion = "us-west-2"
	}
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting Album Store API")
	log.Printf("Albums Table: %s", albumsTable)
	log.Printf("Photos Table: %s", photosTable)
	log.Printf("S3 Bucket: %s", s3Bucket)
	log.Printf("AWS Region: %s", awsRegion)
	log.Printf("Port: %s", port)

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(awsRegion),
	)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	// Create AWS clients
	dynamoClient := dynamodb.NewFromConfig(cfg)
	s3Client := s3.NewFromConfig(cfg)

	// Initialize store and services
	store := NewDynamoStore(dynamoClient, albumsTable, photosTable)
	s3Svc := NewS3Client(s3Client, s3Bucket, awsRegion)
	processor := NewPhotoProcessor(store, s3Svc)
	handlers := NewHandlers(store, processor)

	// Setup Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Configure request size limits (200MB for file uploads)
	router.MaxMultipartMemory = 200 << 20 // 200 MB

	// Health check
	router.GET("/health", handlers.HealthCheck)

	// Album endpoints
	router.PUT("/albums/:album_id", handlers.PutAlbum)
	router.GET("/albums/:album_id", handlers.GetAlbum)
	router.GET("/albums", handlers.ListAlbums)

	// Photo endpoints
	router.POST("/albums/:album_id/photos", handlers.UploadPhoto)
	router.GET("/albums/:album_id/photos/:photo_id", handlers.GetPhoto)
	router.GET("/albums/:album_id/photos", handlers.ListPhotos)
	router.DELETE("/albums/:album_id/photos/:photo_id", handlers.DeletePhoto)

	// Start server
	log.Printf("Server starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
