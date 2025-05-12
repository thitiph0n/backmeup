# BackMeUp Coding Instructions

## Project Overview

BackMeUp is a Go-based backup management tool designed to handle automated backups for various database systems (MySQL, PostgreSQL) and storage systems (MinIO). It includes scheduling, retention policies, and notifications.

## Coding Standards and Preferences

### Go Coding Style

- Follow standard Go conventions and idioms
- Use camelCase for variable names, PascalCase for exported functions/types
- Use meaningful variable and function names that clearly describe their purpose
- Include comments ONLY for public functions and complex logic
- Keep functions small and focused on a single responsibility
- Use error handling with early returns rather than deeply nested conditionals
- Include proper error context using `fmt.Errorf("context: %w", err)` pattern
- Use structured logging with appropriate log levels

### Project Structure

- `/cmd` directory contains executable applications
- `/internal` contains private application and library code
- `/docs` contains documentation files
- `/ittest` contains integration tests

### Error Handling

- Use proper error wrapping with meaningful context
- Return errors rather than logging and continuing when a function cannot complete its purpose
- Consider using custom error types for specific error cases that need special handling

### Tests

- Write unit tests for all non-trivial functions
- Integration tests should be in the `ittest` directory
- Use table-driven tests where appropriate
- Mock external dependencies for unit tests

### Configuration

- Use YAML for configuration files
- Environment variables should be available as alternatives to config file values
- Config file structure should follow established patterns in the codebase

## Dependencies

- Use standard library functions when possible
- Minimize external dependencies

## Documentation

- Document public APIs
- Include examples in documentation where helpful
- Keep README.md up to date with project changes
