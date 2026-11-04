package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"hermannm.dev/wrap"
)

// PromptUserYesOrNo prompts the user for a yes/no answer.
//
// defaultAnswer:
//   - nil: answer required
//   - true: default yes
//   - false: default no
func PromptUserYesOrNo(defaultAnswer *bool) (yes bool, err error) {
	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false, wrap.Error(err, "failed to scan user input")
	}
	answer = strings.TrimSpace(answer)

	switch answer {
	case "y", "Y", "yes", "Yes", "YES":
		return true, nil
	case "n", "N", "no", "No", "NO":
		return false, nil
	case "":
		if defaultAnswer != nil {
			return *defaultAnswer, nil
		} else {
			return false, fmt.Errorf("invalid answer '%s'", answer)
		}
	default:
		return false, fmt.Errorf("invalid answer '%s'", answer)
	}
}
