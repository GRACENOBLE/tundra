# Tundra E-Commerce API

A comprehensive e-commerce REST API built with Go, featuring authentication, product management, order processing, Redis caching, Cloudinary image uploads, and rate limiting.

## Features

- 🔐 **JWT Authentication** - Secure user registration and login
- 👤 **Role-Based Access Control** - Admin and user roles with middleware protection
- 📦 **Product Management** - Full CRUD operations with image upload support
- 🛒 **Order Processing** - Transaction-based order creation with inventory management
- 🚀 **Redis Caching** - Optimized product listing with intelligent cache invalidation
- 📸 **Cloudinary Integration** - Cloud-based image storage and management
- 🛡️ **Rate Limiting** - IP-based rate limiting to prevent abuse and brute force attacks
- 📚 **Swagger/OpenAPI Documentation** - Interactive API documentation
- 🧪 **Comprehensive Testing** - Integration tests with testcontainers
- 🐳 **Docker Support** - Containerized PostgreSQL and Redis services

## Getting Started

### Prerequisites

- Go 1.21 or higher
- Docker and Docker Compose
- PostgreSQL 15
- Redis 7
- Cloudinary account (for image uploads)

### Environment Variables

Create a `.env` file in the root directory with the following variables:

```env
# Server Configuration
PORT=8080

# Database Configuration
BLUEPRINT_DB_HOST=localhost
BLUEPRINT_DB_PORT=5432
BLUEPRINT_DB_DATABASE=blueprint
BLUEPRINT_DB_USERNAME=blueprint
BLUEPRINT_DB_PASSWORD=blueprint

# JWT Configuration
JWT_SECRET=your-super-secret-jwt-key-change-in-production

# Redis Configuration (Optional - graceful fallback if not available)
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=

# Cloudinary Configuration (Optional - required for image uploads)
# Format: cloudinary://API_KEY:API_SECRET@CLOUD_NAME
CLOUDINARY_URL=cloudinary://your_api_key:your_api_secret@your_cloud_name
```

### Installation & Setup

1. **Clone the repository**

```bash
git clone https://github.com/GRACENOBLE/tundra.git
cd tundra
```

2. **Install dependencies**

```bash
go mod download
```

3. **Start Docker services**

```bash
make docker-run
# or
docker-compose up -d
```

4. **Run database migrations**

```bash
go run cmd/migrate/main.go -action=up
```

5. **Build the application**

```bash
make build
```

6. **Run the application**

```bash
make run
```

The API will be available at `http://localhost:8080`
Swagger documentation at `http://localhost:8080/swagger/index.html`

## API Endpoints

### Authentication

- `POST /auth/register` - Register a new user
- `POST /auth/login` - Login and receive JWT token

### Products (Public)

- `GET /products` - List all products (supports pagination, search, caching)
- `GET /products/:id` - Get product details

### Products (Admin Only)

- `POST /products` - Create a new product (supports multipart image upload)
- `PUT /products/:id` - Update product details
- `DELETE /products/:id` - Delete a product
- `POST /products/:id/image` - Upload/update product image

### Orders (Authenticated Users)

- `POST /orders` - Create a new order
- `GET /orders` - Get user's order history

## MakeFile Commands

Run build make command with tests

```bash
make all
```

Build the application

```bash
make build
```

Run the application

```bash
make run
```

Create DB container

```bash
make docker-run
```

Shutdown DB Container

```bash
make docker-down
```

DB Integrations Test:

```bash
make itest
```

Live reload the application:

```bash
make watch
```

Run the test suite:

```bash
make test
```

Clean up binary from the last build:

```bash
make clean
```

## Cloudinary Image Upload

### Setup

1. Create a free account at [Cloudinary](https://cloudinary.com)
2. Get your credentials from the dashboard
3. Set the `CLOUDINARY_URL` environment variable in the format:
   ```
   cloudinary://API_KEY:API_SECRET@CLOUD_NAME
   ```

### Usage

#### Create Product with Image (Multipart Form Data)

```bash
curl -X POST http://localhost:8080/products \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -F "name=Laptop" \
  -F "description=High-performance laptop" \
  -F "price=999.99" \
  -F "stock=50" \
  -F "category=Electronics" \
  -F "image=@/path/to/image.jpg"
```

#### Upload/Update Product Image

```bash
curl -X POST http://localhost:8080/products/{product_id}/image \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -F "image=@/path/to/image.jpg"
```

**Supported image formats:** JPG, JPEG, PNG, GIF, WEBP

## Redis Caching

The API implements intelligent caching for product listings:

- **Cache Key Strategy**: `products:page:{page}:size:{size}:search:{query}`
- **TTL**: 5 minutes
- **Cache Invalidation**: Automatic invalidation on product create/update/delete
- **Graceful Fallback**: API continues to work if Redis is unavailable

## Rate Limiting

The API implements IP-based rate limiting to prevent abuse and protect against brute force attacks:

### Rate Limit Tiers

1. **Global Rate Limit** - Applied to all endpoints

   - **Limit**: 1000 requests per hour per IP
   - **Purpose**: Prevent general API abuse

2. **Authentication Rate Limit** - Applied to login/register endpoints

   - **Limit**: 5 requests per minute per IP
   - **Purpose**: Prevent brute force attacks on authentication
   - **Endpoints**: `/auth/register`, `/auth/login`

3. **API Rate Limit** - Applied to general API endpoints
   - **Limit**: 100 requests per minute per IP
   - **Purpose**: Prevent excessive API usage
   - **Endpoints**: `/products/*`, `/orders/*`

### Rate Limit Headers

When rate limiting is active, the API returns these headers:

- `X-RateLimit-Limit` - Maximum requests allowed in the period
- `X-RateLimit-Remaining` - Requests remaining in current period
- `X-RateLimit-Reset` - Time when the rate limit resets

### Rate Limit Response

When rate limit is exceeded, the API returns:

```json
{
  "error": "rate limit exceeded"
}
```

**Status Code**: `429 Too Many Requests`

### Implementation Details

- **Per-IP Tracking**: Rate limits are tracked per client IP address
- **In-Memory Store**: Uses fast in-memory storage for minimal latency
- **Automatic Reset**: Limits automatically reset after the time window
- **Independent Limits**: Different endpoints have independent rate limit counters

## Testing

Run all tests:

```bash
make test
```

Run integration tests:

```bash
make itest
```

The test suite includes:

- Unit tests for authentication and validation
- Integration tests with testcontainers (PostgreSQL + Redis)
- Product CRUD operation tests
- Order processing tests
- Cache hit/miss scenarios
- Cache invalidation tests
- Rate limiting tests (8 comprehensive scenarios)

## Project Structure

```
tundra/
├── cmd/
│   ├── api/           # Application entry point
│   └── migrate/       # Database migration tools
├── internal/
│   ├── auth/          # JWT authentication & middleware
│   ├── cloudinary/    # Cloudinary image upload client
│   ├── database/      # Database connection & models
│   │   └── models/    # GORM models (User, Product, Order)
│   ├── ratelimit/     # Rate limiting middleware
│   └── server/        # HTTP server & route handlers
├── migrations/        # SQL migration files
├── docs/             # Swagger/OpenAPI documentation
├── docker-compose.yml
├── Makefile
└── README.md
```

## Technologies Used

- **Framework**: [Gin](https://github.com/gin-gonic/gin) - HTTP web framework
- **Database**: [PostgreSQL](https://www.postgresql.org/) with [GORM](https://gorm.io/)
- **Caching**: [Redis](https://redis.io/) with [go-redis](https://github.com/redis/go-redis)
- **Authentication**: [JWT](https://jwt.io/) with [golang-jwt](https://github.com/golang-jwt/jwt)
- **Rate Limiting**: [ulule/limiter](https://github.com/ulule/limiter) - IP-based rate limiting
- **Image Upload**: [Cloudinary](https://cloudinary.com/) Go SDK
- **Documentation**: [Swagger/OpenAPI](https://swagger.io/) with [swaggo](https://github.com/swaggo/swag)
- **Testing**: [Testcontainers](https://golang.testcontainers.org/)
- **Containerization**: [Docker](https://www.docker.com/) & [Docker Compose](https://docs.docker.com/compose/)

## License

This project is licensed under the MIT License.

## Author

**GRACENOBLE**

- GitHub: [@GRACENOBLE](https://github.com/GRACENOBLE)
