## bam target add

Generate a run preset from a plan's declared variables

```
bam target add <name> --plan KEY [flags]
```

### Options

```
      --branch string   default branch for the preset
      --force           replace an existing target
      --from string     build whose values fill the comments: number, key or last
  -h, --help            help for add
      --machine         write to the machine config instead of .bam.yaml
      --plan string     plan key to generate from
      --print           print the YAML and write nothing
```

### Options inherited from parent commands

```
      --color string    color output: auto, always or never
      --debug           log HTTP requests to stderr
      --json            print JSON
      --no-tui          never open the terminal UI
      --server string   server alias to use
```

### SEE ALSO

* [bam target](bam_target.md)	 - Run presets: list, show, add, sync

