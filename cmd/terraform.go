package cmd

import (
	"github.com/spf13/cobra"
)

type terraformCommand struct {
	d         string
	transform bool
}

var terraformCmds = map[string]terraformCommand{
	"init": {
		d:         "Prepare your working directory for other commands",
		transform: false,
	},
	"plan": {
		d:         "Generates a plan based on the specified configuration",
		transform: true,
	},
	"apply": {
		d:         "Create or update infrastructure",
		transform: true,
	},
	"destroy": {
		d:         "Destroy previously-created infrastructure",
		transform: true,
	},
	"console": {
		d:         "Try Terraform expressions at an interactive command prompt",
		transform: true,
	},
	"validate": {
		d:         "Check whether the configuration is valid",
		transform: true,
	},
	"fmt": {
		d:         "Reformat your configuration in the standard style",
		transform: false,
	},
	"force-unlock": {
		d:         "Release a stuck lock on the current workspace",
		transform: true,
	},
	"get": {
		d:         "Install or upgrade remote Terraform modules",
		transform: false,
	},
	"graph": {
		d:         "Generate a Graphviz graph of the steps in an operation",
		transform: true,
	},
	"import": {
		d:         "Associate existing infrastructure with a Terraform resource",
		transform: true,
	},
	"login": {
		d:         "Obtain and save credentials for a remote host",
		transform: false,
	},
	"logout": {
		d:         "Remove locally-stored credentials for a remote host",
		transform: false,
	},
	"metadata": {
		d:         "Metadata related commands",
		transform: false,
	},
	"output": {
		d:         "Show output values from your root module",
		transform: true,
	},
	"providers": {
		d:         "Show the providers required for this configuration",
		transform: true,
	},
	"refresh": {
		d:         "Update the state to match remote systems",
		transform: true,
	},
	"show": {
		d:         "Show the current state or a saved plan",
		transform: true,
	},
	"state": {
		d:         "Advanced state management",
		transform: true,
	},
	"taint": {
		d:         "Mark a resource instance as not fully functional",
		transform: true,
	},
	"test": {
		d:         "Execute integration tests for Terraform modules",
		transform: true,
	},
	"untaint": {
		d:         "Remove the 'tainted' state from a resource instance",
		transform: true,
	},
	"version": {
		d:         "Show the current Terraform version",
		transform: false,
	},
	"workspace": {
		d:         "Workspace management",
		transform: false,
	},
}

var terraformCommands []*cobra.Command

func init() {
	for key, s := range terraformCmds {
		info := s
		cmd := key
		recursive := false
		run := wrapTerraformCommand(cf.tfDir, cmd)
		if info.transform {
			run = wrapTerraformCommandWithEphemeralTransform(cf.tfDir, cmd, &recursive)
		}
		c := &cobra.Command{
			Use:   cmd,
			Short: "[terraform]: " + info.d,
			FParseErrWhitelist: cobra.FParseErrWhitelist{
				UnknownFlags: true,
			},
			RunE: run,
		}

		c.Flags().BoolVarP(&recursive, "recursive", "r", false, "With transforms to all modules or not, default to the root module only.")
		rootCmd.AddCommand(c)
		terraformCommands = append(terraformCommands, c)
	}
}
