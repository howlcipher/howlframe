"""Agent CLI configuration for each provider (model selection §4; Revision 3 G6, G7).

Both CLIs run with their vendor-default system prompt and agent loop, but with a fresh HOME and configuration
directory, so nothing from the host (CLAUDE.md, AGENTS.md, skills, memories, plugins, MCP servers, hooks) reaches
any arm. Tools are limited to each vendor's local file and shell tools: no web search or fetch, no MCP, no
subagents where the CLI allows removing them. The model credential inside the jail is a dummy; the proxy adds the
real key outside the jail.
"""

import json
import os

DUMMY_KEY = "cv1-dummy-credential-not-a-key"
PROXY_PORT = 18080

CLAUDE_TOOLS = "Bash,Read,Edit,Write"

# Codex 0.163.0 features turned off so every arm has the same local-only tool surface. Codex's agent-team tools
# stay offered with gpt-6.1-sol whatever the config says (catalog multi_agent_version v2); its default instructions
# tell it not to spawn sub-agents unless asked, and the proxy counts and bills any spawn to the run.
CODEX_FEATURES_OFF = [
    "multi_agent", "apps", "plugins", "remote_plugin", "plugin_sharing", "skill_search", "skill_mcp_dependency_install",
    "tool_suggest", "image_generation", "browser_use", "browser_use_external", "computer_use", "hooks", "fast_mode",
    "ultrafast_mode", "realtime_conversation", "workspace_dependencies", "worktrees", "goals",
    "enable_request_compression", "sleep_tool",
]


class AgentConfig:
    def __init__(self, provider, model, effort="high", cli_binary=None, max_budget_usd=None):
        if provider not in ("anthropic", "openai"):
            raise ValueError(provider)
        self.provider = provider
        self.model = model
        self.effort = effort
        self.cli_binary = cli_binary
        self.max_budget_usd = max_budget_usd

    @property
    def cli_name(self):
        return "claude" if self.provider == "anthropic" else "codex"

    def describe(self):
        return {"provider": self.provider, "model": self.model, "effort": self.effort, "cli": self.cli_name,
                "claude_tools": CLAUDE_TOOLS if self.provider == "anthropic" else None,
                "codex_features_off": CODEX_FEATURES_OFF if self.provider == "openai" else None,
                "max_budget_usd": self.max_budget_usd}


def prepare_home(cfg, home):
    """Create the jail's HOME with only the CLI configuration the run needs."""
    os.makedirs(home, exist_ok=True)
    if cfg.provider == "anthropic":
        os.makedirs(os.path.join(home, ".claude"), exist_ok=True)
        with open(os.path.join(home, ".claude", "settings.json"), "w") as f:
            json.dump({"autoUpdates": False, "includeCoAuthoredBy": False}, f, indent=2)
    else:
        codex_home = os.path.join(home, ".codex")
        os.makedirs(codex_home, exist_ok=True)
        lines = [
            f'model = "{cfg.model}"',
            'model_provider = "cv1"',
            f'model_reasoning_effort = "{cfg.effort}"',
            'approval_policy = "never"',
            'sandbox_mode = "danger-full-access"',
            'web_search = "disabled"',
            "check_for_update_on_startup = false",
            "",
            "[features]",
        ] + [f"{name} = false" for name in CODEX_FEATURES_OFF] + [
            "",
            "[history]",
            'persistence = "none"',
            "",
            "[model_providers.cv1]",
            'name = "cv1 metering proxy"',
            f'base_url = "http://127.0.0.1:{PROXY_PORT}/v1"',
            'env_key = "CV1_OPENAI_KEY"',
            'wire_api = "responses"',
            "request_max_retries = 2",
            "stream_max_retries = 2",
            "",
        ]
        with open(os.path.join(codex_home, "config.toml"), "w") as f:
            f.write("\n".join(lines))


def agent_env(cfg, base_env):
    env = dict(base_env)
    env["HOME"] = "/home/agent"
    if cfg.provider == "anthropic":
        env.update({
            "CLAUDE_CONFIG_DIR": "/home/agent/.claude",
            "ANTHROPIC_BASE_URL": f"http://127.0.0.1:{PROXY_PORT}",
            "ANTHROPIC_API_KEY": DUMMY_KEY,
            "DISABLE_AUTOUPDATER": "1",
            "DISABLE_TELEMETRY": "1",
            "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
        })
    else:
        env.update({"CODEX_HOME": "/home/agent/.codex", "CV1_OPENAI_KEY": DUMMY_KEY})
    return env


def agent_command(cfg, prompt):
    """The CLI command line inside the jail; the binary is mounted at /opt/agent/<cli>."""
    if cfg.provider == "anthropic":
        cmd = ["/opt/agent/claude", "-p", prompt, "--model", cfg.model, "--effort", cfg.effort,
               "--output-format", "stream-json", "--verbose", "--no-session-persistence",
               "--permission-mode", "bypassPermissions", "--tools", CLAUDE_TOOLS, "--strict-mcp-config"]
        if cfg.max_budget_usd:
            cmd += ["--max-budget-usd", f"{cfg.max_budget_usd:g}"]
        return cmd
    return ["/opt/agent/codex", "exec", "--strict-config", "--ephemeral", "--json", "--skip-git-repo-check",
            "--cd", "/workspace", prompt]
