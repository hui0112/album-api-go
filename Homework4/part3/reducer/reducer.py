from flask import Flask, request, jsonify
import boto3
import json
import os

app = Flask(__name__)

# Initialize S3 client (LabRole handles permissions automatically)
s3 = boto3.client('s3')

@app.route('/reduce', methods=['POST'])
def reduce_words():
    """
    This endpoint receives a list of intermediate result files from S3.
    It aggregates the word counts from all files into a single total.
    Expected JSON payload:
    {
        "bucket": "my-bucket",
        "files": ["split_1_result.json", "split_2_result.json", "split_3_result.json"]
    }
    """
    try:
        # 1. Parse the input JSON
        data = request.get_json()
        if not data or 'bucket' not in data or 'files' not in data:
            return jsonify({'error': 'Please provide "bucket" and a list of "files"'}), 400

        bucket_name = data['bucket']
        input_files = data['files']

        print(f"Starting reduction on {len(input_files)} files from bucket: {bucket_name}")

        # Dictionary to hold the final aggregated counts
        total_counts = {}

        # 2. Loop through each intermediate file
        for file_key in input_files:
            print(f"Processing intermediate file: {file_key}")

            # Download the JSON file from S3
            response = s3.get_object(Bucket=bucket_name, Key=file_key)
            content = response['Body'].read().decode('utf-8')

            # Parse JSON content into a dictionary
            # Example: {"hello": 5, "world": 2}
            file_counts = json.loads(content)

            # 3. Aggregate Logic: Sum up the counts
            for word, count in file_counts.items():
                if word in total_counts:
                    total_counts[word] += count
                else:
                    total_counts[word] = count

        print(f"Reduction complete. Total unique words: {len(total_counts)}")

        # 4. Sort the results (Optional, but makes the output nicer)
        # Sort by count (descending), then by word (alphabetical)
        sorted_counts = dict(sorted(total_counts.items(), key=lambda item: item[1], reverse=True))

        # 5. Upload the final result to S3
        output_key = "final_result.json"
        print(f"Uploading final result to {output_key}...")

        s3.put_object(
            Bucket=bucket_name,
            Key=output_key,
            Body=json.dumps(sorted_counts, indent=2), # indent makes it readable for humans
            ContentType='application/json'
        )

        # 6. Return success message
        return jsonify({
            'message': 'Success',
            'bucket': bucket_name,
            'key': output_key,
            'total_unique_words': len(total_counts)
        })

    except Exception as e:
        print(f"Error: {e}")
        return jsonify({'error': str(e)}), 500

if __name__ == '__main__':
    # Start the server on port 8080
    app.run(host='0.0.0.0', port=8080)