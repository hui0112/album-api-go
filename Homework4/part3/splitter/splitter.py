from flask import Flask, request, jsonify
import boto3
import requests
import os

# Initialize the Flask application
app = Flask(__name__)

# Initialize the S3 client using boto3
# Note: It will automatically use the LabRole credentials when running on Fargate
s3 = boto3.client('s3')

# Configuration: Replace with your actual bucket name created in the previous step
BUCKET_NAME = 'mapreduce-hui-002492813'

@app.route('/split', methods=['POST'])
def split_and_upload():
    """
    This endpoint receives a JSON payload with the 'url' of the input file.
    It downloads the file, splits it into 3 parts, uploads them to S3,
    and returns the S3 keys for the parts.
    """
    try:
        # 1. Get the input file URL from the request
        data = request.get_json()
        if not data or 'url' not in data:
            return jsonify({'error': 'Please provide a "url" in the JSON body'}), 400

        input_url = data['url']
        print(f"Downloading file from: {input_url}")

        # 2. Download the content of the file
        response = requests.get(input_url)
        response.raise_for_status() # Check for download errors
        content = response.text
        lines = content.splitlines() # Split content into a list of lines

        # 3. Calculate the split size
        total_lines = len(lines)
        chunk_size = total_lines // 3 + (1 if total_lines % 3 > 0 else 0)

        print(f"Total lines: {total_lines}, Chunk size: {chunk_size}")

        # 4. Split the content into 3 parts
        parts = []
        parts.append("\n".join(lines[:chunk_size]))
        parts.append("\n".join(lines[chunk_size:2*chunk_size]))
        parts.append("\n".join(lines[2*chunk_size:]))

        uploaded_files = []

        # 5. Upload each part to S3
        for i, part_content in enumerate(parts):
            file_name = f"split_{i+1}.txt"
            print(f"Uploading {file_name} to S3 bucket {BUCKET_NAME}...")

            # Upload the string content directly to S3
            s3.put_object(
                Bucket=BUCKET_NAME,
                Key=file_name,
                Body=part_content
            )
            uploaded_files.append(file_name)

        # 6. Return the list of uploaded files as a JSON response
        return jsonify({
            'message': 'Success',
            'bucket': BUCKET_NAME,
            'files': uploaded_files
        })

    except Exception as e:
        print(f"Error: {e}")
        return jsonify({'error': str(e)}), 500

# Start the Web Server on port 8080 (matching the Task Definition)
if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8080)