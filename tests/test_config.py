from lazyworktree.config import load_config, normalize_command_list


def test_normalize_command_list() -> None:
    assert normalize_command_list(None) == []
    assert normalize_command_list("echo hi") == ["echo hi"]
    assert normalize_command_list("  ") == []
    assert normalize_command_list(["echo a", "", None, "echo b"]) == [
        "echo a",
        "echo b",
    ]


def test_load_config_from_path(tmp_path) -> None:
    config_path = tmp_path / "config.yaml"
    config_path.write_text(
        "worktree_dir: /tmp/worktrees\n"
        "sort_by_active: false\n"
        "auto_fetch_prs: true\n"
        "max_untracked_diffs: 0\n"
        "max_diff_chars: 250000\n"
        "debug_log: /tmp/debug.log\n"
        "init_commands:\n"
        "  - link_topsymlinks\n"
        "terminate_commands: echo bye\n",
        encoding="utf-8",
    )

    config = load_config(str(config_path))

    assert config.worktree_dir == "/tmp/worktrees"
    assert config.debug_log == "/tmp/debug.log"
    assert list(config.init_commands) == ["link_topsymlinks"]
    assert list(config.terminate_commands) == ["echo bye"]
    assert config.sort_by_active is False
    assert config.auto_fetch_prs is True
    assert config.max_untracked_diffs == 0
    assert config.max_diff_chars == 250000
