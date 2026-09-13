# Authentication

bam uses Bamboo personal access tokens (Bamboo Data Center 9.x+).

1. In Bamboo, open your profile → Personal access tokens, and create a token.
2. Run `bam login <alias>` and paste it at the hidden prompt, or pipe it: `printf %s "$TOKEN" | bam login work --with-token`.

bam checks the token before storing it and prints your user name.

## Where tokens live

Tokens are stored per server URL (scheme, host and port), never per alias, so a token is only ever sent to the host it was created for. A repository whose `.bam.yaml` points an alias at a different host needs its own `bam login`.

Lookup order:

1. the environment variable named by the server's `auth_env`
2. the OS keychain (macOS Keychain, Linux Secret Service, Windows Credential Manager)
3. `~/.config/bam/credentials.yaml`, which must be mode 0600; bam uses it when no keychain is available

`bam logout <alias>` removes the stored token. `bam whoami` shows who you are on each server.

## Single sign-on

If your Bamboo uses SSO, password logins usually fail but personal access tokens work. If your instance does not allow creating tokens, ask your Bamboo administrator; bam cannot work without one.
