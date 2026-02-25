# AI Development Sandbox — Project Template

This directory contains the minimal files needed to use the AI development sandbox from any project.

## Prerequisites

Build the base image once from the main sandbox repository:

```bash
cd /path/to/ai-development-sandbox
./dev build
```

This creates the `ai-dev-sandbox:latest` image that the template references.

## Setup

Copy `dev` and `docker-compose.yaml` into your project root:

```bash
cp template/dev template/docker-compose.yaml /path/to/your-project/
```

## Usage

Run from your project directory — cwd becomes the workspace inside the container:

```bash
cd ~/projects/myapp

# Launch Claude Code on this project
./dev claude

# Launch OpenCode
./dev opencode

# Interactive shell
./dev bash

# Mount additional context directories
./dev -c ~/shared-libs claude

# Explicit workspace override
./dev -w ~/other-project claude
```

## Customization

Edit `docker-compose.yaml` to add:
- Additional volume mounts (shared libraries, documentation, etc.)
- Extra environment variables
- Port mappings

The compose file uses `image: ai-dev-sandbox:latest` (no `build:` directive), so it depends on the pre-built image from the main sandbox repository.
