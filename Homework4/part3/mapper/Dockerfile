# 1. Base Image: Use an official Python runtime as a parent image
# "slim" variants are smaller and sufficient for our needs
FROM python:3.9-slim

# 2. Work Directory: Set the working directory in the container
# All following commands will be run inside this folder
WORKDIR /app

# 3. Dependencies: Copy the requirements file into the container
# We copy this first to leverage Docker cache layers (if requirements don't change, this step is skipped on rebuilds)
COPY requirements.txt .

# 4. Install: Install any needed packages specified in requirements.txt
# --no-cache-dir reduces image size by not saving the downloaded files
RUN pip install --no-cache-dir -r requirements.txt

# 5. Copy Code: Copy the rest of the application code into the container
# This copies mapper.py from your machine to the /app folder in the container
COPY mapper.py .

# 6. Port: Make port 8080 available to the world outside this container
# This is documentation for us and AWS ECS to know which port to open
EXPOSE 8080

# 7. Logging: Set environment variable to ensure output is logged immediately
# This prevents Python from buffering stdout/stderr, so you can see logs in CloudWatch in real-time
ENV PYTHONUNBUFFERED=1

# 8. Command: Run mapper.py when the container launches
# This is the command that starts your application
CMD ["python", "mapper.py"]