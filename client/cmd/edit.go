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

var EditConfigurationsCmd = &cobra.Command{
	Use:     "edit <profile name>",
	Aliases: []string{"e"},
	Short:   "Edit an existing SSH tunnel profile.",
	Long: `
Edit an existing tunnel profile interactively.

The form edits forwarding settings and lets you select the reusable SSH machine.
Manage SSH connection details with "sshtm machine edit".

Examples:
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

		profilesCtx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		profilesResponse, err := c.ListTunnelProfiles(profilesCtx, &rpc.ListTunnelProfilesRequest{})
		cancel()
		if err != nil {
			return fmt.Errorf("list tunnel profiles: %w", err)
		}
		if profilesResponse == nil {
			return fmt.Errorf("list tunnel profiles: empty response")
		}
		if profilesResponse.GetStatus() == rpc.ResponseStatus_Error {
			return responseError("list tunnel profiles", profilesResponse.GetMessage())
		}

		var data *rpc.TunnelProfile
		for _, profile := range profilesResponse.GetTunnelProfiles() {
			if profile != nil && profile.GetName() == configName {
				data = profile
				break
			}
		}
		if data == nil {
			return fmt.Errorf("tunnel profile %q not found", configName)
		}
		if data.GetId() == "" {
			return fmt.Errorf("tunnel profile %q has no ID", configName)
		}

		machinesCtx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		machinesResponse, err := c.ListMachines(machinesCtx, &rpc.ListMachinesRequest{})
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

		app := tview.NewApplication()
		formConfig := lib.ConfigurationFormData{
			Title:             "Edit tunnel profile",
			PrimaryBtnLabel:   "Update",
			SecondaryBtnLabel: "Cancel",
			DisableName:       true,
		}

		var callbackErr error
		editForm := lib.TunnelProfileForm(formConfig, app, data, machines, func(data *rpc.TunnelProfile) {
			updateCtx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			r, err := c.UpdateTunnelProfile(updateCtx, &rpc.TunnelProfileMutationRequest{Id: data.GetId(), Data: data})
			if err != nil {
				callbackErr = fmt.Errorf("update tunnel profile: %w", err)
				return
			}
			if r == nil {
				callbackErr = fmt.Errorf("update tunnel profile: empty response")
				return
			}
			if r.GetStatus() == rpc.ResponseStatus_Error {
				callbackErr = responseError("update tunnel profile", r.GetMessage())
				return
			}
			if r.GetMessage() != "" {
				fmt.Fprintln(cmd.OutOrStdout(), r.GetMessage())
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Tunnel profile %q updated.\n", data.GetName())
			}
		})

		if err := app.SetRoot(editForm, true).EnableMouse(true).EnablePaste(true).Run(); err != nil {
			return fmt.Errorf("run edit tunnel profile form: %w", err)
		}
		return callbackErr
	},
}
