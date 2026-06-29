# Project Governance: STACKIT Professional Service

This document defines the management, ownership, and maintenance processes for the STACKIT Professional Service repository.

## 1. Strategy & "The Story"

This repository serves as a bridge between internal excellence and public visibility.

- **Internal Git (Source of Truth):** The primary repository is hosted on our internal STACKIT Git instance. All internal communication, documentation, and chat links MUST point to the internal instance to promote our own infrastructure and tools.
- **GitHub (Public Mirror):** The GitHub repository is a mirror intended for external visibility, SEO, and accessibility for AI models (LLMs). It helps customers find our solutions and establishes STACKIT as a thought leader in cloud automation.

## 2. Ownership

### 2.1 Organizational Ownership

The repository is owned by the **STACKIT Professional Services** organization. High-level decisions regarding repository structure, licensing, and global policies are managed by the Core Maintainers team.

### 2.2 Example & Module Ownership

Individual examples or modules within the repository have specific owners, documented in their respective `MAINTAINERS.md` files.

- **Responsibility:** Owners are responsible for the technical health, periodic updates (e.g., dependency bumps), and community feedback for their specific content.
- **Handover:** If an owner leaves the project or company, ownership reverts to the Core Maintainers until a new owner is assigned.

## 3. Review & Quality Assurance

To ensure high standards and security, we follow a strict contribution process:

- **4-Eyes Principle:** No code enters the `main` branch without at least one successful Peer Review.
- **Automated Validation:** Every Pull Request must pass the CI pipeline, which includes:
  - Linting and formatting checks.
  - License header verification (Apache 2.0).
  - Secret scanning (GitLeaks).
- **Best Effort Policy:** While we strive for quality, the content is provided "as-is." Use in production environments requires independent validation by the user.

## 4. Mirroring Process

The synchronization between the internal Git and GitHub is fully automated:

1.  Changes are merged into the internal `main` branch.
2.  A GitHub Action triggers on every push to `main`.
