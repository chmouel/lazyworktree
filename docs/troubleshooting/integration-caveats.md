# Integration Caveats

Known caveats across shells, pagers, and command execution modes.

## Shell execution mode differences

`--exec` and command execution use shell modes based on your shell:

- zsh: `-ilc`
- bash: `-ic`
- others: `-lc`

Profile or startup scripts can affect behaviour and environment.

## Pager integration caveats

- interactive pagers may require `git_pager_interactive: true`
- command-mode tools may require `git_pager_command_mode: true`
- CI log formatting scripts should be tested independently in shell

## Multiplexer caveats

- tmux/zellij session names are sanitised
- existing-session behaviour depends on `on_exists` configuration
- CLI `exec` does not support `new-tab` commands

## Trust model caveats for `.wt`

- `trust_mode: never` blocks repository `.wt` command execution
- modified `.wt` files trigger trust re-evaluation in `tofu` mode

## Next Steps

<div class="mint-card-grid">
  <a class="mint-card" href="common-problems.md">
    <strong>Common Problems</strong>
    <span>Return to common issue patterns and fixes.</span>
  </a>
  <a class="mint-card" href="faq.md">
    <strong>FAQ</strong>
    <span>Check concise answers before deeper debugging.</span>
  </a>
</div>
