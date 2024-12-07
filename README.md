# MMDev - Mattermost Development Tool

MMDev is a command-line tool designed to streamline the development workflow for Mattermost. It provides a unified interface for managing both the server and client components of Mattermost during development.

## Features

- Start/stop Mattermost server with development configuration
- Manage client development with hot-reloading
- Automated Docker service management for dependencies
- Combined server and client development mode with split view
- Code linting for both server and client
- File watching and auto-restart capabilities
- E2E testing support with Playwright and Cypress
- Automatic Mattermost repository detection and navigation

## Prerequisites

- Go 1.21 or later
- Docker
- Node.js and npm (for client development)
- PostgreSQL client tools (for health checks)
- NVM (Node Version Manager) configured in ~/.nvm

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
- 'tab' to switch between server/client panes
- 'r' to restart server (when server pane is selected)
- 'q' to quit
- ':' to enter command mode

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
mmdev client fix      # Run auto-fix on client code
```

### Docker Commands

```bash
mmdev docker start # Start required Docker services
mmdev docker stop  # Stop Docker services
mmdev docker clean # Remove containers and volumes
```

### E2E Testing Commands

```bash
mmdev e2e playwright run     # Run Playwright E2E tests
mmdev e2e playwright ui      # Open Playwright UI
mmdev e2e playwright report  # Show Playwright test report
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
