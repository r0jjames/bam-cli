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
