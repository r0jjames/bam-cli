# bam

A terminal remote control for Atlassian Bamboo: trigger, watch and diagnose builds without the browser, and get the Bamboo link the moment you need it.

    bam run provision-lab --var cluster_name=alpha --watch
    bam logs --last --failed
    bam open --last

bam works with Bamboo Data Center 9.x and later, using a personal access token.

## Install

Download a binary for your platform from the [releases page](https://github.com/r0jjames/bam-cli/releases), or build from source:

    go install github.com/r0jjames/bam-cli/cmd/bam@latest

Check that `bam` does not shadow another command on your machine: `type bam`.

## Quick start

1. Register your server and log in. `login` asks for a personal access token, which you create in Bamboo under your profile → Personal access tokens; bam prints the link.

       bam server add work --url https://bamboo.example.com --project PROJ

2. Create `.bam.yaml` in your repository with presets generated from your plans' variables:

       bam init --server work --project PROJ --plan PROJ-BUILD --plan PROJ-PROV=provision-lab

   Review the generated file, then commit it. It holds no secrets.

3. Run, watch, and investigate:

       bam run provision-lab --var cluster_name=alpha --watch
       bam logs --last --failed
       bam open --last

A teammate who clones the repository only needs `bam login work`.

## Terminal UI

Running `bam` alone on a terminal opens a lazygit-style UI over the same
commands: browse, watch a build live, read its logs, run a preset, and cancel.

No lowercase key changes anything on the server. `R` only opens the run form,
`ctrl-R` inside that form is the only key that starts a build, and `C` always
asks before it cancels one.

```
┏━1 Plans━━━━━━━━━━┓╭─PROJ-BUILD-44────────────────────────────────────────────╮
┃▸ PROJ-BUILD      ┃│✗ failed   #44   branch main   3m12s                      │
┃  PROJ-DEPLOY     ┃│jdoe · manual                                             │
┃  OPS-NIGHTLY     ┃│                                                          │
┗━━━━━━━━━━━━━━━━━━┛│✓ Build       42s                                         │
╭─2 Builds─────────╮│  ✓ Compile        28s                                    │
│▸ ✗ #44 3m ago    ││  ✓ Unit tests     14s                                    │
│  ✓ #43 1h ago    ││✗ Test        2m30s                                       │
╰──────────────────╯│  ✗ Integration    2m30s                                  │
╭─3 Presets────────╮│– Deploy                                                  │
│  provision-lab   ││                                                          │
│  smoke           ││3 failed tests                                            │
╰──────────────────╯╰──────────────────────────────────────────────────────────╯
lab · 9.6.4 · jdoe                      ?help  tab focus  l logs  o open  q quit
```

`bam ui` opens it explicitly. To keep the UI out of the way, use `--no-tui`,
set `BAM_NO_TUI=1`, or pipe bam's output — on a pipe, on a dumb terminal, or
without a terminal, bare `bam` prints help exactly as before.

### Keys

| | Keys | Does |
| --- | --- | --- |
| Move | `j` `k` `↑` `↓` | up / down in the focused panel |
| | `g` `G` | first / last row |
| | `tab` | next panel |
| | `shift-tab` | previous panel |
| | `1` `2` `3` | focus Plans / Builds / Presets |
| | `enter` | drill in |
| | `esc` | back out one level, close an overlay |
| View | `l` | logs for the selection (failed jobs by default) |
| | `a` | all logs, not only failed |
| | `f` | follow (log screen) |
| | `/` | filter the focused list, or search the log |
| | `n` `N` | next / previous match, or next failure |
| | `r` | refresh the focused panel |
| | `e` | expand the current error |
| Run | `R` | open the run form for the selection |
| | `ctrl-R` | run (in the form) |
| | `d` | dry-run: show what would be sent (in the form) |
| | `C` | cancel the selected build (asks first) |
| Go | `o` | open the selection's Bamboo URL in the browser |
| | `y` | copy the selection's Bamboo URL |
| | `S` | switch server |
| | `P` | filter by project |
| | `b` | switch plan branch |
| Meta | `?` | help |
| | `q` `ctrl-c` | quit |

Leaving the UI never stops a build: quitting, leaving a screen and switching
server all cancel *watches*, and a watch is not a build. Only `C` in the UI,
and `bam build cancel` on the command line, stop a build.

## Everyday commands

| I want to… | Command |
| --- | --- |
| see my plans | `bam plan list` |
| see a plan's variables and what the last build used | `bam plan vars provision-lab` |
| run with the previous build's variables | `bam run provision-lab --from last` |
| check variables without running | `bam run provision-lab --dry-run` |
| see recent builds | `bam build list provision-lab` |
| follow a running build | `bam watch --last` |
| read only the failed jobs' logs | `bam logs --last --failed` |
| stop a build | `bam build cancel --last` |
| open anything in Bamboo | `bam open PROJ-PLAN-123` |

Every command has `--help`, and every read command has `--json`. The full reference is in [docs/cli](docs/cli/bam.md).

## More

- [Configuration and presets](docs/configuration.md)
- [Authentication](docs/authentication.md)
- [Using bam in CI](docs/ci.md)
- [JSON output](docs/json.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Development setup](SETUP.md) — Go, make and linter on macOS and Linux

## License

MIT
