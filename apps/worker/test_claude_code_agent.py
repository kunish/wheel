#!/usr/bin/env python3
"""Integration test simulating Claude Code calls against a wheel endpoint.

This script uses the Python Claude Agent SDK (claude-agent-sdk) to launch
Claude Code, inject wheel environment values (base URL, key, model), and run
small end-to-end checks.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Iterable

import anyio
from claude_agent_sdk import (
    AssistantMessage,
    ClaudeAgentOptions,
    Message,
    ResultMessage,
    TextBlock,
    ToolUseBlock,
    UserMessage,
    query,
)

DEFAULT_SETTINGS_PATH = Path(".claude/settings.json")
DEFAULT_MODEL = "claude-opus-4-6-thinking"
DEFAULT_BASE_URL = "http://localhost:8787"


@dataclass
class ResolvedConfig:
    base_url: str
    api_key: str
    model: str
    env: dict[str, str]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Run Claude Code compatibility checks against wheel by injecting "
            "Anthropic env vars (base URL, API key, model)."
        )
    )
    parser.add_argument(
        "--settings",
        default=str(DEFAULT_SETTINGS_PATH),
        help="Path to Claude settings JSON. Default: .claude/settings.json",
    )
    parser.add_argument(
        "--base-url",
        default=None,
        help="Anthropic-compatible base URL, e.g. http://localhost:8787",
    )
    parser.add_argument(
        "--key",
        default=None,
        help="API key for wheel, e.g. sk-wheel-xxx",
    )
    parser.add_argument(
        "--model",
        default=None,
        help=f"Model name. Default: {DEFAULT_MODEL}",
    )
    parser.add_argument(
        "--cwd",
        default=".",
        help="Working directory visible to Claude Code. Default: current directory",
    )
    parser.add_argument(
        "--read-path",
        default="README.md",
        help="Path used in the tool-use test (Read tool). Default: README.md",
    )
    parser.add_argument(
        "--permission-mode",
        default="bypassPermissions",
        choices=["default", "acceptEdits", "plan", "bypassPermissions"],
        help="Claude Code permission mode. Default: bypassPermissions",
    )
    parser.add_argument(
        "--max-turns",
        type=int,
        default=3,
        help="Max turns per test case. Default: 3",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print resolved config and exit without calling Claude Code",
    )
    parser.add_argument(
        "--verbose",
        action="store_true",
        help="Print message-level debug output",
    )
    return parser.parse_args()


def load_settings_env(path: Path) -> dict[str, str]:
    if not path.exists():
        return {}

    with path.open("r", encoding="utf-8") as f:
        try:
            data = json.load(f)
        except json.JSONDecodeError:
            return {}

    env = data.get("env", {})
    if not isinstance(env, dict):
        return {}

    out: dict[str, str] = {}
    for key, value in env.items():
        if isinstance(key, str) and isinstance(value, str):
            out[key] = value
    return out


def first_non_empty(
    values: Iterable[str | None], fallback: str | None = None
) -> str | None:
    for value in values:
        if value:
            stripped = value.strip()
            if stripped:
                return stripped
    return fallback


def mask_secret(value: str) -> str:
    if len(value) <= 10:
        return "*" * len(value)
    return f"{value[:6]}...{value[-4:]}"


def resolve_config(args: argparse.Namespace) -> ResolvedConfig:
    settings_env = load_settings_env(Path(args.settings))

    base_url = first_non_empty(
        [
            args.base_url,
            os.getenv("ANTHROPIC_BASE_URL"),
            settings_env.get("ANTHROPIC_BASE_URL"),
        ],
        DEFAULT_BASE_URL,
    )

    api_key = first_non_empty(
        [
            args.key,
            os.getenv("ANTHROPIC_AUTH_TOKEN"),
            os.getenv("ANTHROPIC_API_KEY"),
            settings_env.get("ANTHROPIC_AUTH_TOKEN"),
            settings_env.get("ANTHROPIC_API_KEY"),
        ]
    )

    model = first_non_empty(
        [
            args.model,
            os.getenv("ANTHROPIC_MODEL"),
            settings_env.get("ANTHROPIC_MODEL"),
            settings_env.get("ANTHROPIC_DEFAULT_OPUS_MODEL"),
            settings_env.get("ANTHROPIC_DEFAULT_SONNET_MODEL"),
        ],
        DEFAULT_MODEL,
    )

    if not base_url:
        raise ValueError("Missing base URL. Use --base-url or set ANTHROPIC_BASE_URL.")
    if not api_key:
        raise ValueError(
            "Missing API key. Use --key or set ANTHROPIC_AUTH_TOKEN / ANTHROPIC_API_KEY."
        )
    if not model:
        raise ValueError("Missing model. Use --model or set ANTHROPIC_MODEL.")

    merged_env = dict(settings_env)

    # Ensure canonical variables for both Claude Code and SDK compatibility.
    merged_env["ANTHROPIC_BASE_URL"] = base_url
    merged_env["ANTHROPIC_AUTH_TOKEN"] = api_key
    merged_env["ANTHROPIC_API_KEY"] = api_key
    merged_env["ANTHROPIC_MODEL"] = model

    return ResolvedConfig(
        base_url=base_url, api_key=api_key, model=model, env=merged_env
    )


def option_base(args: argparse.Namespace, config: ResolvedConfig) -> ClaudeAgentOptions:
    def stderr_printer(line: str) -> None:
        if args.verbose:
            print(f"[claude-stderr] {line}")

    return ClaudeAgentOptions(
        model=config.model,
        cwd=args.cwd,
        env=config.env,
        max_turns=args.max_turns,
        permission_mode=args.permission_mode,
        stderr=stderr_printer if args.verbose else None,
    )


def collect_text_from_assistant(messages: list[Message]) -> str:
    chunks: list[str] = []
    for message in messages:
        if isinstance(message, AssistantMessage):
            for block in message.content:
                if isinstance(block, TextBlock):
                    chunks.append(block.text)
    return "".join(chunks).strip()


def result_messages(messages: list[Message]) -> list[ResultMessage]:
    return [message for message in messages if isinstance(message, ResultMessage)]


def contains_tool_use(messages: list[Message], tool_name: str) -> bool:
    for message in messages:
        if isinstance(message, AssistantMessage):
            for block in message.content:
                if isinstance(block, ToolUseBlock) and block.name == tool_name:
                    return True
    return False


def has_tool_result_feedback(messages: list[Message]) -> bool:
    for message in messages:
        if isinstance(message, UserMessage) and message.tool_use_result is not None:
            return True
    return False


async def run_query(
    prompt: str, options: ClaudeAgentOptions, verbose: bool = False
) -> list[Message]:
    out: list[Message] = []
    async for message in query(prompt=prompt, options=options):
        out.append(message)
        if verbose:
            print(f"  [msg] {type(message).__name__}")
    return out


async def test_basic_text(
    options: ClaudeAgentOptions, verbose: bool
) -> tuple[bool, str]:
    prompt = "Reply with exactly: PONG"
    messages = await run_query(prompt=prompt, options=options, verbose=verbose)

    results = result_messages(messages)
    if not results:
        return False, "no result message returned"
    if any(result.is_error for result in results):
        return False, "result message indicates error"

    text = collect_text_from_assistant(messages)
    if not text:
        return False, "assistant returned empty text"

    return True, f"assistant text length={len(text)}"


async def test_tool_use_read(
    options: ClaudeAgentOptions,
    read_path: str,
    verbose: bool,
) -> tuple[bool, str]:
    prompt = (
        "Use the Read tool to read this path: "
        f"{read_path}. "
        "You MUST call the Read tool first, then summarize the first line in one sentence."
    )

    messages = await run_query(prompt=prompt, options=options, verbose=verbose)
    results = result_messages(messages)

    if not results:
        return False, "no result message returned"
    if any(result.is_error for result in results):
        return False, "result message indicates error"

    used_read = contains_tool_use(messages, "Read")
    got_tool_result = has_tool_result_feedback(messages)
    text = collect_text_from_assistant(messages)

    if not used_read:
        return False, "assistant did not emit Read tool_use block"
    if not got_tool_result:
        return False, "tool_result feedback message not observed"
    if not text:
        return False, "assistant returned empty text"

    return True, "observed Read tool_use + tool_result round trip"


async def async_main(args: argparse.Namespace, config: ResolvedConfig) -> int:
    base_options = option_base(args, config)

    tests: list[tuple[str, Any]] = [
        ("basic text response", test_basic_text),
        ("tool-use Read round trip", test_tool_use_read),
    ]

    passed = 0
    failed = 0

    for name, fn in tests:
        print(f"- {name} ... ", end="", flush=True)
        try:
            if fn is test_tool_use_read:
                ok, detail = await fn(base_options, args.read_path, args.verbose)
            else:
                ok, detail = await fn(base_options, args.verbose)
        except Exception as exc:  # noqa: BLE001
            ok = False
            detail = f"exception: {exc}"

        if ok:
            passed += 1
            print("PASS")
        else:
            failed += 1
            print("FAIL")
        print(f"  {detail}")

    print(f"\nSummary: {passed + failed} tests, {passed} passed, {failed} failed")
    return 0 if failed == 0 else 1


def main() -> int:
    args = parse_args()

    try:
        config = resolve_config(args)
    except Exception as exc:  # noqa: BLE001
        print(f"Config error: {exc}", file=sys.stderr)
        return 2

    print("Resolved Claude environment:")
    print(f"  ANTHROPIC_BASE_URL={config.base_url}")
    print(f"  ANTHROPIC_AUTH_TOKEN={mask_secret(config.api_key)}")
    print(f"  ANTHROPIC_MODEL={config.model}")
    print(f"  cwd={Path(args.cwd).resolve()}")

    if args.dry_run:
        print("Dry-run mode: no Claude call executed.")
        return 0

    return anyio.run(async_main, args, config)


if __name__ == "__main__":
    raise SystemExit(main())
