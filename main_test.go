//nolint:exhaustruct_v5
package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"hermannm.dev/omnissiah/sbx"
)

func TestGetNumericNameForMachineSpirit(t *testing.T) {
	type testCase struct {
		Test            string
		ExistingSpirits []sbx.Sandbox
		ExpectedName    string
	}

	cases := []testCase{
		{
			Test:            "No existing spirits",
			ExistingSpirits: []sbx.Sandbox{},
			ExpectedName:    "00",
		},
		{
			Test:            "1 existing spirit",
			ExistingSpirits: []sbx.Sandbox{{Name: "00"}},
			ExpectedName:    "01",
		},
		{
			Test:            "1 existing spirit with higher name index",
			ExistingSpirits: []sbx.Sandbox{{Name: "01"}},
			ExpectedName:    "00",
		},
		{
			Test: "Multiple existing spirits",
			ExistingSpirits: []sbx.Sandbox{
				{Name: "00"},
				{Name: "feature-a"},
				{Name: "01"},
			},
			ExpectedName: "02",
		},
		{
			Test: "Existing spirits with non-contiguous names",
			ExistingSpirits: []sbx.Sandbox{
				{Name: "00"},
				{Name: "02"},
				{Name: "feature-b"},
				{Name: "03"},
			},
			ExpectedName: "01",
		},
	}

	for _, test := range cases {
		t.Run(
			test.Test,
			func(t *testing.T) {
				name := getNumericNameForMachineSpirit(test.ExistingSpirits)
				require.Equal(t, test.ExpectedName, name)
			},
		)
	}
}
