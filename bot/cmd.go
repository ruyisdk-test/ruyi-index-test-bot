package bot

import (
	"errors"
	"log/slog"
)

var funcVerify = map[string]func(map[string]string) error{
	"git":  gitVerify,
	"test": testVerify,
}

var funcRun = map[string]func(map[string]string) error{
	"git":  gitRun,
	"test": testRun,
}

func CmdVerify(cmd string, keys map[string]string) error {
	fv := funcVerify[cmd]
	if fv == nil {
		return errors.New("command not found: " + cmd)
	}
	fr := funcRun[cmd]
	if fr == nil {
		return errors.New("command cannot run: " + cmd)
	}

	return fv(keys)
}

func CmdRun(cmd string, keys map[string]string) {
	slog.Info("running command:", cmd)
	err := funcRun[cmd](keys)
	if err != nil {
		slog.Error("run command failed: ", "error", err.Error())
	}
}

func gitVerify(keys map[string]string) error {
	return nil
}

func testVerify(keys map[string]string) error {
	return nil
}

func gitRun(keys map[string]string) error {
	return nil
}

func testRun(keys map[string]string) error {
	return nil
}
