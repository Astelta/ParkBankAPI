# ParkBankApi

A comprehensive Go-based REST API with advanced monitoring and metrics capabilities.

## Table of Contents

- [Features](#features)
- [API Endpoints](#api-endpoints)
- [Metrics Dashboard](#metrics-dashboard)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
- [Development](#development)

## Features

- PostgreSQL Integration: Robust database connectivity with connection pooling
- Health Monitoring: Multiple health check endpoints for different monitoring levels
- Comprehensive Metrics: Detailed system and database performance metrics
- CORS Support: Cross-origin resource sharing enabled for web applications
- Environment Configuration: Flexible configuration via environment variables or `.env` file
- SSL/TLS Support: Secure database connections with certificate validation

## API Endpoints

All of the endpoints are described in detail [https://astelta.github.io/ParkBankAPI/readme.html](here)

## Metrics Dashboard

The project includes an interactive HTML dashboard (`readme.html`) that provides:

- Real-time Metrics Display: Live updating dashboard with key metrics
- Endpoint Testing: Direct access to all API endpoints
- Response Visualization: Formatted JSON responses with syntax highlighting
- Performance Monitoring: Request latency measurements
- Memory Analysis: Detailed memory usage breakdown

To use the dashboard:
1. Start the API server
2. Open `readme.html` in a web browser
3. The dashboard will automatically fetch and display metrics

## Installation

### Prerequisites
- Go 1.19 or later
- PostgreSQL database (local or remote)

### Build
```bash
go mod tidy
go build -o astelta-api-v2 main.go
```

### Run
```bash
./astelta-api-v2
```

## Configuration

Configure the API using environment variables or a `.env` file:

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | Full PostgreSQL connection string | - |
| `DB_HOST` | Database host | - |
| `DB_PORT` | Database port | - |
| `DB_USER` | Database username | - |
| `DB_PASSWORD` | Database password | - |
| `DB_NAME` | Database name | - |
| `CA_CERT_PATH` | Path to CA certificate for SSL | - |

### Connection Examples

**Using DATABASE_URL:**
```bash
DATABASE_URL="postgres://user:password@host:port/database?sslmode=require"
```

**Using individual variables:**
```bash
DB_HOST=localhost
DB_PORT=5432
DB_USER=myuser
DB_PASSWORD=mypassword
DB_NAME=mydatabase
```

## Usage

### Starting the Server
```bash
# With environment variables
export DB_HOST=your-host
export DB_USER=your-user
export DB_PASSWORD=your-password
./astelta-api-v2

# Or with .env file
./astelta-api-v2
```

### Testing Endpoints
```bash
# Health check
curl http://localhost:8097/health

# Metrics
curl http://localhost:8097/metrics

# Database status
curl http://localhost:8097/db-status
```

## Development

### Project Structure
```
.
├── main.go           # Main application file
├── readme.html       # Interactive metrics dashboard
├── .gitignore       # Git ignore rules
├── go.mod           # Go module file
├── go.sum           # Go dependencies
└── .env             # Environment configuration (ignored)
```

### Key Components

- Database Connection: PostgreSQL connection with SSL support
- HTTP Handlers: RESTful endpoints with CORS middleware
- Metrics Collection: Runtime statistics and database monitoring
- Configuration Management: Environment variable parsing
- Error Handling: Comprehensive error responses

### Adding New Endpoints

1. Create a handler function:
```go
func myHandler(w http.ResponseWriter, r *http.Request) {
    // Handler logic
}
```

2. Register the endpoint in `main()`:
```go
http.HandleFunc("/my-endpoint", corsMiddleware(myHandler))
```

## Monitoring & Observability

The API provides multiple levels of monitoring:

- Health Checks: Basic service availability
- Metrics: Detailed performance and resource usage data
- Database Monitoring: Connection health and query performance
- Memory Profiling: Go runtime memory statistics

Use these endpoints with monitoring tools like Prometheus, Grafana, or custom dashboards.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## License

This project is licensed under the GPL-3.0 License - see the LICENSE file for details.

---

Built with Go and PostgreSQL
