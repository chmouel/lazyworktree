from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
import os
import yaml


DEFAULT_CONFIG_PATHS = (
    Path(os.environ.get("XDG_CONFIG_HOME", "~/.config")).expanduser()
    / "lazyworktree"
    / "config.yaml",
    Path(os.environ.get("XDG_CONFIG_HOME", "~/.config")).expanduser()
    / "lazyworktree"
    / "config.yml",
)


def normalize_command_list(value: object) -> list[str]:
    if value is None:
        return []
    if isinstance(value, str):
        text = value.strip()
        return [text] if text else []
    if isinstance(value, (list, tuple)):
        commands: list[str] = []
        for item in value:
            if item is None:
                continue
            text = str(item).strip()
            if text:
                commands.append(text)
        return commands
    return []


def _coerce_bool(value: object, default: bool) -> bool:
    if isinstance(value, bool):
        return value
    if isinstance(value, int):
        return bool(value)
    if isinstance(value, str):
        text = value.strip().lower()
        if text in {"1", "true", "yes", "y", "on"}:
            return True
        if text in {"0", "false", "no", "n", "off"}:
            return False
    return default


def _coerce_int(value: object, default: int) -> int:
    if isinstance(value, bool):
        return default
    if isinstance(value, int):
        return value
    if isinstance(value, str):
        text = value.strip()
        if not text:
            return default
        try:
            return int(text)
        except ValueError:
            return default
    return default


@dataclass(frozen=True)
class AppConfig:
    worktree_dir: str | None = None
    init_commands: tuple[str, ...] = ()
    terminate_commands: tuple[str, ...] = ()
    sort_by_active: bool = True
    auto_fetch_prs: bool = False
    max_untracked_diffs: int = 10
    max_diff_chars: int = 200_000
    trust_mode: str = "tofu"
    debug_log: str | None = None


def _parse_config(data: object) -> AppConfig:
    if not isinstance(data, dict):
        return AppConfig()
    worktree_dir = data.get("worktree_dir")
    if isinstance(worktree_dir, str):
        worktree_dir = worktree_dir.strip() or None
    else:
        worktree_dir = None
    
    debug_log = data.get("debug_log")
    if isinstance(debug_log, str):
        debug_log = debug_log.strip() or None
    else:
        debug_log = None

    init_commands = tuple(normalize_command_list(data.get("init_commands")))
    terminate_commands = tuple(normalize_command_list(data.get("terminate_commands")))
    sort_by_active = _coerce_bool(data.get("sort_by_active"), True)
    auto_fetch_prs = _coerce_bool(data.get("auto_fetch_prs"), False)
    max_untracked_diffs = _coerce_int(data.get("max_untracked_diffs"), 10)
    max_diff_chars = _coerce_int(data.get("max_diff_chars"), 200_000)

    trust_mode = str(data.get("trust_mode", "tofu")).strip().lower()
    if trust_mode not in {"tofu", "never", "always"}:
        trust_mode = "tofu"

    if max_untracked_diffs < 0:
        max_untracked_diffs = 0
    if max_diff_chars < 0:
        max_diff_chars = 0
    return AppConfig(
        worktree_dir=worktree_dir,
        init_commands=init_commands,
        terminate_commands=terminate_commands,
        sort_by_active=sort_by_active,
        auto_fetch_prs=auto_fetch_prs,
        max_untracked_diffs=max_untracked_diffs,
        max_diff_chars=max_diff_chars,
        trust_mode=trust_mode,
        debug_log=debug_log,
    )


def load_config(config_path: str | None = None) -> AppConfig:
    paths = [Path(config_path).expanduser()] if config_path else DEFAULT_CONFIG_PATHS
    for path in paths:
        if not path.exists():
            continue
        try:
            with path.open("r", encoding="utf-8") as handle:
                data = yaml.safe_load(handle) or {}
        except (OSError, yaml.YAMLError):
            return AppConfig()
        return _parse_config(data)
    return AppConfig()
