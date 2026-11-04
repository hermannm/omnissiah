package main

import (
	"context"

	"hermannm.dev/wrap"

	"hermannm.dev/omnissiah/sbx"
	"hermannm.dev/omnissiah/tui"
)

func commune(ctx context.Context) error {
	machineSpirits, err := sbx.List(ctx)
	if err != nil {
		return wrap.Error(err, "failed to list machine spirits")
	}

	sortSpirits(machineSpirits, sortArgs{runningFirst: true, newestFirst: true})

	if err := tui.RenderMachineSpiritPicker(
		ctx,
		machineSpirits,
		[]tui.PickerAction{
			pickerActionCommune,
			pickerActionWake,
			pickerActionSleep,
			pickerActionKill,
			pickerActionExterminatus,
		},
		pickerActionCommune.Name,
	); err != nil {
		return err
	}
	return nil
}

var (
	pickerActionCommune = tui.PickerAction{
		Name: "commune",
		Key:  "c",
		Action: func(ctx context.Context, spirit sbx.Sandbox) (sbx.Sandbox, error) {
			if err := sbx.RunExisting(ctx, spirit.Name); err != nil {
				return sbx.Sandbox{}, err
			}
			spirit.Status = sbx.StatusRunning
			return spirit, nil
		},
	}
	pickerActionWake = tui.PickerAction{
		Name:       "wake",
		Key:        "w",
		Background: "Waking...",
		Action: func(ctx context.Context, spirit sbx.Sandbox) (sbx.Sandbox, error) {
			if _, err := sbx.RunExistingDetached(ctx, spirit.Name); err != nil {
				return sbx.Sandbox{}, err
			}
			spirit.Status = sbx.StatusRunning
			return spirit, nil
		},
	}
	pickerActionSleep = tui.PickerAction{
		Name:       "sleep",
		Key:        "s",
		Background: "Putting to sleep...",
		Action: func(ctx context.Context, spirit sbx.Sandbox) (sbx.Sandbox, error) {
			if _, err := sbx.Stop(ctx, spirit.Name); err != nil {
				return sbx.Sandbox{}, err
			}
			spirit.Status = sbx.StatusStopped
			return spirit, nil
		},
	}
	pickerActionKill = tui.PickerAction{
		Name:         "kill",
		Key:          "k",
		Confirmation: "Kill %s?",
		Background:   "Killing...",
		Action: func(ctx context.Context, spirit sbx.Sandbox) (sbx.Sandbox, error) {
			if _, err := sbx.Remove(ctx, spirit.Name); err != nil {
				return sbx.Sandbox{}, err
			}
			return spirit, tui.ErrRemovePickerItem
		},
	}
	pickerActionExterminatus = tui.PickerAction{
		Name:         "exterminatus",
		Key:          "e",
		Confirmation: "Commence EXTERMINATUS?",
		Background:   "Commencing EXTERMINATUS...",
		Action: func(ctx context.Context, spirit sbx.Sandbox) (sbx.Sandbox, error) {
			if _, err := sbx.RemoveAll(ctx); err != nil {
				return sbx.Sandbox{}, err
			}
			return spirit, tui.ErrRemoveAllPickerItems
		},
	}
)
