# Contributing to Gress

Thank you for your interest in contributing to Gress! This document provides guidelines and instructions for contributing.

## 🌟 Ways to Contribute

- **Code**: Implement features, fix bugs, improve performance
- **Documentation**: Write tutorials, improve docs, create examples
- **Testing**: Write tests, report bugs, benchmark performance
- **Design**: Create diagrams, improve UI/UX, design logos
- **Community**: Answer questions, help users, spread the word

---

## 🚀 Getting Started

### 1. Fork and Clone

```bash
# Fork on GitHub, then clone your fork
git clone https://github.com/YOUR_USERNAME/gress.git
cd gress

# Add upstream remote
git remote add upstream https://github.com/therealutkarshpriyadarshi/gress.git
```

### 2. Set Up Development Environment

```bash
# Install Go 1.21+
# Install Docker & Docker Compose

# Install dependencies
go mod download

# Run tests to verify setup
make test
```

### 3. Create a Branch

```bash
# Update main
git checkout main
git pull upstream main

# Create feature branch
git checkout -b feature/your-feature-name
# or
git checkout -b fix/issue-123
```

---

## 📝 Development Workflow

### Code Style

We follow standard Go conventions:

```bash
# Format code
go fmt ./...
make fmt

# Run linter (install golangci-lint first)
make lint

# Or use gofmt
gofmt -s -w .
```

### Guidelines

1. **Code Quality**
   - Write clear, self-documenting code
   - Add comments for complex logic
   - Follow Go best practices
   - Keep functions focused and small

2. **Testing**
   - Write unit tests for new features
   - Maintain or improve test coverage
   - Include benchmarks for performance-critical code
   - Test edge cases

3. **Documentation**
   - Update README if needed
   - Add godoc comments for public APIs
   - Include examples in documentation
   - Update ARCHITECTURE.md for design changes

4. **Commits**
   - Write clear commit messages
   - One logical change per commit
   - Reference issue numbers (#123)

### Commit Message Format

```
<type>: <subject>

<body>

<footer>
```

**Types**:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation
- `style`: Formatting, missing semicolons, etc.
- `refactor`: Code refactoring
- `test`: Adding tests
- `chore`: Maintenance tasks

**Example**:
```
feat: add RocksDB state backend

Implement persistent state storage using RocksDB to support
state larger than available RAM. Includes incremental snapshots
and state recovery.

Closes #42
```

---

## 🧪 Testing

### Running Tests

```bash
# Run all tests
make test

# Run specific package
go test ./pkg/stream/...

# Run with coverage
make test-coverage

# Run benchmarks
make bench
```

### Writing Tests

```go
func TestMapOperator(t *testing.T) {
    // Arrange
    op := NewMapOperator(func(e *Event) (*Event, error) {
        e.Value = e.Value.(int) * 2
        return e, nil
    })

    // Act
    result, err := op.Process(ctx, &Event{Value: 5})

    // Assert
    assert.NoError(t, err)
    assert.Equal(t, 10, result[0].Value)
}

func BenchmarkMapOperator(b *testing.B) {
    op := NewMapOperator(doubleValue)
    event := &Event{Value: 5}

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        op.Process(ctx, event)
    }
}
```

---

## 📦 Submitting Changes

### 1. Push Your Changes

```bash
# Run tests and linting
make test
make lint

# Commit your changes
git add .
git commit -m "feat: add awesome feature"

# Push to your fork
git push origin feature/your-feature-name
```

### 2. Create Pull Request

1. Go to your fork on GitHub
2. Click "Pull Request"
3. Fill out the PR template
4. Link related issues
5. Wait for review

### PR Checklist

- [ ] Tests pass locally (`make test`)
- [ ] Code is formatted (`make fmt`)
- [ ] New tests added for new features
- [ ] Documentation updated
- [ ] CHANGELOG.md updated (for significant changes)
- [ ] No merge conflicts
- [ ] PR description is clear

### PR Template

```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
How has this been tested?

## Checklist
- [ ] Tests pass
- [ ] Documentation updated
- [ ] No breaking changes (or documented)

## Related Issues
Closes #123
```

---

## 🐛 Reporting Bugs

### Before Reporting

1. Check existing issues
2. Verify it's reproducible
3. Test on latest version
4. Gather relevant information

### Bug Report Template

```markdown
## Bug Description
Clear description of the bug

## Steps to Reproduce
1. Step one
2. Step two
3. See error

## Expected Behavior
What should happen

## Actual Behavior
What actually happens

## Environment
- Gress version: 1.0.0
- Go version: 1.21.0
- OS: Ubuntu 22.04
- Deployment: Docker Compose

## Additional Context
Logs, screenshots, etc.
```

---

## 💡 Proposing Features

### Feature Request Template

```markdown
## Feature Description
Clear description of the feature

## Use Case
Why is this needed? Who benefits?

## Proposed Solution
How should it work?

## Alternatives Considered
Other approaches you've thought about

## Additional Context
Examples, mockups, references
```

### Discussion Process

1. Open an issue with feature request
2. Discuss with maintainers
3. Get approval before starting work
4. Create design doc for large features
5. Implement and submit PR

---

## 📚 Documentation

### Types of Documentation

1. **Code Documentation** (godoc)
   ```go
   // NewEngine creates a new stream processing engine with the given configuration.
   // The logger parameter is optional; if nil, a default logger will be created.
   //
   // Example:
   //   config := stream.DefaultEngineConfig()
   //   engine := stream.NewEngine(config, logger)
   func NewEngine(config EngineConfig, logger *zap.Logger) *Engine {
   ```

2. **README Updates**
   - Keep README.md current
   - Add examples for new features
   - Update configuration docs

3. **Tutorials**
   - Step-by-step guides
   - Real-world examples
   - Best practices

4. **Architecture Docs**
   - Update ARCHITECTURE.md for design changes
   - Include diagrams
   - Explain trade-offs

---

## 🏗️ Project Structure

```
gress/
├── cmd/               # Main applications
│   └── gress/        # Primary binary
├── pkg/              # Public libraries
│   ├── stream/       # Core engine
│   ├── ingestion/    # Data sources
│   ├── sink/         # Data outputs
│   ├── window/       # Windowing
│   ├── watermark/    # Watermarks
│   └── checkpoint/   # Checkpointing
├── internal/         # Private libraries
├── examples/         # Example applications
├── deployments/      # Deployment configs
│   ├── docker/       # Docker Compose
│   └── kubernetes/   # K8s manifests
└── docs/             # Additional documentation
```

### Package Guidelines

- **cmd/**: Entry points only, minimal logic
- **pkg/**: Reusable, well-documented packages
- **internal/**: Private implementation details
- **examples/**: Complete, runnable examples

---

## 🤝 Community Guidelines

### Code of Conduct

We follow the [Contributor Covenant](https://www.contributor-covenant.org/):

- Be respectful and inclusive
- Welcome newcomers
- Accept constructive criticism
- Focus on what's best for the community
- Show empathy

### Communication Channels

- **GitHub Issues**: Bug reports, feature requests
- **GitHub Discussions**: Questions, ideas, showcase
- **Pull Requests**: Code reviews, discussions
- **Email**: [your-email@example.com] for private matters

### Review Process

1. **Automated Checks**
   - CI/CD runs tests
   - Linting checks
   - Coverage reports

2. **Human Review**
   - Code quality review
   - Design feedback
   - Performance considerations

3. **Merge**
   - Requires 1+ approvals
   - All checks must pass
   - Squash and merge

---

## 🎯 Good First Issues

Looking for where to start? Check issues labeled:
- `good first issue`
- `help wanted`
- `documentation`

### Suggested Starter Tasks

1. **Add Examples**
   - Create tutorials
   - Write blog posts
   - Record videos

2. **Improve Tests**
   - Add edge case tests
   - Improve coverage
   - Write benchmarks

3. **Documentation**
   - Fix typos
   - Improve clarity
   - Add diagrams

4. **Small Features**
   - Add new window types
   - Implement additional sinks
   - Create utility functions

---

## 🚀 Advanced Contributions

### Large Features

For major features:

1. **Proposal**: Open issue with detailed design
2. **Discussion**: Get feedback from maintainers
3. **Design Doc**: Write RFC-style document
4. **Implementation**: Break into smaller PRs
5. **Review**: Iterate based on feedback
6. **Merge**: Celebrate! 🎉

### Performance Optimization

1. Benchmark current performance
2. Profile to identify bottlenecks
3. Implement optimization
4. Benchmark improvement
5. Document trade-offs

### Breaking Changes

Breaking changes require:
- Strong justification
- Migration guide
- Deprecation period
- Major version bump

---

## 🔍 Code Review Guidelines

### For Authors

- Respond to feedback promptly
- Ask questions if unclear
- Make requested changes
- Keep PR scope focused

### For Reviewers

- Be constructive and kind
- Explain reasoning
- Suggest alternatives
- Approve when satisfied

---

## 📄 License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.

---

## 🙏 Recognition

Contributors are recognized in:
- CONTRIBUTORS.md file
- Release notes
- Project documentation

Thank you for making Gress better! 🚀

---

## Questions?

- 📖 Read the [Documentation](README.md)
- 💬 Ask in [Discussions](https://github.com/therealutkarshpriyadarshi/gress/discussions)
- 📧 Email: your-email@example.com
