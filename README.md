# 1ms.my URL Shortener

A **serverless URL shortener** built on AWS that generates both random and custom short URLs (8+ characters). The system uses Docker-based CI/CD on GitHub, pushing the Lambda container to AWS ECR.

## Architecture

1. **Docker Container**  
   - Locally builds the Golang code for the Lambda function.  
   - Pushed to **AWS ECR** via GitHub Actions on every commit to `main`.

2. **GitHub Actions**  
   - Automates building and testing.  
   - On push, it triggers a Docker build, then deploys the Lambda.  
   - Future enhancements can include security scanning and integration tests.

3. **AWS ECR**  
   - Stores the Docker image for the Lambda function.

4. **AWS Lambda (Golang)**  
   - Core logic for creating and resolving short URLs.  
   - Interacts with DynamoDB for storing/retrieving URL mappings.

5. **AWS API Gateway**  
   - Routes requests to the Lambda function.  
   - Manages the custom domain (`1ms.my`) and CORS.

6. **Amazon DynamoDB**  
   - Key-value store for `Code` → `LongURL`.  
   - Tracks creation timestamps as well.

7. **Amazon S3 + CloudFront**  
   - Hosts the static website (frontend).  
   - CloudFront provides caching and HTTPS access.

## Endpoints & API

1. **`POST /shorten`**  
   - **Request Body**: `{"url": "https://example.com"}`
   - **Response**: `{"short_url": "https://1ms.my/abc"}`

2. **`POST /createcustom`**  
   - **Request Body**:
     ```json
     {
       "url": "https://example.com",
       "code": "mycustomcode" // 8+ chars
     }
     ```
   - **Response**
     - **200 OK** → `{"short_url": "https://1ms.my/mycustomcode"}`
     - **409 Conflict** → `{"error": "Custom short code already taken"}`

3. **`GET /{code}`**  
   - **Redirects** to the original URL.  
   - **Response**  
     - **301 Redirect** → to `LongURL`
     - **404 Not Found** → if code doesn’t exist

## Future Plans

1. **Link Statistics & Analytics**  
   - Track visits, unique visitors, creation date, etc.  
   - Possibly store info in DynamoDB or use Athena.

2. **Automated Testing**  
   - Integration tests with GitHub Actions or Jenkins.  
   - Load testing with Locust or Artillery.

3. **Security Checks**  
   - Use Snyk or Trivy to scan Docker images for vulnerabilities.  
   - Enforce best practices for IAM policies.

4. **Monitoring & Logging**  
   - AWS CloudWatch alarms for 4xx/5xx error spikes.  
   - Structured logs for easier debugging.

5. **Jenkins Pipeline**  
   - Alternative or addition to GitHub Actions for more complex pipelines.  

