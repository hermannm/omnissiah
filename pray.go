package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"

	"hermannm.dev/wrap"

	"hermannm.dev/omnissiah/config"
	"hermannm.dev/omnissiah/internal/color"
	"hermannm.dev/omnissiah/sbx"
)

type prayArgs struct {
	name string
}

func pray(ctx context.Context, args prayArgs) error {
	cfg, err := config.Load()
	if err != nil {
		return wrap.Error(err, "failed to load Omnissiah config")
	}

	project, err := cfg.GetCurrentProject()
	if err != nil {
		return err
	}

	existingSpirits, err := sbx.List(ctx)
	if err != nil {
		return wrap.Error(err, "failed to list machine spirits for giving name to the new one")
	}

	if args.name == "" {
		// If no name is given: Get random name from [machineSpiritNames]
		args.name, err = getRandomNameForMachineSpirit(existingSpirits, defaultRand())
		if err != nil {
			return wrap.Error(err, "failed to get random name for machine spirit")
		}
	} else {
		// If name is given: check that it's unique (otherwise sbx reattaches to existing sandbox,
		// which we reserve for the `commune` command)
		if slices.ContainsFunc(
			existingSpirits,
			func(spirit sbx.Sandbox) bool {
				return spirit.Name == args.name
			},
		) {
			return fmt.Errorf("machine spirit with name '%s' already exists", args.name)
		}
	}

	fmt.Printf("Waking machine spirit: %s\n", color.Red.On(args.name))

	if err := sbx.RunDetached(
		ctx,
		args.name,
		// project.Model should never be nil here (populated with cfg.DefaultModel if not set)
		*project.Model,
		project.AbsolutePath,
		project.FullTemplateTag(),
		project.Kits,
	); err != nil {
		return err
	}

	for _, cmd := range project.Startup {
		if err := runConfiguredCommand(ctx, cmd, args.name, "startup"); err != nil {
			return err
		}
	}

	if err := sbx.RunExisting(ctx, args.name); err != nil {
		return err
	}

	return nil
}

func getRandomNameForMachineSpirit(existingSpirits []sbx.Sandbox, rng *rand.Rand) (string, error) {
	availableNames := make([]string, 0, len(machineSpiritNames))

	for _, name := range machineSpiritNames {
		if !slices.ContainsFunc(
			existingSpirits,
			func(spirit sbx.Sandbox) bool {
				return spirit.Name == name
			},
		) {
			availableNames = append(availableNames, name)
		}
	}

	if len(availableNames) == 0 {
		return "", errors.New(
			"all random names taken - either kill an existing machine spirit, or set name explicitly",
		)
	}

	randomIndex := rng.IntN(len(availableNames))
	return availableNames[randomIndex], nil
}

func defaultRand() *rand.Rand {
	// #nosec G404 - Weak randomness is sufficient here
	return rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
}

var machineSpiritNames = []string{
	"bax",
	"bix",
	"dax",
	"fex",
	"gix",
	"hax",
	"jax",
	"kex",
	"lux",
	"mox",
	"nux",
	"pex",
	"rax",
	"tox",
	"vox",
	"zyx",
}
