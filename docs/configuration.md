# Configuration

bam reads three layers. Only the third is secret.

| Layer | File | Commit it? | Holds |
| --- | --- | --- | --- |
| Project | `.bam.yaml` at the repository root | yes | servers, projects, targets |
| Machine | `~/.config/bam/config.yaml` (`BAM_CONFIG` overrides) | no | your own servers, overrides, personal targets |
| Credentials | OS keychain, an env variable, or `~/.config/bam/credentials.yaml` (0600) | never | tokens only |

bam looks for `.bam.yaml` from the current directory up to the git root (or your home directory outside a repository).

## Project file

    version: 1
    servers:
      work:
        url: https://bamboo.example.com
    default_server: work
    projects: [PROJ, OPS]
    targets:
      provision-lab:
        plan: PROJ-PROV
        branch: develop
        defaults:
          cluster_type: k8s
          db_password: "${LAB_DB_PASSWORD}"
        options:
          cluster_type: [k8s, dcos]
        required: [cluster_name]
        watch: true
        timeout: 45m

A token, or a literal value for a variable named like `password`, `secret`, `passphrase` or `sshkey`, is an error in this file. Use `"${ENV_NAME}"` for secret values; bam reads the variable when it runs.

The server alias `env` is reserved for the server defined by `BAM_URL` and is rejected if a config file defines a server by that name.

## Targets (run presets)

| Field | Meaning |
| --- | --- |
| `plan` | plan key (required) |
| `server` | server alias, if not the default |
| `branch` | default plan branch; `--branch` overrides |
| `defaults` | variable values; `--from` and `--var` override |
| `options` | allowed values per variable; anything else is refused before triggering |
| `required` | variables that must have a value |
| `watch` | watch by default; `--watch=false` overrides |
| `timeout` | default `--timeout` |

Generate a target from a plan's declared variables instead of typing it:

    bam target add provision-lab --plan PROJ-PROV --branch develop
    bam target add mine --plan PROJ-PROV --machine     # personal, not committed
    bam target add x --plan PROJ-PROV --print          # show only

Variable precedence, lowest to highest: plan values, target `defaults`, `--from <build>`, `--var name=value`.

## Machine file

    version: 1
    default_server: home
    servers:
      home:
        url: http://bamboo.lab.example:8085
        projects: [LAB]
      work:
        url: https://bamboo-eu.example.com   # overrides the project file on this machine
        auth_env: BAM_WORK_TOKEN
    targets:
      lab-smoke:
        server: home
        plan: LAB-SMOKE
    repos:
      /home/jdoe/src/some-repo:
        server: work
        projects: [PROJ]
    color: auto
    pager: less -FRX

A server defined in both files is merged field by field, and the machine file wins.

## Which server is used

First match wins: `--server`, `BAM_SERVER`, `BAM_URL`, the target's `server`, the machine file's `repos` entry, the project's `default_server`, the machine's `default_server`, the only configured server.
