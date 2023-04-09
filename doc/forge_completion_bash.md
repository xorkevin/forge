## forge completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(forge completion bash)

To load completions for every new session, execute once:

#### Linux:

	forge completion bash > /etc/bash_completion.d/forge

#### macOS:

	forge completion bash > $(brew --prefix)/etc/bash_completion.d/forge

You will need to start a new shell for this setup to take effect.


```
forge completion bash
```

### Options

```
  -h, --help              help for bash
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --log-json           output json logs
      --log-level string   log level (default "info")
```

### SEE ALSO

* [forge completion](forge_completion.md)	 - Generate the autocompletion script for the specified shell

