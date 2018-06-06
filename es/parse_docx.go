package es

import (
	"bytes"
	"os"
	"os/exec"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
)

func ParseDocx(docxPath string) (string, error) {
	if _, err := os.Stat(docxPath); os.IsNotExist(err) {
		return "", errors.Wrapf(err, "os.Stat %s", docxPath)
	}
	var cmd *exec.Cmd
	if IsWindows() {
		cmd = exec.Command(pythonPath, parseDocsBin, docxPath)
	} else {
		cmd = exec.Command(parseDocsBin, docxPath)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Warnf("[%s %s]\nstdout: [%s]\nstderr: [%s]\nError: %+v\n", parseDocsBin, docxPath, stdout.String(), stderr.String(), err)
		return "", errors.Wrapf(err, "cmd.Run %s", docxPath)
	}
	return stdout.String(), nil
}
