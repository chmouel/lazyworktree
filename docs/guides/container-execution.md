# Container Execution

LazyWorktree can run custom commands inside OCI containers (Docker or Podman), giving each worktree an isolated, reproducible environment without requiring local toolchains.

<div class="mint-callout">
  <p><strong>Use this page when:</strong> you want to run custom commands inside Docker or Podman containers, with automatic worktree mounting.</p>
</div>

## Overview

Container execution enables you to:

- Run builds and tests in isolated containers with pinned toolchain versions
- Reproduce CI environments locally without installing dependencies
- Combine containers with [multiplexer sessions](multiplexer-integration.md) for multi-window container workflows
- Auto-detect Docker or Podman — no runtime configuration needed

## Basic Examples

### Image Only

Specify just an image to launch a container with the image's default entrypoint. The worktree is automatically mounted at `/workspace`.

```yaml
custom_commands:
  ctrl+d:
    description: Default shell in container
    container:
      image: "ubuntu:24.04"
      interactive: true
```

### Image + Command

Run a specific command inside the container:

```yaml
custom_commands:
  T:
    command: "go test ./..."
    description: Tests in container
    show_output: true
    container:
      image: "golang:1.22"
```

### Image + Entrypoint

Override the image's default entrypoint — useful for images that bundle multiple tools or need a specific shell:

```yaml
custom_commands:
  ctrl+c:
    description: Claude Code in container
    container:
      image: "ghcr.io/anthropics/claude-code:latest"
      entrypoint: "/bin/bash"
      interactive: true
```

!!! tip
    **`entrypoint` vs `command`:** The `entrypoint` overrides the binary that runs inside the container, whilst `command` provides arguments passed via `sh -c`. When both are set, the entrypoint runs with the command as its argument. When neither is set, the container runs with its image defaults.

## Combining with Multiplexers

Container execution can be combined with tmux or zellij. Each window/tab command is individually wrapped in a container invocation, so every pane runs inside the same image.

### Container + tmux

```yaml
custom_commands:
  ctrl+l:
    description: Dev container
    tmux:
      session_name: "dev:$WORKTREE_NAME"
      windows:
        - name: test
          command: "go test -v ./..."
        - name: shell
    container:
      image: "golang:1.22"
      extra_args:
        - "--network=host"
```

### Container + zellij

```yaml
custom_commands:
  ctrl+z:
    description: Node dev container
    zellij:
      session_name: "dev:$WORKTREE_NAME"
      windows:
        - name: server
          command: "npm run dev"
        - name: test
          command: "npm test"
        - name: shell
    container:
      image: "node:20"
```

## Mounts and Environment

### Automatic Mount

The worktree path is automatically bind-mounted to the container's working directory (`/workspace` by default). No configuration needed.

If you specify a custom mount targeting the same path as `working_dir`, the automatic mount is skipped.

### Custom Mounts

Add extra bind mounts for caches, credentials, or shared data:

```yaml
custom_commands:
  B:
    command: "go build ./..."
    description: Build with cache
    container:
      image: "golang:1.22"
      mounts:
        - source: ~/.cache/go-build
          target: /root/.cache/go-build
        - source: ~/go/pkg/mod
          target: /root/go/pkg/mod
          read_only: true
```

### Environment Variables

Forward environment variables into the container:

```yaml
container:
  image: "node:20"
  env:
    NODE_ENV: development
    API_KEY: "${API_KEY}"
```

`WORKTREE_*` environment variables (`WORKTREE_BRANCH`, `WORKTREE_PATH`, `WORKTREE_NAME`, `MAIN_WORKTREE_PATH`, `REPO_NAME`) are forwarded into the container automatically.

## Advanced

### Extra Arguments

Pass additional flags to the `docker run` or `podman run` invocation:

```yaml
container:
  image: "golang:1.22"
  extra_args:
    - "--network=host"
    - "--cpus=2"
    - "--memory=4g"
```

### Interactive Mode

Allocate a TTY for interactive use (adds `-it` flags):

```yaml
container:
  image: "ubuntu:24.04"
  interactive: true
```

### Runtime Selection

By default, LazyWorktree auto-detects the container runtime, preferring Podman over Docker. Override with:

```yaml
container:
  image: "golang:1.22"
  runtime: docker   # or "podman"
```

### Working Directory

Change the mount point inside the container:

```yaml
container:
  image: "golang:1.22"
  working_dir: /src
```

## Field Reference

For the complete list of container fields and mount options, see the [Container Fields](../custom-commands.md#container-fields) section in the Custom Commands reference.
