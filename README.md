# MMDev - Mattermost Development Tool

MMDev is a command-line tool designed to streamline the development workflow for Mattermost. It provides a unified interface for managing both the server and client components of Mattermost during development.

## Features

- Start/stop Mattermost server with development configuration
- Manage client development with hot-reloading
- Automated Docker service management for dependencies
- Combined server and client development mode with split view
- Code linting for both server and client
- File watching and auto-restart capabilities

## Prerequisites

- Go 1.21 or later
- Docker
- Node.js and npm (for client development)
- PostgreSQL client tools (for health checks)

## Installation

```bash
go install github.com/jespino/mmdev@latest
```

## Usage

### Start Everything (Server + Client)

```bash
mmdev start
```

This command starts both the server and client in a split view with live output from both processes. Use:
- 'q' or ESC to quit
- 'h' for horizontal split
- 'v' for vertical split

### Server Commands

```bash
mmdev server start    # Start the server
mmdev server start -w # Start with file watching
mmdev server lint     # Run server code linting
mmdev server generate layers  # Generate app/store layers and plugin API
mmdev server generate mocks   # Generate mock files
mmdev server generate all     # Generate all code (layers and mocks)
```

### Client Commands

```bash
mmdev client start    # Start the client
mmdev client start -w # Start with file watching
mmdev client lint     # Run client code linting
```

### Docker Commands

```bash
mmdev docker start # Start required Docker services
mmdev docker stop  # Stop Docker services
mmdev docker clean # Remove containers and volumes
```

## Docker Services

The tool manages these Docker services automatically:
- PostgreSQL (Database)
- Minio (S3-compatible storage)
- OpenLDAP (Directory service)
- Elasticsearch (Search engine)
- Inbucket (Email testing)
- Redis (Caching)

## Development

To contribute to MMDev:

1. Clone the repository
2. Install dependencies: `go mod download`
3. Make your changes
4. Run tests: `go test ./...`
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.
