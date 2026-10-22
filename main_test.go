//nolint:exhaustruct_v5
package main

import (
	"errors"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"

	"hermannm.dev/omnissiah/internal/sliceutil"
	"hermannm.dev/omnissiah/sbx"
)

func TestGetNumericNameForMachineSpirit(t *testing.T) {
	type testCase struct {
		Test            string
		ExistingSpirits []sbx.Sandbox
		ExpectedName    string
		ExpectedError   error
	}

	cases := []testCase{
		{
			Test:            "No existing spirits",
			ExistingSpirits: []sbx.Sandbox{},
			ExpectedName:    machineSpiritNames[0],
		},
		{
			Test: "1 existing spirit",
			ExistingSpirits: []sbx.Sandbox{
				{Name: machineSpiritNames[0]},
			},
			ExpectedName: machineSpiritNames[1],
		},
		{
			Test: "1 existing spirit with higher name index",
			ExistingSpirits: []sbx.Sandbox{
				{Name: machineSpiritNames[1]},
			},
			ExpectedName: machineSpiritNames[0],
		},
		{
			Test: "Multiple existing spirits",
			ExistingSpirits: []sbx.Sandbox{
				{Name: machineSpiritNames[0]},
				{Name: "feature-a"},
				{Name: machineSpiritNames[1]},
				{Name: machineSpiritNames[3]},
			},
			ExpectedName: machineSpiritNames[2],
		},
		{
			Test: "All random names taken",
			ExistingSpirits: sliceutil.Map(
				machineSpiritNames,
				func(name string) sbx.Sandbox {
					return sbx.Sandbox{Name: name}
				},
			),
			ExpectedName: "",
			ExpectedError: errors.New(
				"all random names taken - either kill an existing machine spirit, or set name explicitly",
			),
		},
	}

	for _, test := range cases {
		t.Run(
			test.Test,
			func(t *testing.T) {
				rng := rand.New(&incrementingRandSource{count: 0})
				name, err := getRandomNameForMachineSpirit(test.ExistingSpirits, rng)
				require.Equal(t, test.ExpectedName, name)
				require.Equal(t, test.ExpectedError, err)
			},
		)
	}
}

type incrementingRandSource struct {
	count uint64
}

func (source *incrementingRandSource) Uint64() uint64 {
	value := source.count
	source.count++
	return value
}
