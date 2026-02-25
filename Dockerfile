FROM ubuntu:22.04

# AI Development Sandbox
# This Docker environment provides a safe sandbox for AI-assisted development
# with Claude Code and OpenCode, isolated from the host system.

# Install essential development tools including Playwright dependencies
RUN apt-get update && apt-get install -y \
    git curl build-essential python3 python3-pip unzip tree sudo jq \
    # Playwright dependencies for headless browser support
    libnss3 libatk-bridge2.0-0 libdrm2 libxkbcommon0 libxcomposite1 \
    libxdamage1 libxrandr2 libgbm1 libxss1 libasound2 \
    # Additional Playwright requirements
    libcups2 libxfixes3 libcairo2 libpango-1.0-0 \
    && rm -rf /var/lib/apt/lists/*

# Install modern Node.js (instead of old Ubuntu version)
RUN curl -fsSL https://deb.nodesource.com/setup_lts.x | bash - \
    && apt-get install -y nodejs

# Install Playwright globally as root
RUN npm install -g playwright

# Install gh CLI
RUN curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
    | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg \
    && chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
    | tee /etc/apt/sources.list.d/github-cli.list > /dev/null \
    && apt-get update && apt-get install -y gh \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user 'dev' with passwordless sudo access
# This allows installing packages inside the container when needed
RUN useradd -m -s /bin/bash dev && usermod -aG sudo dev
RUN echo 'dev ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers
USER dev
WORKDIR /workspace

# Set PATH so non-interactive shells can find locally installed tools
ENV PATH="/home/dev/.local/bin:$PATH"

# Install Playwright browsers as dev user so they're accessible
RUN npx playwright install

# Install AI coding assistants
# Claude Code: Anthropic's official CLI for AI-assisted development
RUN curl -fsSL https://claude.ai/install.sh | bash

# OpenCode: Open-source alternative with multi-provider support
RUN curl -fsSL https://opencode.ai/install | bash

# Copy sandbox config, rules, and entrypoint
COPY --chown=dev:dev config/claude-settings.json /home/dev/.claude-sandbox/settings.json
COPY --chown=dev:dev rules/ /home/dev/.claude-sandbox/rules/
COPY --chown=dev:dev entrypoint.sh /home/dev/entrypoint.sh
RUN chmod +x /home/dev/entrypoint.sh

ENTRYPOINT ["/home/dev/entrypoint.sh"]
CMD ["bash"]
