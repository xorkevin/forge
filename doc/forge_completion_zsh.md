## forge completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(forge completion zsh)

To load completions for every new session, execute once:

#### Linux:

	forge completion zsh > "${fpath[1]}/_forge"

#### macOS:

	forge completion zsh > $(brew --prefix)/share/zsh/site-functions/_forge

You will need to start a new shell for this setup to take effect.


```
forge completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --log-json           output json logs
      --log-level string   log level (default "info")
```

### SEE ALSO

* [forge completion](forge_completion.md)	 - Generate the autocompletion script for the specified shell

