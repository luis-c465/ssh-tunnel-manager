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

var AddConfigurationsCmd = &cobra.Command{
	Use:     "add",
	Aliases: []string{"a"},
	Short:   "Add a new SSH tunnel configuration",
	Long: `
Add a new SSH tunnel configuration using an interactive form.

This command launches an interactive GUI form that allows you to enter details for a new SSH tunnel configuration, such as the SSH server address, local and remote ports, and any other required tunnel parameters. Once the form is submitted, the configuration is saved, enabling you to use this setup for future SSH tunnel initiations.

The form supports mouse interactions and clipboard operations, enhancing ease of use and efficiency. Use this command to configure new tunnels without manually editing configuration files or directly manipulating database entries.

Features:
- Interactive form with field validations to guide you through the setup process.
- Mouse and clipboard support for a better user experience.
- Immediate feedback on the success or failure of the configuration addition.

Examples:
- sshtm add
- sshtm a
	`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, cleanup, err := lib.CreateDaemonServiceClient()
		if err != nil {
			return err
		}
		defer cleanup()

		data := rpc.TunnelConfig{}

		app := tview.NewApplication()

		formConfig := lib.ConfigurationFormData{
			Title:             "Add configuration",
			PrimaryBtnLabel:   "Add",
			SecondaryBtnLabel: "Cancel",
		}

		var callbackErr error
		addForm := lib.ConfigurationForm(formConfig, app, &data, func(data *rpc.TunnelConfig) {
			addCtx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			r, err := c.AddConfiguration(addCtx, &rpc.AddOrUpdateConfigurationRequest{Name: data.Name, Data: data})
			if err != nil {
				callbackErr = fmt.Errorf("add configuration: %w", err)
				return
			}
			if r.GetStatus() == rpc.ResponseStatus_Error {
				callbackErr = fmt.Errorf("add configuration: %s", r.GetMessage())
				return
			}

			fmt.Print(formatters.NewMutationFormatter(os.Stdout).Format(formatters.MutationFromAddOrUpdate(r)))
		})

		if err := app.SetRoot(addForm, true).EnableMouse(true).EnablePaste(true).Run(); err != nil {
			return fmt.Errorf("run add configuration form: %w", err)
		}
		return callbackErr
	},
}
