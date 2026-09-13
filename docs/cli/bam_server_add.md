## bam server add

Add a server alias to the machine config, then log in

```
bam server add <alias> --url URL [flags]
```

### Options

```
      --force             replace an existing alias
  -h, --help              help for add
      --project strings   project key to scope navigation (repeatable)
      --url string        server URL, e.g. https://bamboo.example.com
```

### Options inherited from parent commands

```
      --color string    color output: auto, always or never
      --debug           log HTTP requests to stderr
      --json            print JSON
      --server string   server alias to use
```

### SEE ALSO

* [bam server](bam_server.md)	 - Manage Bamboo server aliases

