# AI Development Sandbox

Safe Docker environment for AI-assisted development with Claude Code and OpenCode.

## Quick Start

```bash
# Build the sandbox image (one-time)
/path/to/ai-development-sandbox/dev build

# Run from your project directory — cwd becomes the workspace
cd ~/projects/myapp
/path/to/ai-development-sandbox/dev claude
```

Tip: symlink `dev` into your PATH for convenience:
```bash
ln -s /path/to/ai-development-sandbox/dev ~/bin/dev
```

Then from any project:
```bash
cd ~/projects/myapp
dev claude
```

## Usage

```
dev [OPTIONS] [COMMAND] [ARGS...]

Run from your project directory — cwd becomes the workspace inside the container.

Commands:
  claude [args...]     Launch Claude Code (permissive mode)
  opencode [args...]   Launch OpenCode
  bash                 Interactive shell (default)
  build                Build/rebuild the sandbox image

Options:
  -w, --workspace DIR       Override workspace directory (default: cwd)
  -c, --context DIR[:PATH]  Mount additional context directory (repeatable)
                             DIR = host path, PATH = container path
                             Default PATH: /workspace/context/<basename>
  -h, --help                Show help
```

## Examples

```bash
cd ~/projects/myapp && dev claude            # Claude Code on myapp
cd ~/projects/myapp && dev opencode          # OpenCode on myapp
cd ~/projects/myapp && dev bash              # shell in myapp

dev -c ~/docs -c ~/shared-libs claude        # with extra context dirs
dev -c ~/lib:/workspace/lib bash             # custom mount point
dev -w ~/projects/other claude               # explicit workspace override
```

## Architecture

### Volume Strategy

The sandbox mounts only what's needed — no host config directories are inherited wholesale:

| Mount | Container Path | Mode |
|-------|---------------|------|
| Workspace (cwd) | `/workspace/project` | rw |
| This sandbox repo | `/workspace/sandbox` | rw |
| Claude credentials | Staged, then symlinked | ro |
| OpenCode auth | Direct mount | ro |
| Git config | `/home/dev/.gitconfig` | ro |
| SSH keys | `/home/dev/.ssh` | ro |
| Context dirs (`-c`) | `/workspace/context/<name>` | ro |

### Permissive Settings

The container uses its own `settings.json` (baked into the image) that allows all tool operations. The host's `~/.claude/settings.json` is never mounted, preventing restrictive host settings from blocking sandbox operations. Claude Code is also launched with `--permission-mode bypassPermissions` for belt-and-suspenders permission bypass.

### Self-Improvement

The sandbox repo itself is mounted at `/workspace/sandbox:rw` inside every container, allowing agents to modify the Dockerfile, entrypoint, or launcher script.

### Agent Rules

The sandbox injects instructions into agents at startup via `rules/sandbox.md`. The entrypoint assembles rules from three layers (later layers extend earlier ones):

1. **Baked-in defaults** — `rules/sandbox.md` from when the image was built
2. **Sandbox-repo edits** — if `rules/sandbox.md` has been modified since the image was built (via the `/workspace/sandbox` mount), the updated version is used instead
3. **Project-local rules** — `.sandbox-rules.md` in the project root (optional)

The assembled rules are:
- Injected into Claude Code via `customInstructions` in `~/.claude/settings.json`
- Available to any agent at the path in the `SANDBOX_RULES_FILE` environment variable

The default rules include a **HANDOFF.md** convention: agents maintain a `HANDOFF.md` file in the project root to capture goals, progress, open problems, and next steps, so the next session can pick up where the last one left off.

To add project-specific rules, create `.sandbox-rules.md` in your project:

```markdown
## Project Rules

- Always run `npm test` before committing
- The API schema is defined in `docs/api.yaml` — keep it in sync
```

## Template for Other Projects

The `template/` directory contains a minimal `dev` script and `compose.yaml` that can be copied into any project to use the sandbox without the full repo. See `template/README.md` for setup instructions.

## What's Inside

- **AI Tools**: Claude Code, OpenCode
- **Dev Tools**: Git, Node.js (LTS), Python 3, build-essential, jq, gh CLI
- **Browser Automation**: Playwright with headless browsers
- **Sudo Access**: Install packages as needed with `sudo apt install`
