package git

import (
	"os/exec"
	"strings"
)

// Staged gets the staged files of the repos directory
func Staged() ([]string, error) {
	// Git status command
	gsArgs := []string{"diff", "--name-only", "--cached"}
	gitStatus := exec.Command("git", gsArgs...)

	// Get stdout and trim the empty last index
	fileStatus, err := gitStatus.CombinedOutput()
	if err != nil {
		return nil, err
	}

	fsSplit := strings.Split(string(fileStatus), "\n")

	return fsSplit, nil
}
