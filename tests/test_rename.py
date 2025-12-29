import shutil
import pytest
from lazyworktree.app import GitWtStatus
from lazyworktree.config import AppConfig
from lazyworktree.screens import InputScreen


@pytest.mark.asyncio
async def test_rename_worktree_success(fake_repo, tmp_path, monkeypatch):
    monkeypatch.chdir(fake_repo.root)
    # Ensure git is available (fixture skips if not, but good to be safe)
    if shutil.which("git") is None:
        pytest.skip("git not found")

    config = AppConfig(
        worktree_dir=str(fake_repo.worktree_root.parent),
        sort_by_active=False,  # deterministic order
    )
    app = GitWtStatus(config=config)

    async with app.run_test() as pilot:
        # Wait for startup
        await pilot.pause()
        
        # Wait for table to be populated
        table = app.query_one("#worktree-table")
        async def wait_for_rows():
            for _ in range(20):
                if table.row_count > 0:
                    return
                await pilot.pause(0.1)
        await wait_for_rows()

        # Find feature1 path
        feature1_path = fake_repo.worktrees["feature1"]
        
        # Select it directly by finding the row index
        # We need to find which row corresponds to this path
        # The key is the path
        try:
            row_index = table.get_row_index(str(feature1_path))
            table.move_cursor(row=row_index)
            table.focus()
        except Exception:
            pytest.fail(f"Could not find row for {feature1_path}")
        
        # Trigger rename
        await pilot.press("m")
        
        # Should be input screen
        await pilot.pause(0.5)
        assert isinstance(app.screen, InputScreen)
        
        # Type new name "feature1-renamed"
        await pilot.press("f", "e", "a", "t", "u", "r", "e", "1", "-", "r", "e", "n", "a", "m", "e", "d", "enter")
        
        # Wait for operation
        await pilot.pause(1.0)
        
        # Verify directory moved
        old_path = fake_repo.worktrees["feature1"]
        new_path = old_path.parent / "feature1-renamed"
        
        assert not old_path.exists()
        assert new_path.exists()
        assert (new_path / "README.md").read_text(encoding="utf-8") == "dirty\n"

        # Verify app state updated
        renamed_wt = next((w for w in app.worktrees if w.branch == "feature1-renamed"), None)
        assert renamed_wt is not None
        assert str(renamed_wt.path) == str(new_path)


@pytest.mark.asyncio
async def test_rename_worktree_cancel(fake_repo, tmp_path, monkeypatch):
    monkeypatch.chdir(fake_repo.root)
    config = AppConfig(
        worktree_dir=str(fake_repo.worktree_root.parent),
    )
    app = GitWtStatus(config=config)

    async with app.run_test() as pilot:
        await pilot.pause()
        
        # Wait for table to be populated
        table = app.query_one("#worktree-table")
        async def wait_for_rows():
            for _ in range(20):
                if table.row_count > 0:
                    return
                await pilot.pause(0.1)
        await wait_for_rows()
        
        await pilot.press("down")
        await pilot.press("m")
        await pilot.pause(0.1)
        
        # Escape to cancel
        await pilot.press("escape")
        await pilot.pause(0.1)
        
        # Verify nothing changed
        old_path = fake_repo.worktrees["feature1"]
        assert old_path.exists()
