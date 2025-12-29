from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
import os
import shutil
import subprocess

import pytest


@dataclass(frozen=True)
class FakeRepo:
    root: Path
    worktree_root: Path
    worktrees: dict[str, Path]


def _git(args: list[str], *, cwd: Path, env: dict) -> None:
    subprocess.run(
        ["git", *args],
        cwd=cwd,
        env=env,
        check=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )


@pytest.fixture()
def fake_repo(tmp_path: Path) -> FakeRepo:
    if shutil.which("git") is None:
        pytest.skip("git is required for integration tests")
    env = os.environ.copy()
    for key in (
        "GIT_DIR",
        "GIT_WORK_TREE",
        "GIT_INDEX_FILE",
        "GIT_COMMON_DIR",
        "GIT_ALTERNATE_OBJECT_DIRECTORIES",
    ):
        env.pop(key, None)
    env.setdefault("GIT_AUTHOR_NAME", "Test User")
    env.setdefault("GIT_AUTHOR_EMAIL", "test@example.com")
    env.setdefault("GIT_COMMITTER_NAME", "Test User")
    env.setdefault("GIT_COMMITTER_EMAIL", "test@example.com")

    repo = tmp_path / "repo"
    worktree_root = tmp_path / "worktrees" / "repo"
    repo.mkdir()
    _git(["init", "-b", "main"], cwd=repo, env=env)
    _git(["config", "user.name", "Test User"], cwd=repo, env=env)
    _git(["config", "user.email", "test@example.com"], cwd=repo, env=env)
    _git(["config", "commit.gpgsign", "false"], cwd=repo, env=env)

    (repo / "README.md").write_text("hello\n", encoding="utf-8")
    _git(["add", "README.md"], cwd=repo, env=env)
    _git(["commit", "-m", "init"], cwd=repo, env=env)

    _git(["branch", "feature1"], cwd=repo, env=env)
    _git(["branch", "feature2"], cwd=repo, env=env)
    _git(["branch", "new-branch"], cwd=repo, env=env)

    worktree_root.parent.mkdir(parents=True, exist_ok=True)
    wt1 = worktree_root / "feature1"
    wt2 = worktree_root / "feature2"
    _git(["worktree", "add", str(wt1), "feature1"], cwd=repo, env=env)
    _git(["worktree", "add", str(wt2), "feature2"], cwd=repo, env=env)

    (wt1 / "README.md").write_text("dirty\n", encoding="utf-8")
    (wt2 / "README.md").write_text("change\n", encoding="utf-8")
    _git(["add", "README.md"], cwd=wt2, env=env)

    return FakeRepo(
        root=repo,
        worktree_root=worktree_root,
        worktrees={"feature1": wt1, "feature2": wt2},
    )
