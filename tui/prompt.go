package tui

import (
	"fmt"

	"hermannm.dev/wrap"
)

func PromptUserYesOrNo() (yes bool, err error) {
	var answer string
	if _, err := fmt.Scanf("%s", &answer); err != nil {
		return false, wrap.Error(err, "failed to scan user input")
	}

	switch answer {
	case "y", "Y", "yes", "Yes", "YES":
		return true, nil
	case "n", "N", "no", "No", "NO":
		return false, nil
	default:
		return false, fmt.Errorf("invalid answer '%s'", answer)
	}
}
