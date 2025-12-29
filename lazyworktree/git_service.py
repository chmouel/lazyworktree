import asyncio
import json
import os
import re
from typing import Iterable, Optional, Callable, Awaitable

from .models import PRInfo, WorktreeInfo

NotifyFn = Callable[[str, str], None]
NotifyOnceFn = Callable[[str, str, str], None]
RunnerFn = Callable[..., Awaitable[str]]


class GitService:
    def __init__(
        self,
        notify: NotifyFn,
        notify_once: NotifyOnceFn,
        runner: Optional[RunnerFn] = None,
    ) -> None:
        self._notify = notify
        self._notify_once = notify_once
        self._runner = runner
        self._semaphore = asyncio.Semaphore(24)
        self._main_branch: Optional[str] = None
        self._git_host: Optional[str] = None

    async def run_git(
        self,
        args: list[str],
        cwd: Optional[str] = None,
        ok_returncodes: Iterable[int] = (0,),
        strip: bool = True,
        silent: bool = False,
    ) -> str:
        if self._runner is not None:
            return await self._runner(
                args, cwd=cwd, ok_returncodes=ok_returncodes, strip=strip
            )
        try:
            proc = await asyncio.create_subprocess_exec(
                *args,
                cwd=cwd,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
            )
            stdout, stderr = await proc.communicate()
            if proc.returncode not in set(ok_returncodes):
                if silent:
                    return ""
                detail = (
                    stderr.decode(errors="replace").strip()
                    or stdout.decode(errors="replace").strip()
                )
                command = " ".join(args)
                suffix = f": {detail}" if detail else f" (exit {proc.returncode})"
                self._notify_once(
                    f"git_fail:{cwd}:{command}",
                    f"Command failed: {command}{suffix}",
                    severity="error",
                )
                return ""
            out = stdout.decode(errors="replace")
            return out.strip() if strip else out
        except FileNotFoundError:
            if silent:
                return ""
            command = args[0] if args else "command"
            self._notify_once(
                f"cmd_missing:{command}",
                f"Command not found: {command}",
                severity="error",
            )
            return ""
        except Exception as exc:
            if silent:
                return ""
            command = " ".join(args)
            self._notify_once(
                f"cmd_error:{cwd}:{command}",
                f"Failed to run command: {command}: {exc}",
                severity="error",
            )
            return ""

    async def run_command_checked(
        self, args: list[str], cwd: Optional[str], error_prefix: str
    ) -> bool:
        try:
            proc = await asyncio.create_subprocess_exec(
                *args,
                cwd=cwd,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
            )
            stdout, stderr = await proc.communicate()
        except Exception as exc:
            self._notify(f"{error_prefix}: {exc}", severity="error")
            return False
        if proc.returncode == 0:
            return True
        detail = (
            stderr.decode(errors="replace").strip()
            or stdout.decode(errors="replace").strip()
        )
        if detail:
            self._notify(f"{error_prefix}: {detail}", severity="error")
        else:
            self._notify(error_prefix, severity="error")
        return False

    async def get_main_branch(self) -> str:
        if self._main_branch:
            return self._main_branch
        out = await self.run_git(
            ["git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD"]
        )
        if out:
            self._main_branch = out.split("/")[-1]
        else:
            self._main_branch = "main"
        return self._main_branch

    async def get_worktrees(self) -> list[WorktreeInfo]:
        raw_wts = await self.run_git(["git", "worktree", "list", "--porcelain"])
        if not raw_wts:
            return []
        wts: list[dict[str, object]] = []
        current_wt: dict[str, object] = {}
        for line in raw_wts.splitlines():
            if line.startswith("worktree "):
                if current_wt:
                    wts.append(current_wt)
                current_wt = {"path": line.split(" ", 1)[1]}
            elif line.startswith("branch "):
                current_wt["branch"] = line.split(" ", 1)[1].replace("refs/heads/", "")
        if current_wt:
            wts.append(current_wt)
        for i, wt_data in enumerate(wts):
            wt_data["is_main"] = i == 0
        branch_raw = await self.run_git(
            [
                "git",
                "for-each-ref",
                "--format=%(refname:short)|%(committerdate:relative)|%(committerdate:unix)",
                "refs/heads",
            ]
        )
        branch_info: dict[str, tuple[str, int]] = {}
        for line in branch_raw.splitlines():
            if "|" in line:
                parts = line.split("|")
                if len(parts) == 3:
                    branch_info[parts[0]] = (parts[1], int(parts[2]))

        async def get_wt_info(wt_data: dict[str, object]) -> WorktreeInfo:
            async with self._semaphore:
                path = str(wt_data["path"])
                branch = str(wt_data.get("branch", "(detached)"))
                status_raw = await self.run_git(
                    ["git", "status", "--porcelain=v2", "--branch"], cwd=path
                )
                ahead = 0
                behind = 0
                untracked = 0
                modified = 0
                staged = 0
                for line in status_raw.splitlines():
                    if line.startswith("# branch.ab "):
                        parts = line.split()
                        if len(parts) >= 4:
                            ahead = int(parts[2].replace("+", ""))
                            behind = int(parts[3].replace("-", ""))
                    elif line.startswith("?"):
                        untracked += 1
                    elif line.startswith("1 ") or line.startswith("2 "):
                        parts = line.split()
                        if len(parts) > 1:
                            xy = parts[1]
                            if len(xy) >= 2:
                                if xy[0] != ".":
                                    staged += 1
                                if xy[1] != ".":
                                    modified += 1
                last_active, last_active_ts = branch_info.get(branch, ("", 0))
                return WorktreeInfo(
                    path=path,
                    branch=branch,
                    is_main=bool(wt_data["is_main"]),
                    dirty=(untracked + modified + staged > 0),
                    ahead=ahead,
                    behind=behind,
                    last_active=last_active,
                    last_active_ts=last_active_ts,
                    untracked=untracked,
                    modified=modified,
                    staged=staged,
                )

        return await asyncio.gather(*(get_wt_info(wt) for wt in wts))

    async def _detect_host(self) -> str:
        if self._git_host:
            return self._git_host
        # Try finding origin URL
        try:
            remote_url = await self.run_git(
                ["git", "remote", "get-url", "origin"], silent=True
            )
            # Match standard git URLs:
            # git@github.com:user/repo.git
            # https://github.com/user/repo.git
            # ssh://git@gitlab.com:2222/user/repo.git
            match = re.search(
                r"(?:git@|https?://|ssh://|git://)(?:[^@]+@)?([^/:]+)", remote_url
            )
            if match:
                hostname = match.group(1).lower()
                if "gitlab" in hostname:
                    self._git_host = "gitlab"
                    return "gitlab"
                if "github" in hostname:
                    self._git_host = "github"
                    return "github"
        except Exception:
            pass

        self._git_host = "unknown"
        return "unknown"

    async def _fetch_gitlab_prs(self) -> Optional[dict[str, PRInfo]]:
        pr_raw = await self.run_git(
            ["glab", "api", "merge_requests?state=all&per_page=100"], silent=False
        )
        if not pr_raw:
            return None
        try:
            prs = json.loads(pr_raw)
        except json.JSONDecodeError as exc:
            self._notify_once(
                "pr_json_decode_glab",
                f"Failed to parse GLAB PR data: {exc}",
                severity="error",
            )
            return None
        pr_map: dict[str, PRInfo] = {}
        for p in prs:
            state = p.get("state", "").upper()
            if state == "OPENED":
                state = "OPEN"
            pr_map[p["source_branch"]] = PRInfo(
                p["iid"], state, p["title"], p["web_url"]
            )
        return pr_map

    async def fetch_pr_map(self) -> Optional[dict[str, PRInfo]]:
        host = await self._detect_host()
        if host == "gitlab":
            return await self._fetch_gitlab_prs()

        # Default to github logic (or if explicitly detected)
        pr_raw = await self.run_git(
            [
                "gh",
                "pr",
                "list",
                "--state",
                "all",
                "--json",
                "headRefName,state,number,title,url",
                "--limit",
                "100",
            ],
            silent=host == "unknown",  # Silence if we aren't sure it's github
        )
        if pr_raw == "":
            return None
        try:
            prs = json.loads(pr_raw)
        except json.JSONDecodeError as exc:
            self._notify_once(
                "pr_json_decode", f"Failed to parse PR data: {exc}", severity="error"
            )
            return None
        pr_map: dict[str, PRInfo] = {}
        for p in prs:
            pr_map[p["headRefName"]] = PRInfo(
                p["number"], p["state"], p["title"], p["url"]
            )
        return pr_map

    async def get_main_worktree_path(self) -> str:
        raw_wts = await self.run_git(["git", "worktree", "list", "--porcelain"])
        for line in raw_wts.splitlines():
            if line.startswith("worktree "):
                return line.split(" ", 1)[1]
        return os.getcwd()
