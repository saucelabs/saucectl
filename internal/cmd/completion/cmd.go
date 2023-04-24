package completion

import (
	"os"

	"github.com/spf13/cobra"
)

// Command creates the `completion` command
func Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate completion script",
		Long: `To load completions:

Bash:

  $ source <(saucectl completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ saucectl completion bash > /etc/bash_completion.d/saucectl
  # macOS:
  $ saucectl completion bash > /usr/local/etc/bash_completion.d/saucectl

Zsh:

  # If shell completion is not already enabled in your environment,
  # enable it by executing the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ saucectl completion zsh > "${fpath[1]}/_saucectl"

  # Start a new shell to apply this setup.

fish:

  $ saucectl completion fish | source

  # To load completions for each session, execute once:
  $ saucectl completion fish > ~/.config/fish/completions/saucectl.fish

PowerShell:

  PS> saucectl completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, 
  # run the following and then source this file from your Powershell profile:
  PS> saucectl completion powershell > saucectl.ps1
`,
		DisableFlagsInUseLine: true,
		SilenceUsage:          true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.ExactValidArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "bash":
				_ = cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				_ = cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				_ = cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				_ = cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			}
		},
	}

	return cmd
}
