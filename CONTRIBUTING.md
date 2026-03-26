# Contributing to ChatApp

Thank you for your interest in contributing to ChatApp! This document provides guidelines and information for contributors.

## 🚀 Getting Started

### Prerequisites

Before contributing, make sure you have the following installed:

- **Go 1.21+**: Programming language
- **Docker 20.10+**: Containerization
- **Kubernetes 1.25+**: Container orchestration
- **Helm 3.10+**: Kubernetes package manager
- **kubectl**: Kubernetes CLI
- **Make**: Build automation

### Development Setup

1. **Fork the repository**
   ```bash
   # Fork on GitHub and clone your fork
   git clone https://github.com/YOUR_USERNAME/chatapp.git
   cd chatapp
   ```

2. **Set up upstream remote**
   ```bash
   git remote add upstream https://github.com/chatapp/chatapp.git
   ```

3. **Install dependencies**
   ```bash
   go mod download
   ```

4. **Start local development environment**
   ```bash
   # Start infrastructure
   docker-compose up -d

   # Build services
   make build

   # Run tests
   make test
   ```

## 📋 Development Workflow

### 1. Create a Branch

```bash
# Sync with upstream
git fetch upstream
git checkout main
git merge upstream/main

# Create feature branch
git checkout -b feature/your-feature-name
```

### 2. Make Changes

- Follow the existing code style and patterns
- Add tests for new functionality
- Update documentation as needed
- Ensure all tests pass

### 3. Test Your Changes

```bash
# Run unit tests
make test

# Run integration tests
make test-integration

# Run linting
make lint

# Run security scan
make security

# Run benchmarks
make benchmark
```

### 4. Commit Your Changes

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
feat: add new feature
fix: resolve bug in gateway
docs: update API documentation
test: add unit tests for presence service
refactor: optimize message processing
perf: improve connection handling
```

### 5. Submit Pull Request

1. Push your branch to your fork
2. Create a pull request against the `main` branch
3. Fill out the PR template
4. Wait for code review

## 🏗️ Code Style Guidelines

### Go Code Style

We use the standard Go formatting and additional tools:

```bash
# Format code
go fmt ./...

# Run linter
golangci-lint run

# Run security scan
gosec ./...
```

### Naming Conventions

- **Packages**: Lowercase, short, descriptive
- **Variables**: camelCase for local variables, PascalCase for exported
- **Functions**: camelCase, descriptive names
- **Constants**: UPPER_SNAKE_CASE
- **Files**: lowercase with underscores

### Code Organization

```
package/
├── types.go          # Type definitions
├── interface.go      # Interface definitions
├── implementation.go # Main implementation
├── test.go          # Tests
└── errors.go        # Error definitions
```

### Documentation

- **Public functions**: Must have godoc comments
- **Complex logic**: Add inline comments
- **Configuration**: Document all options
- **API endpoints**: Update OpenAPI specs

## 🧪 Testing Guidelines

### Test Structure

```go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name     string
        input    InputType
        expected ExpectedType
        wantErr  bool
    }{
        {
            name:     "valid input",
            input:    validInput,
            expected: expectedOutput,
            wantErr:  false,
        },
        {
            name:     "invalid input",
            input:    invalidInput,
            expected: nil,
            wantErr:  true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := FunctionName(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("FunctionName() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(result, tt.expected) {
                t.Errorf("FunctionName() = %v, want %v", result, tt.expected)
            }
        })
    }
}
```

### Test Categories

1. **Unit Tests**: Test individual functions and methods
2. **Integration Tests**: Test component interactions
3. **End-to-End Tests**: Test complete workflows
4. **Performance Tests**: Benchmark critical paths

### Coverage Requirements

- **New code**: Minimum 80% test coverage
- **Critical paths**: 95%+ coverage
- **Public APIs**: 100% coverage

## 📝 Documentation Guidelines

### README Updates

When adding new features:
- Update the feature list
- Add configuration examples
- Include usage examples
- Update performance metrics

### API Documentation

- Update OpenAPI specifications
- Add request/response examples
- Document error codes
- Include authentication requirements

### Code Documentation

```go
// UserService handles user-related operations
type UserService struct {
    db     Database
    cache  Cache
    logger Logger
}

// CreateUser creates a new user with the given parameters.
// It returns the created user ID or an error if the operation fails.
//
// Parameters:
//   - ctx: Context for the request
//   - req: CreateUserRequest containing user details
//
// Returns:
//   - string: Created user ID
//   - error: Error if operation fails
func (s *UserService) CreateUser(ctx context.Context, req *CreateUserRequest) (string, error) {
    // Implementation
}
```

## 🔒 Security Guidelines

### Security Best Practices

1. **Input Validation**: Always validate user input
2. **SQL Injection**: Use parameterized queries
3. **XSS Prevention**: Sanitize output
4. **Authentication**: Use strong authentication mechanisms
5. **Authorization**: Implement proper access controls
6. **Encryption**: Encrypt sensitive data at rest and in transit

### Security Testing

```bash
# Run security scan
make security

# Check for vulnerabilities
go list -json -m all | nancy sleuth

# Run dependency check
go mod verify
```

## 🚀 Performance Guidelines

### Performance Requirements

- **WebSocket Gateway**: 200K connections per pod
- **Message Latency**: P99 < 100ms
- **API Response Time**: P95 < 200ms
- **Memory Usage**: < 8GB per pod
- **CPU Usage**: < 80% average

### Optimization Techniques

1. **Connection Pooling**: Reuse database and Redis connections
2. **Batching**: Group operations for efficiency
3. **Caching**: Cache frequently accessed data
4. **Async Processing**: Use goroutines for concurrent operations
5. **Memory Management**: Use object pools and avoid allocations

### Performance Testing

```bash
# Run benchmarks
go test -bench=. -benchmem ./...

# Run load tests
./scripts/run_benchmarks.sh

# Profile memory
go tool pprof http://localhost:8080/debug/pprof/heap
```

## 🐛 Bug Reports

### Bug Report Template

```markdown
## Bug Description
Brief description of the bug

## Steps to Reproduce
1. Go to...
2. Click on...
3. See error

## Expected Behavior
What you expected to happen

## Actual Behavior
What actually happened

## Environment
- OS: [e.g., Linux, macOS]
- Go version: [e.g., 1.21.0]
- Browser: [e.g., Chrome, Firefox]

## Additional Context
Add any other context about the problem
```

## ✨ Feature Requests

### Feature Request Template

```markdown
## Feature Description
Brief description of the feature

## Problem Statement
What problem does this solve?

## Proposed Solution
How do you propose to solve it?

## Alternatives Considered
What other approaches did you consider?

## Additional Context
Add any other context or screenshots
```

## 📋 Review Process

### Code Review Checklist

- [ ] Code follows style guidelines
- [ ] Tests are included and passing
- [ ] Documentation is updated
- [ ] Security considerations addressed
- [ ] Performance impact considered
- [ ] Error handling is appropriate
- [ ] Logging is adequate
- [ ] Configuration is documented

### Review Guidelines

1. **Be constructive**: Provide helpful feedback
2. **Be specific**: Point out exact issues
3. **Be respectful**: Maintain professional tone
4. **Be thorough**: Review all aspects of the change

## 🏷️ Release Process

### Version Management

We use [Semantic Versioning](https://semver.org/):

- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes (backward compatible)

### Release Checklist

- [ ] All tests pass
- [ ] Documentation is updated
- [ ] CHANGELOG is updated
- [ ] Version is bumped
- [ ] Tag is created
- [ ] Release notes are written

## 🤝 Community Guidelines

### Code of Conduct

1. **Be respectful**: Treat everyone with respect
2. **Be inclusive**: Welcome all contributors
3. **Be helpful**: Assist others when possible
4. **Be patient**: Understand that everyone learns at different paces

### Getting Help

- **GitHub Issues**: For bug reports and feature requests
- **GitHub Discussions**: For general questions
- **Discord/Slack**: For real-time chat (if available)
- **Email**: team@chatapp.com

## 🎉 Recognition

### Contributor Recognition

- **Contributors list**: All contributors are listed in README
- **Release notes**: Contributors are mentioned in release notes
- **Special thanks**: Major contributors receive special recognition

### Ways to Contribute

1. **Code**: Write code for features and fixes
2. **Documentation**: Improve documentation
3. **Testing**: Write and improve tests
4. **Design**: Help with UI/UX design
5. **Translation**: Help with internationalization
6. **Community**: Help others in discussions

## 📞 Contact

- **Maintainers**: team@chatapp.com
- **Security Issues**: security@chatapp.com
- **General Questions**: discussions@chatapp.com

---

Thank you for contributing to ChatApp! Your contributions help make this project better for everyone. 🚀
