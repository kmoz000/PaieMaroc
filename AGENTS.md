# Repository Guidelines

## Project Structure & Module Organization

This repository is currently a blank scaffold. Keep the root limited to project-wide configuration and documentation. Place application code in `src/`, tests in `tests/` (or next to source files when the framework favors co-location), static files in `assets/` or `public/`, and maintenance scripts in `scripts/`.

Organize `src/` by feature or domain rather than by generic file type. Keep modules focused, expose deliberate public interfaces, and avoid circular dependencies. Update this guide when the initial language, framework, and directory layout are established.

## Build, Test, and Development Commands

No build system or package manifest exists yet. The first implementation should provide a documented command surface through a `Makefile` or the ecosystem's standard task runner. Use predictable targets such as:

- `make dev` — start the local development environment.
- `make test` — run the complete automated test suite.
- `make lint` — run formatting and static-analysis checks.
- `make build` — produce a reproducible release artifact.

Do not commit commands to this guide until they work from a clean checkout.

## Coding Style & Naming Conventions

Adopt the official formatter for the selected language and run it before committing. Use the ecosystem's conventional indentation and file naming. Prefer descriptive names, small functions, and explicit error handling. Name tests after the behavior under test—for example, `user-login.test.ts` or `test_user_login.py`. Add formatter and linter configuration to the repository so local and CI results match.

## Testing Guidelines

Every behavior change should include an automated test. Cover successful paths, validation failures, and important edge cases. Keep fixtures deterministic; do not depend on external services unless the test is marked integration-only. Once a framework is chosen, document its exact test and coverage commands here and enforce them in CI.

## Commit & Pull Request Guidelines

There is no Git history from which to infer an established convention. Until one is adopted, use short, imperative commit subjects such as `Add initial API scaffold`, with one logical change per commit. Pull requests should explain the purpose, summarize notable decisions, list verification performed, and link relevant issues. Include screenshots or recordings for visible UI changes and call out configuration changes, migrations, or follow-up work.

## Security & Configuration

Never commit credentials, tokens, or local environment files. Provide sanitized examples such as `.env.example`, validate configuration at startup, and document required variables without including real values.
