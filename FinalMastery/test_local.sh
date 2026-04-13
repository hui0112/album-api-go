#!/bin/bash
# Local testing script for Album Store API

set -e

echo "========================================="
echo "Album Store Local Testing"
echo "========================================="

# Check if server is running
if ! curl -s http://localhost:8080/health > /dev/null 2>&1; then
    echo "Error: Server is not running on http://localhost:8080"
    echo "Please start the server first with: go run src/*.go"
    exit 1
fi

BASE_URL="http://localhost:8080"
ALBUM_ID="test-album-$(date +%s)"
echo "Using Album ID: $ALBUM_ID"

# Test 1: Health check
echo ""
echo "Test 1: Health Check"
curl -s "$BASE_URL/health" | jq '.'

# Test 2: Create album
echo ""
echo "Test 2: Create Album (PUT $BASE_URL/albums/$ALBUM_ID)"
RESPONSE=$(curl -s -X PUT "$BASE_URL/albums/$ALBUM_ID" \
  -H "Content-Type: application/json" \
  -d "{
    \"album_id\": \"$ALBUM_ID\",
    \"title\": \"Test Album\",
    \"description\": \"Testing the album store API\",
    \"owner\": \"test@example.com\"
  }")
echo "$RESPONSE" | jq '.'
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$BASE_URL/albums/$ALBUM_ID" \
  -H "Content-Type: application/json" \
  -d "{
    \"album_id\": \"$ALBUM_ID\",
    \"title\": \"Test Album\",
    \"description\": \"Testing the album store API\",
    \"owner\": \"test@example.com\"
  }")
echo "HTTP Status: $HTTP_CODE (should be 201 for new, 200 for update)"

# Test 3: Get album
echo ""
echo "Test 3: Get Album (GET $BASE_URL/albums/$ALBUM_ID)"
curl -s "$BASE_URL/albums/$ALBUM_ID" | jq '.'

# Test 4: List albums
echo ""
echo "Test 4: List All Albums (GET $BASE_URL/albums)"
curl -s "$BASE_URL/albums" | jq '. | length' | xargs echo "Total albums:"

# Test 5: Upload photo (need a test image)
echo ""
echo "Test 5: Upload Photo"
# Create a small test image if it doesn't exist
if [ ! -f "test.jpg" ]; then
    echo "Creating test image..."
    # Create a 1x1 pixel JPEG
    echo -e "\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00\xFF\xDB\x00C\x00\x08\x06\x06\x07\x06\x05\x08\x07\x07\x07\t\t\x08\n\x0C\x14\r\x0C\x0B\x0B\x0C\x19\x12\x13\x0F\x14\x1D\x1A\x1F\x1E\x1D\x1A\x1C\x1C $.' \",#\x1C\x1C(7),01444\x1F'9=82<.342\xFF\xC0\x00\x0B\x08\x00\x01\x00\x01\x01\x01\x11\x00\xFF\xC4\x00\x14\x00\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\xFF\xDA\x00\x08\x01\x01\x00\x00?\x00\x7F\x00\xFF\xD9" > test.jpg
fi

PHOTO_RESPONSE=$(curl -s -X POST "$BASE_URL/albums/$ALBUM_ID/photos" \
  -F "photo=@test.jpg")
echo "$PHOTO_RESPONSE" | jq '.'
PHOTO_ID=$(echo "$PHOTO_RESPONSE" | jq -r '.photo_id')
SEQ=$(echo "$PHOTO_RESPONSE" | jq -r '.seq')
echo "Photo ID: $PHOTO_ID"
echo "Sequence: $SEQ"

# Test 6: Get photo (may be processing)
echo ""
echo "Test 6: Get Photo Status (GET $BASE_URL/albums/$ALBUM_ID/photos/$PHOTO_ID)"
sleep 2  # Wait for background processing
curl -s "$BASE_URL/albums/$ALBUM_ID/photos/$PHOTO_ID" | jq '.'

# Test 7: List photos
echo ""
echo "Test 7: List Photos in Album (GET $BASE_URL/albums/$ALBUM_ID/photos)"
curl -s "$BASE_URL/albums/$ALBUM_ID/photos" | jq '.'

# Test 8: Upload another photo (test sequence increment)
echo ""
echo "Test 8: Upload Second Photo (test sequence increment)"
PHOTO2_RESPONSE=$(curl -s -X POST "$BASE_URL/albums/$ALBUM_ID/photos" \
  -F "photo=@test.jpg")
echo "$PHOTO2_RESPONSE" | jq '.'
PHOTO2_ID=$(echo "$PHOTO2_RESPONSE" | jq -r '.photo_id')
SEQ2=$(echo "$PHOTO2_RESPONSE" | jq -r '.seq')
echo "Second photo sequence: $SEQ2 (should be $((SEQ + 1)))"

# Test 9: Delete photo
echo ""
echo "Test 9: Delete Photo (DELETE $BASE_URL/albums/$ALBUM_ID/photos/$PHOTO_ID)"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/albums/$ALBUM_ID/photos/$PHOTO_ID")
echo "HTTP Status: $HTTP_CODE (should be 204)"

# Test 10: Verify deletion
echo ""
echo "Test 10: Verify Photo Deleted (should return 404)"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/albums/$ALBUM_ID/photos/$PHOTO_ID")
echo "HTTP Status: $HTTP_CODE (should be 404)"

echo ""
echo "========================================="
echo "All tests completed!"
echo "========================================="
echo ""
echo "Cleanup: To delete the test album and remaining photos,"
echo "you would need to implement DELETE /albums/:album_id endpoint"
echo "(not required for the assignment)"
