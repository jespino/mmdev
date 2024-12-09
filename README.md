# MMDev - Mattermost Development Tool

MMDev is a command-line tool designed to streamline the development workflow for Mattermost. It provides a unified interface for managing both the server and client components of Mattermost during development.

![output](https://github.com/user-attachments/assets/f3b51cd0-1f24-404c-8f62-729c8f3b6bab)

## Features

- Start/stop Mattermost server with development configuration
- Manage webapp development with hot-reloading
- Automated Docker service management for dependencies
- Combined server and webapp development mode with split view
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

After installation, configure the tool:

```bash
mmdev config
```

This will guide you through setting up:
- Jira integration (URL, username, API token)
- Sentry integration (API token)

## Usage

### Start Everything (Server + Webapp)

```bash
mmdev start
```

This command starts both the server and webapp in a split view with live output from both processes. Use:
- 'tab' to switch between server/webapp panes
- 'r' to restart server (when server pane is selected)
- 's' to toggle between vertical/horizontal split (auto-scrolls to bottom)
- 'q' to quit
- ':' to enter command mode with the following commands:
  - quit: Exit the application
  - client-restart: Stop and restart the webapp process
  - server-restart: Restart the server process

### Server Commands

```bash
mmdev server start    # Start the server
mmdev server start -w # Start with file watching
mmdev server lint     # Run server code linting
mmdev server generate layers  # Generate app/store layers and plugin API
mmdev server generate mocks   # Generate mock files
mmdev server generate all     # Generate all code (layers and mocks)
```

### Webapp Commands

```bash
mmdev webapp start    # Start the webapp
mmdev webapp start -w # Start with file watching
mmdev webapp lint     # Run webapp code linting
mmdev webapp fix      # Run auto-fix on webapp code
```

### Docker Commands

```bash
mmdev docker start # Start required Docker services
mmdev docker stop  # Stop Docker services
mmdev docker clean # Remove containers and volumes

### Configuration Command

```bash
mmdev config # Configure Jira and Sentry integration
```

### Release Dates Command

```bash
mmdev dates # Show upcoming Mattermost release dates and milestones
```
```

### E2E Testing Commands

```bash
mmdev e2e playwright run     # Run Playwright E2E tests
mmdev e2e playwright ui      # Open Playwright UI
mmdev e2e playwright report  # Show Playwright test report

mmdev e2e cypress run        # Run Cypress E2E tests
mmdev e2e cypress ui         # Open Cypress UI
mmdev e2e cypress report     # Show Cypress test report
```

### AI-Assisted Development Commands

```bash
mmdev aider github owner/repo#123  # Process GitHub issue with aider
mmdev aider jira PROJECT-123       # Process Jira issue with aider
mmdev aider sentry ISSUE-ID        # Process Sentry issue with aider
```

The aider commands require:
- For GitHub: Public repository access
- For Jira: Credentials configured in ~/.mmdev.toml or environment variables:
  - JIRA_URL: Your Jira instance URL
  - JIRA_USER: Your Jira username
  - JIRA_TOKEN: Your Jira API token
- For Sentry: Environment variables or ~/.mmdev.toml configuration:
  - SENTRY_DSN: Your Sentry project DSN
  - SENTRY_TOKEN: Your Sentry authentication token

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
