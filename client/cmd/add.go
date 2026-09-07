package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/besrabasant/ssh-tunnel-manager/client/lib"
	"github.com/besrabasant/ssh-tunnel-manager/rpc"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
)

var AddConfigurationsCmd = &cobra.Command{
	Use:     "add",
	Aliases: []string{"a"},
	Short:   "Add a new SSH tunnel profile",
	Long: `
Add a tunnel profile using an interactive form.

The profile references an existing SSH machine and stores only its forwarding
settings. Create a machine with "sshtm machine add" before adding a profile.

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

		listCtx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		machinesResponse, err := c.ListMachines(listCtx, &rpc.ListMachinesRequest{})
		cancel()
		if err != nil {
			return fmt.Errorf("list machines: %w", err)
		}
		if machinesResponse == nil {
			return fmt.Errorf("list machines: empty response")
		}
		if machinesResponse.GetStatus() == rpc.ResponseStatus_Error {
			return responseError("list machines", machinesResponse.GetMessage())
		}
		machines := nonNilMachines(machinesResponse.GetMachines())
		if len(machines) == 0 {
			return fmt.Errorf("no machines available; run `sshtm machine add` first")
		}

		data := rpc.TunnelProfile{}
		app := tview.NewApplication()
		formConfig := lib.ConfigurationFormData{
			Title:             "Add tunnel profile",
			PrimaryBtnLabel:   "Add",
			SecondaryBtnLabel: "Cancel",
		}

		var callbackErr error
		addForm := lib.TunnelProfileForm(formConfig, app, &data, machines, func(data *rpc.TunnelProfile) {
			addCtx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			r, err := c.AddTunnelProfile(addCtx, &rpc.TunnelProfileMutationRequest{Data: data})
			if err != nil {
				callbackErr = fmt.Errorf("add tunnel profile: %w", err)
				return
			}
			if r == nil {
				callbackErr = fmt.Errorf("add tunnel profile: empty response")
				return
			}
			if r.GetStatus() == rpc.ResponseStatus_Error {
				callbackErr = responseError("add tunnel profile", r.GetMessage())
				return
			}
			if r.GetMessage() != "" {
				fmt.Fprintln(cmd.OutOrStdout(), r.GetMessage())
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Tunnel profile %q added.\n", data.GetName())
			}
		})

		if err := app.SetRoot(addForm, true).EnableMouse(true).EnablePaste(true).Run(); err != nil {
			return fmt.Errorf("run add tunnel profile form: %w", err)
		}
		return callbackErr
	},
}
