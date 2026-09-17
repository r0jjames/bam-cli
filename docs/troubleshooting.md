# Troubleshooting

Start with `bam doctor`. It checks your config files, which server is selected, where the token comes from, whether Bamboo accepts it, and which optional REST features your Bamboo supports.

| Symptom | Fix |
| --- | --- |
| `no token for server "work"` | `bam login work` |
| `401 from bamboo.example.com` | the token expired or was revoked: create a new one and `bam login work` |
| `403 from …` | your user lacks permission on that project or plan |
| `cannot reach bamboo.example.com` | check the URL, your network and VPN |
| `credentials.yaml is readable by other users` | `chmod 600 ~/.config/bam/credentials.yaml` |
| no keychain on Linux | bam stores the token in `credentials.yaml` (0600) and says so |
| `plan variables cannot be listed` | your Bamboo version lacks the endpoint; bam uses past builds and target defaults instead |
| `cannot reuse variables of …` | your Bamboo does not return a build's variables; pass them with `--var` |
| colors in a log file | colors are off on pipes; force with `--color=never` or `NO_COLOR=1` |

`--debug` prints every HTTP request (method, URL, status, duration) to stderr. Tokens and secret variable values are never printed.
