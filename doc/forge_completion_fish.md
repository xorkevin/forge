## forge completion fish

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	forge completion fish | source

To load completions for every new session, execute once:

	forge completion fish > ~/.config/fish/completions/forge.fish

You will need to start a new shell for this setup to take effect.


```
forge completion fish [flags]
```

### Options

```
  -h, --help              help for fish
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --config string   config file (default is $XDG_CONFIG_HOME/.forge.yaml)
      --debug           turn on debug output
```

### SEE ALSO

* [forge completion](forge_completion.md)	 - Generate the autocompletion script for the specified shell

