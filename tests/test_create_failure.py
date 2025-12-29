import pytest
import asyncio
from unittest.mock import patch, MagicMock, AsyncMock
from lazyworktree.app import GitWtStatus
from lazyworktree.config import AppConfig
from lazyworktree.screens import InputScreen

@pytest.mark.asyncio
async def test_create_worktree_failure_tui(fake_repo, tmp_path):
    debug_log = tmp_path / "debug.log"
    config = AppConfig(
        worktree_dir=str(fake_repo.worktree_root.parent),
        debug_log=str(debug_log)
    )
    app = GitWtStatus(config=config)

    mock_process = MagicMock()
    mock_process.returncode = 128
    mock_process.communicate = AsyncMock(return_value=(b"", b"fatal: simulated failure"))

    # We need to capture the real create_subprocess_exec to use in side_effect
    real_create_subprocess_exec = asyncio.create_subprocess_exec

    async def side_effect(program, *args, **kwargs):
        # Check if this is the worktree add command
        # The call in app.py: asyncio.create_subprocess_exec("git", "worktree", "add", new_path, name, ...)
        if program == "git" and len(args) >= 2 and args[0] == "worktree" and args[1] == "add":
            return mock_process
        return await real_create_subprocess_exec(program, *args, **kwargs)

    with patch("asyncio.create_subprocess_exec", side_effect=side_effect) as mock_exec:
        async with app.run_test() as pilot:
            # Wait for startup and initial refresh
            await pilot.pause()
            
            # Trigger create dialog
            await pilot.press("c")
            
            # Wait for InputScreen to appear
            # We can loop briefly to ensure it's there
            await pilot.pause(0.1)
            assert isinstance(app.screen, InputScreen)
            
            # Type name and submit
            await pilot.press("n", "e", "w", "-", "f", "a", "i", "l", "enter")
            
            # Wait for the worker to finish and notification to happen
            await pilot.pause(0.5)
            
            # Check debug log content
            assert debug_log.exists()
            content = debug_log.read_text()
            assert "Creating worktree new-fail" in content
            assert "fatal: simulated failure" in content
            assert "Failed to create worktree new-fail" in content
