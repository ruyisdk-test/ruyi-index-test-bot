package bot

import (
	"errors"
	"log/slog"
)

func CmdVerify(cmd string) error {
	ok := len(cmd) > 0
	if !ok {
		return errors.New("invalid command")
	}

	return nil
}

func CmdRun(cmd string) {
	slog.Info("running command:", cmd)
}
