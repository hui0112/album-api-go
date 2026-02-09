from flask import Flask, request, jsonify
import boto3
import os
import json
import re

app = Flask(__name__)

# Initialize S3 client (uses LabRole automatically on Fargate)
s3 = boto3.client('s3')

@app.route('/map', methods=['POST'])
def map_words():
    """
    This endpoint receives the location of a split file on S3.
    It downloads the file, counts the words, and uploads the result as a JSON file.
    Expected JSON payload: {"bucket": "my-bucket", "key": "split_1.txt"}
    """
    try:
        # 1. Parse the input JSON to get bucket and file name
        data = request.get_json()
        if not data or 'bucket' not in data or 'key' not in data:
            return jsonify({'error': 'Please provide "bucket" and "key"'}), 400

        bucket_name = data['bucket']
        input_key = data['key'] # e.g., "split_1.txt"

        print(f"Processing file: {input_key} from bucket: {bucket_name}")

        # 2. Download the split file from S3
        # We use boto3 to get the object directly from the bucket
        response = s3.get_object(Bucket=bucket_name, Key=input_key)
        content = response['Body'].read().decode('utf-8')

        # 3. Core Logic: Word Count
        # We use regex to find words (ignoring punctuation) and convert to lowercase
        words = re.findall(r'[a-zA-Z]+', content.lower())

        word_counts = {}
        for word in words:
            if word in word_counts:
                word_counts[word] += 1
            else:
                word_counts[word] = 1

        print(f"Counted {len(word_counts)} unique words.")

        # 4. Prepare the result filename
        # If input is "split_1.txt", output will be "split_1_result.json"
        output_key = input_key.replace('.txt', '_result.json')

        # 5. Upload the result (JSON) back to S3
        print(f"Uploading result to {output_key}...")
        s3.put_object(
            Bucket=bucket_name,
            Key=output_key,
            Body=json.dumps(word_counts), # Convert dictionary to JSON string
            ContentType='application/json'
        )

        # 6. Return the location of the result
        return jsonify({
            'message': 'Success',
            'bucket': bucket_name,
            'key': output_key
        })

    except Exception as e:
        print(f"Error: {e}")
        return jsonify({'error': str(e)}), 500

if __name__ == '__main__':
    # Start the server on port 8080
    app.run(host='0.0.0.0', port=8080)