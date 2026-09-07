package lib

import (
	"strconv"

	"github.com/besrabasant/ssh-tunnel-manager/rpc"
	"github.com/besrabasant/ssh-tunnel-manager/utils"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ConfigurationFormData struct {
	Title             string
	PrimaryBtnLabel   string
	SecondaryBtnLabel string
	DisableName       bool
}

func ConfigurationForm(formConfig ConfigurationFormData, app *tview.Application, data *rpc.TunnelConfig, updateFn func(data *rpc.TunnelConfig)) *tview.Form {
	app.SetInputCapture(configurationFormInputCapture(app))

	defaultFieldWidth := 40
	form := tview.NewForm().
		SetFieldBackgroundColor(tcell.ColorDefault).
		SetItemPadding(1).
		AddInputField(
			"Name",
			data.Name,
			defaultFieldWidth,
			func(textToCheck string, lastChar rune) bool {
				return true
			},
			func(text string) {
				data.Name = text
			},
		).
		AddTextArea("Description", data.Description, 40, 0, 0, func(text string) {
			data.Description = text
		}).
		AddInputField(
			"SSH Server",
			data.Server,
			defaultFieldWidth,
			func(textToCheck string, lastChar rune) bool {
				return true
			},
			func(text string) {
				data.Server = text
			},
		).
		AddInputField(
			"SSH User",
			data.User,
			defaultFieldWidth,
			func(textToCheck string, lastChar rune) bool {
				return true
			},
			func(text string) {
				data.User = text
			},
		).
		AddInputField(
			"Key file Absolute Path",
			data.KeyFile,
			defaultFieldWidth,
			func(textToCheck string, lastChar rune) bool {
				return true
			},
			func(text string) {
				data.KeyFile = text
			},
		).
		AddInputField(
			"Remote Host",
			data.RemoteHost,
			defaultFieldWidth,
			func(textToCheck string, lastChar rune) bool {
				return true
			},
			func(text string) {
				data.RemoteHost = text
			},
		).
		AddInputField(
			"Remote Port",
			utils.IntToString(int(data.RemotePort)),
			defaultFieldWidth,
			func(textToCheck string, lastChar rune) bool {
				return true
			},
			func(text string) {
				port, err := strconv.Atoi(text)
				if err == nil {
					data.RemotePort = int32(port)
				}
			},
		).
		AddInputField(
			"Local Port",
			utils.IntToString(int(data.LocalPort)),
			defaultFieldWidth,
			func(textToCheck string, lastChar rune) bool {
				return true
			},
			func(text string) {
				port, err := strconv.Atoi(text)
				if err == nil {
					data.LocalPort = int32(port)
				}
			},
		).
		AddButton(formConfig.PrimaryBtnLabel, func() {
			app.Stop()

			updateFn(data)
		}).
		AddButton(formConfig.SecondaryBtnLabel, func() {
			app.Stop()
		})

	form.SetBorder(true).SetTitle(formConfig.Title).SetTitleAlign(tview.AlignLeft)
	return form
}

// TunnelProfileForm builds the interactive editor for a tunnel profile. Machines
// are selected by display name while the profile keeps the selected machine ID.
func TunnelProfileForm(formConfig ConfigurationFormData, app *tview.Application, data *rpc.TunnelProfile, machines []*rpc.Machine, updateFn func(data *rpc.TunnelProfile)) *tview.Form {
	app.SetInputCapture(configurationFormInputCapture(app))

	machineNames := make([]string, 0, len(machines))
	machineIDs := make([]string, 0, len(machines))
	selectedMachine := 0
	for _, machine := range machines {
		if machine == nil {
			continue
		}
		if machine.GetId() == data.GetMachineId() {
			selectedMachine = len(machineIDs)
		}
		machineNames = append(machineNames, machine.GetName())
		machineIDs = append(machineIDs, machine.GetId())
	}
	if len(machineIDs) > 0 {
		data.MachineId = machineIDs[selectedMachine]
	}

	const defaultFieldWidth = 40
	form := tview.NewForm().
		SetFieldBackgroundColor(tcell.ColorDefault).
		SetItemPadding(1).
		AddInputField("Name", data.GetName(), defaultFieldWidth, nil, func(text string) {
			data.Name = text
		}).
		AddTextArea("Description", data.GetDescription(), 40, 0, 0, func(text string) {
			data.Description = text
		}).
		AddDropDown("Machine", machineNames, selectedMachine, func(_ string, index int) {
			if index >= 0 && index < len(machineIDs) {
				data.MachineId = machineIDs[index]
			}
		}).
		AddInputField("Remote Host", data.GetRemoteHost(), defaultFieldWidth, nil, func(text string) {
			data.RemoteHost = text
		}).
		AddInputField("Remote Port", utils.IntToString(int(data.GetRemotePort())), defaultFieldWidth, nil, func(text string) {
			port, err := strconv.Atoi(text)
			if err != nil {
				data.RemotePort = 0
				return
			}
			data.RemotePort = int32(port)
		}).
		AddInputField("Local Port", utils.IntToString(int(data.GetLocalPort())), defaultFieldWidth, nil, func(text string) {
			if text == "" {
				data.LocalPort = 0
				return
			}
			port, err := strconv.Atoi(text)
			if err != nil {
				data.LocalPort = -1
				return
			}
			data.LocalPort = int32(port)
		}).
		AddButton(formConfig.PrimaryBtnLabel, func() {
			app.Stop()
			updateFn(data)
		}).
		AddButton(formConfig.SecondaryBtnLabel, func() {
			app.Stop()
		})

	if formConfig.DisableName {
		form.GetFormItemByLabel("Name").SetDisabled(true)
	}
	form.SetBorder(true).SetTitle(formConfig.Title).SetTitleAlign(tview.AlignLeft)
	return form
}

func configurationFormInputCapture(app *tview.Application) func(*tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			app.Stop()
			return nil
		}
		return event
	}
}
