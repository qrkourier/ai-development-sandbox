# Sandbox Rules

You are running inside an AI development sandbox container.

## Environment

- Your project workspace is at `/workspace/project`
- The sandbox definition (Dockerfile, entrypoint, rules) is at `/workspace/sandbox` and is writable — you can improve the sandbox itself
- Additional context directories may be mounted under `/workspace/context/`
- You have full sudo access and all tool permissions are granted

## Session Continuity: HANDOFF.md

Sandbox sessions are ephemeral. To preserve continuity across sessions, maintain a `HANDOFF.md` file in the project root.

**At session start**: Read `HANDOFF.md` if it exists. Orient yourself before diving in.

**At session end** (or when wrapping up a significant chunk of work): Update `HANDOFF.md` with:

- **Goal**: What you were working toward
- **Done**: What was accomplished this session
- **Problems**: Blockers, bugs, or open questions encountered
- **Next**: Concrete next steps for the next session to pick up

Keep it concise — a few bullet points per section. Replace the previous session's content rather than appending indefinitely.
