package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/besrabasant/ssh-tunnel-manager/client/lib"
	"github.com/besrabasant/ssh-tunnel-manager/rpc"
	"github.com/spf13/cobra"
)

const machineCommandTimeout = 10 * time.Second

// MachineCmd manages the SSH machines shared by tunnel profiles.
var MachineCmd = newMachineCommand()

func newMachineCommand() *cobra.Command {
	machineCmd := &cobra.Command{
		Use:   "machine",
		Short: "Manage SSH machines",
		Args:  cobra.NoArgs,
	}

	machineCmd.AddCommand(newMachineListCommand())
	machineCmd.AddCommand(newMachineAddCommand())
	machineCmd.AddCommand(newMachineEditCommand())
	machineCmd.AddCommand(newMachineDeleteCommand())
	return machineCmd
}

func newMachineListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List SSH machines",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, cleanup, err := lib.CreateDaemonServiceClient()
			if err != nil {
				return fmt.Errorf("connect to daemon: %w", err)
			}
			defer cleanup()

			machines, err := listMachines(cmd.Context(), client)
			if err != nil {
				return err
			}
			if len(machines) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No machines configured.")
				return nil
			}

			for _, machine := range machines {
				if machine == nil {
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", machine.GetName(), machine.GetServer(), machine.GetUser(), machine.GetKeyFile())
			}
			return nil
		},
	}
}

func newMachineAddCommand() *cobra.Command {
	var server, user, keyFile string
	command := &cobra.Command{
		Use:   "add <name>",
		Short: "Add an SSH machine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(server) == "" || strings.TrimSpace(user) == "" || strings.TrimSpace(keyFile) == "" {
				return fmt.Errorf("--server, --user, and --key-file are required")
			}

			client, cleanup, err := lib.CreateDaemonServiceClient()
			if err != nil {
				return fmt.Errorf("connect to daemon: %w", err)
			}
			defer cleanup()

			ctx, cancel := context.WithTimeout(cmd.Context(), machineCommandTimeout)
			defer cancel()
			response, err := client.AddMachine(ctx, &rpc.MachineMutationRequest{Data: &rpc.Machine{
				Name: args[0], Server: server, User: user, KeyFile: keyFile,
			}})
			if err != nil {
				return fmt.Errorf("add machine: %w", err)
			}
			if err := checkMachineMutation("add machine", response); err != nil {
				return err
			}
			writeMutationMessage(cmd, response.GetMessage(), fmt.Sprintf("Machine %q added.", args[0]))
			return nil
		},
	}
	command.Flags().StringVar(&server, "server", "", "SSH server address")
	command.Flags().StringVar(&user, "user", "", "SSH user")
	command.Flags().StringVar(&keyFile, "key-file", "", "SSH private key file")
	_ = command.MarkFlagRequired("server")
	_ = command.MarkFlagRequired("user")
	_ = command.MarkFlagRequired("key-file")
	return command
}

func newMachineEditCommand() *cobra.Command {
	var name, server, user, keyFile string
	command := &cobra.Command{
		Use:   "edit <name>",
		Short: "Edit an SSH machine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cleanup, err := lib.CreateDaemonServiceClient()
			if err != nil {
				return fmt.Errorf("connect to daemon: %w", err)
			}
			defer cleanup()

			machine, err := findMachine(cmd.Context(), client, args[0])
			if err != nil {
				return err
			}
			data := &rpc.Machine{
				Id:      machine.GetId(),
				Name:    machine.GetName(),
				Server:  machine.GetServer(),
				User:    machine.GetUser(),
				KeyFile: machine.GetKeyFile(),
			}
			if cmd.Flags().Changed("name") {
				data.Name = name
			}
			if cmd.Flags().Changed("server") {
				data.Server = server
			}
			if cmd.Flags().Changed("user") {
				data.User = user
			}
			if cmd.Flags().Changed("key-file") {
				data.KeyFile = keyFile
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), machineCommandTimeout)
			defer cancel()
			response, err := client.UpdateMachine(ctx, &rpc.MachineMutationRequest{Id: machine.GetId(), Data: data})
			if err != nil {
				return fmt.Errorf("update machine: %w", err)
			}
			if err := checkMachineMutation("update machine", response); err != nil {
				return err
			}
			writeMutationMessage(cmd, response.GetMessage(), fmt.Sprintf("Machine %q updated.", data.GetName()))
			return nil
		},
	}
	command.Flags().StringVar(&name, "name", "", "New display name")
	command.Flags().StringVar(&server, "server", "", "SSH server address")
	command.Flags().StringVar(&user, "user", "", "SSH user")
	command.Flags().StringVar(&keyFile, "key-file", "", "SSH private key file")
	return command
}

func newMachineDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete an SSH machine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cleanup, err := lib.CreateDaemonServiceClient()
			if err != nil {
				return fmt.Errorf("connect to daemon: %w", err)
			}
			defer cleanup()

			machine, err := findMachine(cmd.Context(), client, args[0])
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), machineCommandTimeout)
			defer cancel()
			response, err := client.DeleteMachine(ctx, &rpc.DeleteMachineRequest{Id: machine.GetId()})
			if err != nil {
				return fmt.Errorf("delete machine: %w", err)
			}
			if err := checkMachineMutation("delete machine", response); err != nil {
				return err
			}
			writeMutationMessage(cmd, response.GetMessage(), fmt.Sprintf("Machine %q deleted.", machine.GetName()))
			return nil
		},
	}
}

func listMachines(parent context.Context, client rpc.DaemonServiceClient) ([]*rpc.Machine, error) {
	ctx, cancel := context.WithTimeout(parent, machineCommandTimeout)
	defer cancel()
	response, err := client.ListMachines(ctx, &rpc.ListMachinesRequest{})
	if err != nil {
		return nil, fmt.Errorf("list machines: %w", err)
	}
	if response == nil {
		return nil, fmt.Errorf("list machines: empty response")
	}
	if response.GetStatus() == rpc.ResponseStatus_Error {
		return nil, responseError("list machines", response.GetMessage())
	}
	return nonNilMachines(response.GetMachines()), nil
}

func nonNilMachines(machines []*rpc.Machine) []*rpc.Machine {
	result := make([]*rpc.Machine, 0, len(machines))
	for _, machine := range machines {
		if machine != nil {
			result = append(result, machine)
		}
	}
	return result
}

func findMachine(ctx context.Context, client rpc.DaemonServiceClient, name string) (*rpc.Machine, error) {
	machines, err := listMachines(ctx, client)
	if err != nil {
		return nil, err
	}
	for _, machine := range machines {
		if machine != nil && machine.GetName() == name {
			if machine.GetId() == "" {
				return nil, fmt.Errorf("machine %q has no ID", name)
			}
			return machine, nil
		}
	}
	return nil, fmt.Errorf("machine %q not found", name)
}

func checkMachineMutation(operation string, response *rpc.MachineMutationResponse) error {
	if response == nil {
		return fmt.Errorf("%s: empty response", operation)
	}
	if response.GetStatus() == rpc.ResponseStatus_Error {
		return responseError(operation, response.GetMessage())
	}
	return nil
}

func responseError(operation, message string) error {
	if message == "" {
		return fmt.Errorf("%s failed", operation)
	}
	return fmt.Errorf("%s: %s", operation, message)
}

func writeMutationMessage(cmd *cobra.Command, message, fallback string) {
	if message == "" {
		message = fallback
	}
	fmt.Fprintln(cmd.OutOrStdout(), message)
}
