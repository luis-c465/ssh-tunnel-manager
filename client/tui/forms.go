package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/besrabasant/ssh-tunnel-manager/pkg/configmanager"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func showAddForm(s *State) {
	if len(s.Machines) == 0 {
		showError(s, fmt.Errorf("add a machine first (press m) before creating a tunnel profile"))
		return
	}
	showProfileForm(s, configmanager.Entry{}, false)
}

func showEditForm(s *State, entry configmanager.Entry) {
	if len(s.Machines) == 0 {
		showError(s, fmt.Errorf("no machines are available; add a machine before editing this tunnel profile"))
		return
	}
	showProfileForm(s, entry, true)
}

func showProfileForm(s *State, initial configmanager.Entry, editing bool) {
	form, collect := buildProfileForm(initial, s.Machines, editing)
	form.AddButton("Save", func() {
		profile, err := collect()
		if err != nil {
			showError(s, err)
			return
		}
		if editing {
			err = UpdateTunnelProfile(profile)
		} else {
			err = AddTunnelProfile(profile)
		}
		if err != nil {
			showError(s, err)
			return
		}
		if err := s.ReloadData(); err != nil {
			showError(s, err)
			return
		}
		populateList(s)
		selectByName(s, profile.Name)
		updateStatus(s)
		s.Pages.RemovePage("modal")
	})
	form.AddButton("Cancel", func() { s.Pages.RemovePage("modal") })
	form.SetButtonsAlign(tview.AlignCenter)
	if editing {
		form.SetBorder(true).SetTitle(" Edit tunnel profile ")
	} else {
		form.SetBorder(true).SetTitle(" Add tunnel profile ")
	}
	addModalPage(s, form)
}

func buildProfileForm(initial configmanager.Entry, machines []configmanager.Machine, editing bool) (*tview.Form, func() (configmanager.TunnelProfile, error)) {
	form := tview.NewForm()
	form.SetItemPadding(1)

	name := tview.NewInputField().SetLabel("Name").SetText(initial.Name)
	if editing {
		name.SetDisabled(true)
		name.SetLabelColor(tcell.ColorYellow)
		name.SetFieldTextColor(tcell.ColorGray)
	}
	description := tview.NewInputField().SetLabel("Description").SetText(strings.TrimSpace(initial.Description))

	machines = sortedMachines(machines)
	options := make([]string, len(machines))
	selected := 0
	for i, machine := range machines {
		options[i] = machine.Name
		if machine.ID == initial.MachineID {
			selected = i
		}
	}
	machine := tview.NewDropDown().SetLabel("Machine")
	machine.SetOptions(options, nil)
	machine.SetCurrentOption(selected)

	remoteHost := tview.NewInputField().SetLabel("RemoteHost").SetText(initial.RemoteHost)
	remotePort := tview.NewInputField().SetLabel("RemotePort").SetText(intToStr(initial.RemotePort))
	localPort := tview.NewInputField().SetLabel("LocalPort (0=auto)").SetText(intToStr(initial.LocalPort))
	for _, item := range []tview.FormItem{name, description, machine, remoteHost, remotePort, localPort} {
		form.AddFormItem(item)
	}
	configureFormNavigation(form)

	collect := func() (configmanager.TunnelProfile, error) {
		if len(machines) == 0 {
			return configmanager.TunnelProfile{}, fmt.Errorf("at least one machine is required")
		}
		machineIndex, _ := machine.GetCurrentOption()
		if machineIndex < 0 || machineIndex >= len(machines) {
			return configmanager.TunnelProfile{}, fmt.Errorf("select a machine")
		}
		remotePortValue, err := strconv.Atoi(strings.TrimSpace(remotePort.GetText()))
		if err != nil {
			return configmanager.TunnelProfile{}, fmt.Errorf("remote port must be a number")
		}
		localPortValue := 0
		if value := strings.TrimSpace(localPort.GetText()); value != "" {
			localPortValue, err = strconv.Atoi(value)
			if err != nil {
				return configmanager.TunnelProfile{}, fmt.Errorf("local port must be a number")
			}
		}
		profile := configmanager.TunnelProfile{
			ID:          initial.ID,
			Name:        strings.TrimSpace(name.GetText()),
			Description: strings.TrimSpace(description.GetText()),
			MachineID:   machines[machineIndex].ID,
			RemoteHost:  strings.TrimSpace(remoteHost.GetText()),
			RemotePort:  remotePortValue,
			LocalPort:   localPortValue,
		}
		return profile, profile.Validate()
	}
	return form, collect
}

func showDeleteConfirm(s *State, entry configmanager.Entry) {
	modal := tview.NewModal().SetText(fmt.Sprintf("Delete tunnel profile '%s'?", entry.Name)).AddButtons([]string{"Yes", "No"}).SetDoneFunc(func(_ int, label string) {
		if label != "Yes" {
			s.Pages.RemovePage("modal")
			return
		}
		if err := DeleteTunnelProfile(entry.ID); err != nil {
			showError(s, err)
			return
		}
		if err := s.ReloadData(); err != nil {
			showError(s, err)
			return
		}
		populateList(s)
		updateStatus(s)
		s.Pages.RemovePage("modal")
	})
	addModalPage(s, modal)
}

func showMachineManager(s *State) {
	machines := sortedMachines(s.Machines)
	list := tview.NewList().ShowSecondaryText(true)
	list.SetBorder(true).SetTitle(" Machines (a add, e edit, d delete, Esc close) ")
	for _, machine := range machines {
		list.AddItem(machine.Name, fmt.Sprintf("%s as %s", machine.Server, machine.User), 0, nil)
	}
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			s.Pages.RemovePage("modal")
			return nil
		}
		switch event.Rune() {
		case 'a':
			showMachineForm(s, configmanager.Machine{}, false)
			return nil
		case 'e':
			if index := list.GetCurrentItem(); index >= 0 && index < len(machines) {
				showMachineForm(s, machines[index], true)
			}
			return nil
		case 'd':
			if index := list.GetCurrentItem(); index >= 0 && index < len(machines) {
				showMachineDeleteConfirm(s, machines[index])
			}
			return nil
		case 'q':
			s.Pages.RemovePage("modal")
			return nil
		}
		return event
	})
	addModalPage(s, list)
}

func showMachineForm(s *State, initial configmanager.Machine, editing bool) {
	form, collect := buildMachineForm(initial)
	form.AddButton("Save", func() {
		machine, err := collect()
		if err != nil {
			showError(s, err)
			return
		}
		if editing {
			err = UpdateMachine(machine)
		} else {
			err = AddMachine(machine)
		}
		if err != nil {
			showError(s, err)
			return
		}
		if err := s.ReloadData(); err != nil {
			showError(s, err)
			return
		}
		populateList(s)
		showMachineManager(s)
	})
	form.AddButton("Cancel", func() { showMachineManager(s) })
	form.SetButtonsAlign(tview.AlignCenter)
	if editing {
		form.SetBorder(true).SetTitle(" Edit machine ")
	} else {
		form.SetBorder(true).SetTitle(" Add machine ")
	}
	addModalPage(s, form)
}

func buildMachineForm(initial configmanager.Machine) (*tview.Form, func() (configmanager.Machine, error)) {
	form := tview.NewForm()
	form.SetItemPadding(1)
	name := tview.NewInputField().SetLabel("Name").SetText(initial.Name)
	server := tview.NewInputField().SetLabel("Server (host[:port])").SetText(initial.Server)
	user := tview.NewInputField().SetLabel("User").SetText(initial.User)
	keyFile := tview.NewInputField().SetLabel("KeyFile").SetText(initial.KeyFile)
	for _, item := range []tview.FormItem{name, server, user, keyFile} {
		form.AddFormItem(item)
	}
	configureFormNavigation(form)

	return form, func() (configmanager.Machine, error) {
		machine := configmanager.Machine{
			ID:      initial.ID,
			Name:    strings.TrimSpace(name.GetText()),
			Server:  strings.TrimSpace(server.GetText()),
			User:    strings.TrimSpace(user.GetText()),
			KeyFile: strings.TrimSpace(keyFile.GetText()),
		}
		return machine, machine.Validate()
	}
}

func showMachineDeleteConfirm(s *State, machine configmanager.Machine) {
	modal := tview.NewModal().SetText(fmt.Sprintf("Delete machine '%s'?", machine.Name)).AddButtons([]string{"Yes", "No"}).SetDoneFunc(func(_ int, label string) {
		if label != "Yes" {
			showMachineManager(s)
			return
		}
		if err := DeleteMachine(machine.ID); err != nil {
			showError(s, err)
			return
		}
		if err := s.ReloadData(); err != nil {
			showError(s, err)
			return
		}
		populateList(s)
		showMachineManager(s)
	})
	addModalPage(s, modal)
}

func configureFormNavigation(form *tview.Form) {
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp, tcell.KeyPgUp:
			return tcell.NewEventKey(tcell.KeyBacktab, 0, event.Modifiers())
		case tcell.KeyDown, tcell.KeyPgDn:
			return tcell.NewEventKey(tcell.KeyTab, 0, event.Modifiers())
		}
		return event
	})
}

func showError(s *State, err error) {
	modal := tview.NewModal().SetText(fmt.Sprintf("Error: %v", err)).AddButtons([]string{"OK"}).SetDoneFunc(func(int, string) { s.Pages.RemovePage("modal") })
	addModalPage(s, modal)
}

func addModalPage(s *State, primitive tview.Primitive) {
	col := tview.NewFlex().SetDirection(tview.FlexColumn)
	col.AddItem(nil, 0, 1, false)
	col.AddItem(primitive, 0, 2, true)
	col.AddItem(nil, 0, 1, false)

	wrapper := tview.NewFlex().SetDirection(tview.FlexRow)
	wrapper.AddItem(nil, 0, 1, false)
	wrapper.AddItem(col, 0, 1, true)
	wrapper.AddItem(nil, 0, 1, false)
	wrapper.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			s.Pages.RemovePage("modal")
			return nil
		}
		return event
	})

	s.Pages.AddPage("modal", wrapper, true, true)
	s.App.SetFocus(primitive)
}

func selectByName(s *State, name string) {
	for index, entry := range s.Filtered() {
		if entry.Name == name && index < s.List.GetItemCount() {
			s.List.SetCurrentItem(index)
			return
		}
	}
}

func sortedMachines(machines []configmanager.Machine) []configmanager.Machine {
	out := append([]configmanager.Machine(nil), machines...)
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func intToStr(value int) string {
	if value == 0 {
		return ""
	}
	return strconv.Itoa(value)
}
