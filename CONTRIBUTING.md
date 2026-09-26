# Contributing to Scheduler0

Thank you for your interest in contributing to Scheduler0!

## Development Setup

1. Install Go 1.26.5 or later
2. Install etcd (required for local development)
3. Clone the repository
4. Run `CGO_ENABLED=1 go build -o scheduler0 ./`

## Running Locally

See AGENTS.md for detailed instructions on running a local node.

## Pull Requests

- Write clear commit messages
- Add tests for new features
- Update documentation as needed
- Ensure `go vet ./...` passes

## Code of Conduct

Be respectful and inclusive in all interactions.
