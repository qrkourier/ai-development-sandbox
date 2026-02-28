#!/bin/bash
set -euo pipefail

# --- Install permissive Claude Code settings ---
# Copy baked-in settings to ~/.claude/settings.json only if not already present
# (allows override via volume mount)
if [ ! -f "$HOME/.claude/settings.json" ]; then
    mkdir -p "$HOME/.claude"
    if [ -f "$HOME/.claude-sandbox/settings.json" ]; then
        cp "$HOME/.claude-sandbox/settings.json" "$HOME/.claude/settings.json"
    fi
fi

# --- Assemble sandbox rules ---
# Two layers: sandbox-level defaults, then project-local extensions.
# If the sandbox rules have been edited at runtime (via the /workspace/sandbox
# mount), use the runtime version instead of what was baked into the image.
RULES_FILE="$HOME/.sandbox-rules.md"
: > "$RULES_FILE"

# 1. Sandbox rules — prefer runtime edits over baked-in
if [ -f /workspace/sandbox/rules/sandbox.md ]; then
    cat /workspace/sandbox/rules/sandbox.md >> "$RULES_FILE"
elif [ -f "$HOME/.claude-sandbox/rules/sandbox.md" ]; then
    cat "$HOME/.claude-sandbox/rules/sandbox.md" >> "$RULES_FILE"
fi

# 2. Project-local rules (the project being worked on can add its own)
if [ -f /workspace/project/.sandbox-rules.md ]; then
    printf '\n---\n\n' >> "$RULES_FILE"
    cat /workspace/project/.sandbox-rules.md >> "$RULES_FILE"
fi

export SANDBOX_RULES_FILE="$RULES_FILE"

# Inject rules into Claude Code settings as customInstructions
if [ -s "$RULES_FILE" ] && command -v jq &>/dev/null; then
    rules_text=$(cat "$RULES_FILE")
    tmp=$(mktemp)
    jq --arg rules "$rules_text" '.customInstructions = $rules' \
        "$HOME/.claude/settings.json" > "$tmp" && mv "$tmp" "$HOME/.claude/settings.json"
fi

# --- Symlink credentials ---
# If Claude credentials were mounted at the staging path, symlink into ~/.claude/
if [ -f "$HOME/.claude-credentials/.credentials.json" ]; then
    mkdir -p "$HOME/.claude"
    ln -sf "$HOME/.claude-credentials/.credentials.json" "$HOME/.claude/.credentials.json"
fi

# --- Fix git config ---
# The host's ~/.gitconfig may reference host-specific paths (GPG programs,
# credential helpers). Override these to avoid errors inside the container.
git config --global --unset gpg.program 2>/dev/null || true
git config --global --unset commit.gpgsign 2>/dev/null || true
git config --global --unset tag.gpgsign 2>/dev/null || true
git config --global --unset credential.helper 2>/dev/null || true
git config --global commit.gpgsign false

# --- Handle SSH permissions ---
# Docker ro-mounts preserve host permissions which may be too open for SSH.
# Copy to a temp location with correct permissions if needed.
if [ -d "$HOME/.ssh" ] && [ ! -w "$HOME/.ssh" ]; then
    SSH_DIR=$(mktemp -d)
    cp -a "$HOME/.ssh/." "$SSH_DIR/"
    chmod 700 "$SSH_DIR"
    chmod 600 "$SSH_DIR"/* 2>/dev/null || true
    chmod 644 "$SSH_DIR"/*.pub 2>/dev/null || true
    chmod 644 "$SSH_DIR"/known_hosts 2>/dev/null || true
    chmod 644 "$SSH_DIR"/config 2>/dev/null || true
    export GIT_SSH_COMMAND="ssh -o StrictHostKeyChecking=no -i $SSH_DIR/id_ed25519 -o UserKnownHostsFile=$SSH_DIR/known_hosts"
fi

# --- HANDOFF.md convention ---
# If a HANDOFF.md exists in the project, ensure Claude reads it on startup
# by prepending instructions to the rules
if [ -f /workspace/project/HANDOFF.md ]; then
    HANDOFF_NOTICE="IMPORTANT: A HANDOFF.md file exists in the project root. Read it FIRST before doing anything else. It contains context from a previous session."
    if [ -s "$RULES_FILE" ]; then
        printf '%s\n\n' "$HANDOFF_NOTICE" | cat - "$RULES_FILE" > "$RULES_FILE.tmp" && mv "$RULES_FILE.tmp" "$RULES_FILE"
        # Re-inject into settings
        if command -v jq &>/dev/null; then
            rules_text=$(cat "$RULES_FILE")
            tmp=$(mktemp)
            jq --arg rules "$rules_text" '.customInstructions = $rules' \
                "$HOME/.claude/settings.json" > "$tmp" && mv "$tmp" "$HOME/.claude/settings.json"
        fi
    fi
fi

# --- Tool call logging ---
# When TOOL_CALL_LOG is set, Claude Code output is tee'd to a log file
# for the manager to parse significant actions
if [ -n "${TOOL_CALL_LOG:-}" ]; then
    mkdir -p "$(dirname "$TOOL_CALL_LOG")"
fi

# --- Dispatch to agent ---
case "${1:-}" in
    claude)
        shift
        if [ -n "${TOOL_CALL_LOG:-}" ]; then
            exec claude --permission-mode bypassPermissions "$@" 2>&1 | tee -a "$TOOL_CALL_LOG"
        else
            exec claude --permission-mode bypassPermissions "$@"
        fi
        ;;
    opencode)
        shift
        exec opencode "$@"
        ;;
    *)
        exec "$@"
        ;;
esac
