package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/besrabasant/ssh-tunnel-manager/client/formatters"
	"github.com/besrabasant/ssh-tunnel-manager/client/lib"
	"github.com/besrabasant/ssh-tunnel-manager/rpc"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
)

var EditConfigurationsCmd = &cobra.Command{
	Use:     "edit <configuration name>",
	Aliases: []string{"e"},
	Short:   "Edit an existing SSH tunnel configuration.",
	Long: `
Edit an existing SSH tunnel configuration interactively.

Use this command to modify the details of a saved SSH tunnel configuration, such as changing the local or remote ports, the SSH server, or any other parameter defined in the configuration. This command provides an interactive interface where you can select the configuration you wish to edit and make changes as required.

The command requires the name of the configuration as an argument. If the configuration name is not provided or is incorrect, the command will prompt for the correct name. After selecting a configuration, you will be guided through a series of prompts to update the desired fields.

Example Usage:
- sshtm edit my_configuration
- sshtm e my_configuration
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		configName := args[0]

		c, cleanup, err := lib.CreateDaemonServiceClient()
		if err != nil {
			return err
		}
		defer cleanup()

		ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		defer cancel()

		r, err := c.FetchConfiguration(ctx, &rpc.FetchConfigurationRequest{Name: configName})
		if err != nil {
			return fmt.Errorf("fetch configuration: %w", err)
		}
		if r.GetStatus() == rpc.ResponseStatus_Error {
			return fmt.Errorf("fetch configuration: %s", r.GetMessage())
		}

		data := r.GetData()

		cancel()

		app := tview.NewApplication()

		formConfig := lib.ConfigurationFormData{
			Title:             "Edit configuration",
			PrimaryBtnLabel:   "Update",
			SecondaryBtnLabel: "Cancel",
		}

		var callbackErr error
		editForm := lib.ConfigurationForm(formConfig, app, data, func(data *rpc.TunnelConfig) {
			updateCtx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			r, err := c.UpdateConfiguration(updateCtx, &rpc.AddOrUpdateConfigurationRequest{Name: configName, Data: data})
			if err != nil {
				callbackErr = fmt.Errorf("update configuration: %w", err)
				return
			}
			if r.GetStatus() == rpc.ResponseStatus_Error {
				callbackErr = fmt.Errorf("update configuration: %s", r.GetMessage())
				return
			}

			fmt.Print(formatters.NewMutationFormatter(os.Stdout).Format(formatters.MutationFromAddOrUpdate(r)))
		})

		if err := app.SetRoot(editForm, true).EnableMouse(true).EnablePaste(true).Run(); err != nil {
			return fmt.Errorf("run edit configuration form: %w", err)
		}
		return callbackErr
	},
}
