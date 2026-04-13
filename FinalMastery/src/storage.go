package main

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// AlbumStore defines the interface for album and photo storage
type AlbumStore interface {
	// Albums
	PutAlbum(album *Album) (isNew bool, error error)
	GetAlbum(albumID string) (*Album, error)
	ListAlbums() ([]*Album, error)

	// Photos
	CreatePhoto(photo *Photo) error
	GetPhoto(albumID, photoID string) (*Photo, error)
	GetPhotoByID(photoID string) (*Photo, error)
	ListPhotos(albumID string) ([]*Photo, error)
	UpdatePhotoStatus(photoID, status, url string) error
	DeletePhoto(photoID string) error

	// Atomic sequence increment
	IncrementSeq(albumID string) (int, error)
}

// DynamoStore implements AlbumStore using DynamoDB
type DynamoStore struct {
	client      *dynamodb.Client
	albumsTable string
	photosTable string
}

// NewDynamoStore creates a new DynamoDB store
func NewDynamoStore(client *dynamodb.Client, albumsTable, photosTable string) *DynamoStore {
	return &DynamoStore{
		client:      client,
		albumsTable: albumsTable,
		photosTable: photosTable,
	}
}

// PutAlbum creates or updates an album
func (d *DynamoStore) PutAlbum(album *Album) (bool, error) {
	// Initialize next_seq to 0 for new albums
	if album.NextSeq == 0 {
		album.NextSeq = 0
	}

	// First, try to get the existing album
	existing, err := d.GetAlbum(album.AlbumID)
	isNew := err != nil || existing == nil

	// Preserve next_seq if album exists
	if !isNew && existing.NextSeq > 0 {
		album.NextSeq = existing.NextSeq
	}

	item, err := attributevalue.MarshalMap(album)
	if err != nil {
		return false, fmt.Errorf("failed to marshal album: %w", err)
	}

	_, err = d.client.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(d.albumsTable),
		Item:      item,
	})
	if err != nil {
		return false, fmt.Errorf("failed to put album: %w", err)
	}

	return isNew, nil
}

// GetAlbum retrieves an album by ID
func (d *DynamoStore) GetAlbum(albumID string) (*Album, error) {
	result, err := d.client.GetItem(context.TODO(), &dynamodb.GetItemInput{
		TableName: aws.String(d.albumsTable),
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get album: %w", err)
	}

	if result.Item == nil {
		return nil, fmt.Errorf("album not found")
	}

	var album Album
	if err := attributevalue.UnmarshalMap(result.Item, &album); err != nil {
		return nil, fmt.Errorf("failed to unmarshal album: %w", err)
	}

	return &album, nil
}

// ListAlbums retrieves all albums with pagination
func (d *DynamoStore) ListAlbums() ([]*Album, error) {
	var albums []*Album
	var lastKey map[string]types.AttributeValue

	for {
		input := &dynamodb.ScanInput{
			TableName: aws.String(d.albumsTable),
		}
		if lastKey != nil {
			input.ExclusiveStartKey = lastKey
		}

		output, err := d.client.Scan(context.TODO(), input)
		if err != nil {
			return nil, fmt.Errorf("failed to scan albums: %w", err)
		}

		var batch []*Album
		if err := attributevalue.UnmarshalListOfMaps(output.Items, &batch); err != nil {
			return nil, fmt.Errorf("failed to unmarshal albums: %w", err)
		}
		albums = append(albums, batch...)

		// Check for more pages
		if output.LastEvaluatedKey == nil {
			break
		}
		lastKey = output.LastEvaluatedKey
	}

	return albums, nil
}

// CreatePhoto creates a new photo record
func (d *DynamoStore) CreatePhoto(photo *Photo) error {
	item, err := attributevalue.MarshalMap(photo)
	if err != nil {
		return fmt.Errorf("failed to marshal photo: %w", err)
	}

	_, err = d.client.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(d.photosTable),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("failed to create photo: %w", err)
	}

	return nil
}

// GetPhoto retrieves a photo by album ID and photo ID
func (d *DynamoStore) GetPhoto(albumID, photoID string) (*Photo, error) {
	return d.GetPhotoByID(photoID)
}

// GetPhotoByID retrieves a photo by photo ID only
func (d *DynamoStore) GetPhotoByID(photoID string) (*Photo, error) {
	result, err := d.client.GetItem(context.TODO(), &dynamodb.GetItemInput{
		TableName: aws.String(d.photosTable),
		Key: map[string]types.AttributeValue{
			"photo_id": &types.AttributeValueMemberS{Value: photoID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get photo: %w", err)
	}

	if result.Item == nil {
		return nil, fmt.Errorf("photo not found")
	}

	var photo Photo
	if err := attributevalue.UnmarshalMap(result.Item, &photo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal photo: %w", err)
	}

	return &photo, nil
}

// ListPhotos retrieves all photos for an album
func (d *DynamoStore) ListPhotos(albumID string) ([]*Photo, error) {
	var photos []*Photo
	var lastKey map[string]types.AttributeValue

	for {
		input := &dynamodb.QueryInput{
			TableName:              aws.String(d.photosTable),
			IndexName:              aws.String("album_id-index"),
			KeyConditionExpression: aws.String("album_id = :album_id"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":album_id": &types.AttributeValueMemberS{Value: albumID},
			},
		}
		if lastKey != nil {
			input.ExclusiveStartKey = lastKey
		}

		output, err := d.client.Query(context.TODO(), input)
		if err != nil {
			return nil, fmt.Errorf("failed to query photos: %w", err)
		}

		var batch []*Photo
		if err := attributevalue.UnmarshalListOfMaps(output.Items, &batch); err != nil {
			return nil, fmt.Errorf("failed to unmarshal photos: %w", err)
		}
		photos = append(photos, batch...)

		// Check for more pages
		if output.LastEvaluatedKey == nil {
			break
		}
		lastKey = output.LastEvaluatedKey
	}

	return photos, nil
}

// UpdatePhotoStatus updates the status and URL of a photo
func (d *DynamoStore) UpdatePhotoStatus(photoID, status, url string) error {
	updateExpr := "SET #status = :status"
	exprAttrNames := map[string]string{
		"#status": "status",
	}
	exprAttrValues := map[string]types.AttributeValue{
		":status": &types.AttributeValueMemberS{Value: status},
	}

	if url != "" {
		updateExpr += ", #url = :url"
		exprAttrNames["#url"] = "url"
		exprAttrValues[":url"] = &types.AttributeValueMemberS{Value: url}
	}

	_, err := d.client.UpdateItem(context.TODO(), &dynamodb.UpdateItemInput{
		TableName: aws.String(d.photosTable),
		Key: map[string]types.AttributeValue{
			"photo_id": &types.AttributeValueMemberS{Value: photoID},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeNames:  exprAttrNames,
		ExpressionAttributeValues: exprAttrValues,
	})
	if err != nil {
		return fmt.Errorf("failed to update photo status: %w", err)
	}

	return nil
}

// DeletePhoto deletes a photo record
func (d *DynamoStore) DeletePhoto(photoID string) error {
	_, err := d.client.DeleteItem(context.TODO(), &dynamodb.DeleteItemInput{
		TableName: aws.String(d.photosTable),
		Key: map[string]types.AttributeValue{
			"photo_id": &types.AttributeValueMemberS{Value: photoID},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete photo: %w", err)
	}

	return nil
}

// IncrementSeq atomically increments the sequence number for an album
func (d *DynamoStore) IncrementSeq(albumID string) (int, error) {
	output, err := d.client.UpdateItem(context.TODO(), &dynamodb.UpdateItemInput{
		TableName: aws.String(d.albumsTable),
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
		},
		UpdateExpression: aws.String("ADD next_seq :inc"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":inc": &types.AttributeValueMemberN{Value: "1"},
		},
		ReturnValues: types.ReturnValueUpdatedNew,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to increment sequence: %w", err)
	}

	// Extract new seq value
	seqAttr, ok := output.Attributes["next_seq"]
	if !ok {
		return 0, fmt.Errorf("next_seq not returned")
	}

	seqNum, ok := seqAttr.(*types.AttributeValueMemberN)
	if !ok {
		return 0, fmt.Errorf("next_seq is not a number")
	}

	seq, err := strconv.Atoi(seqNum.Value)
	if err != nil {
		return 0, fmt.Errorf("failed to parse sequence number: %w", err)
	}

	log.Printf("Incremented sequence for album %s to %d", albumID, seq)
	return seq, nil
}
