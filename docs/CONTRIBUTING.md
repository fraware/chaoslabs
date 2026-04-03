# Contributing to ChaosLabs

Documentation index: [docs/README.md](README.md). Repository overview: [README.md](../README.md).

Thank you for your interest in contributing to ChaosLabs! Your contributions help improve the project and enhance its value for the community. Please review the guidelines below before contributing.

## How to Contribute

### Reporting Issues

- **Before opening an issue:**  
  - Check the [FAQ](../README.md#faq), [Troubleshooting Guide](TROUBLESHOOTING.md), and [documentation index](README.md) for similar issues.
- **When reporting a bug:**  
  - Use a descriptive title.
  - Provide clear steps to reproduce the issue.
  - Include expected vs. actual behavior.
  - Attach logs or error messages if available.

### Submitting Pull Requests

- **Fork the Repository:**
Create a branch for your feature or bug fix:
  ```bash
  git checkout -b feature/your-feature-name
  ```
  - **Commit Your Changes:**
Write clear, concise commit messages.

  - **Push and Open a PR:**
Push your branch to your fork and open a pull request against the `main` branch.

  - **Code Style:**
Follow idiomatic Go style (`gofmt`, `golangci-lint`) and TypeScript/React conventions in `dashboard-v2` (`npm run lint`, `npm run type-check`).
Run `make verify` before opening a PR when possible.
Ensure your code passes all tests.

  - **Testing:**
Add or update tests as necessary.
Verify your changes using the CI/CD pipeline.

### Documentation contributions
Update the [root README](../README.md), [docs/README.md](README.md), and focused guides under `docs/`. When you change HTTP handlers, update [api/openapi.yaml](api/openapi.yaml). Keep [ARCHITECTURE.md](ARCHITECTURE.md) in sync for new components or data flows.

## Community & Communication
  - **Issues & Discussions:**
Use GitHub Issues to report bugs, request features, or ask questions.

  - **Pull Requests:**
Reference relevant issues (e.g., "Closes #123") in your PR descriptions.

  - **Feedback:**
Join our GitHub Discussions for collaborative problem solving.

  - **Contact:**
For major contributions or questions, reach out to maintainers via GitHub or our community chat (e.g., Slack/Discord).
Thank you for contributing to ChaosLabs!
