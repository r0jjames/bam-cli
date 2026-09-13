## bam build cancel

Stop a queued or running build

```
bam build cancel [<build>|<plan>|<target>] [flags]
```

### Options

```
      --branch string   plan branch when a plan or target is given
  -h, --help            help for cancel
      --last            the last build bam triggered from this repository
```

### Options inherited from parent commands

```
      --color string    color output: auto, always or never
      --debug           log HTTP requests to stderr
      --json            print JSON
      --server string   server alias to use
```

### SEE ALSO

* [bam build](bam_build.md)	 - Builds: list, show, run, watch, logs, cancel

