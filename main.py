#!/usr/bin/env -S uv --quiet run --script
# /// script
# requires-python = ">=3.12"
# dependencies = [
#     "click",
#     "textual",
#     "rich",
#     "PyYAML",
# ]
# ///

import os
from dataclasses import replace

import click

from lazyworktree.config import load_config
from lazyworktree.app import GitWtStatus


@click.command()
@click.option(
    "--worktree-dir",
    type=click.Path(file_okay=False, dir_okay=True),
    default=None,
    help="Override the default worktree root directory.",
)
@click.option(
    "--debug-log",
    type=click.Path(dir_okay=False, writable=True),
    default=None,
    help="Path to debug log file.",
)
@click.argument("initial_filter", nargs=-1)
def main(
    worktree_dir: str | None, debug_log: str | None, initial_filter: tuple[str, ...]
) -> None:
    config = load_config()
    resolved_dir = None
    if worktree_dir:
        resolved_dir = os.path.expanduser(worktree_dir)
    elif config.worktree_dir:
        resolved_dir = os.path.expanduser(config.worktree_dir)

    if not resolved_dir:
        # Fallback to default if not specified anywhere
        resolved_dir = os.path.expanduser("~/.local/share/worktrees")

    # Update config with the authoritative worktree_dir and debug_log if provided
    updates = {"worktree_dir": resolved_dir}
    if debug_log:
        updates["debug_log"] = os.path.expanduser(debug_log)
    
    config = replace(config, **updates)

    filter_value = " ".join(initial_filter).strip()
    app = GitWtStatus(initial_filter=filter_value, config=config)
    run_result = app.run()
    if run_result:
        click.echo(run_result)


if __name__ == "__main__":
    main()
