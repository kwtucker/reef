package clipboard

import (
	"log"
	"os/exec"
	"runtime"
)

func Copy(input []byte) {
	arch := runtime.GOOS

	var copyCmd *exec.Cmd

	// Mac "OS"
	if arch == "darwin" {
		copyCmd = exec.Command("pbcopy")
	}
	// Linux
	if arch == "linux" {
		copyCmd = exec.Command("xclip", "-selection", "c")
	}

	in, err := copyCmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}

	if err := copyCmd.Start(); err != nil {
		log.Fatal(err)
	}

	if _, err := in.Write([]byte(input)); err != nil {
		log.Fatal(err)
	}

	if err := in.Close(); err != nil {
		log.Fatal(err)
	}

	copyCmd.Wait()
}
