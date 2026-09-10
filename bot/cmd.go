package bot

import (
	"errors"
	"log/slog"

	"github.com/ruyisdk-test/ruyi-index-test-bot/bot/repo"
)

type Cmd struct {
	Name   string            `yaml:"name"`
	Params map[string]string `yaml:"params"`
}

var funcVerify = map[string]func(Cmd, *Config) error{
	"":     emptyVerify,
	"git":  gitVerify,
	"test": testVerify,
}

var funcRun = map[string]func(Cmd, *Config) error{
	"":     emptyRun,
	"git":  gitRun,
	"test": testRun,
}

func CmdVerify(cmd Cmd, cfg *Config) error {
	fv := funcVerify[cmd.Name]
	if fv == nil {
		return errors.New("command not found:" + cmd.Name)
	}
	fr := funcRun[cmd.Name]
	if fr == nil {
		return errors.New("command cannot run:" + cmd.Name)
	}

	return fv(cmd, cfg)
}

func CmdRun(cmd Cmd, cfg *Config) error {
	slog.Info("running command:", "name", cmd.Name, "params", cmd.Params)
	err := funcRun[cmd.Name](cmd, cfg)
	if err != nil {
		slog.Error("run command failed:", "error", err.Error())
	}

	return err
}

func emptyVerify(cmd Cmd, _ *Config) error {
	if cmd.Name != "" {
		return errors.New("command not empty:" + cmd.Name)
	}
	return nil
}

func gitVerify(cmd Cmd, cfg *Config) error {
	do := cmd.Params["do"]
	if do != "pull" {
		return errors.New("git command only support do=pull")
	}

	if cfg.RepoBranch == "" || cfg.RepoRemote == "" {
		return errors.New("invalid remote or branch: " + "remote=" + cfg.RepoRemote + "branch=" + cfg.RepoBranch)
	}

	return nil
}

func testVerify(cmd Cmd, cfg *Config) error {
	do := cmd.Params["do"]
	if do != "all" {
		return errors.New("git command only support do=all")
	}

	return nil
}

func emptyRun(_ Cmd, _ *Config) error {
	return nil
}

func gitRun(cmd Cmd, cfg *Config) error {
	err := repo.CheckLatest(cfg.repoCacheDir, cfg.RepoRemote, cfg.RepoBranch)
	if err != nil {
		return err
	}

	return repo.LoadData(cfg.repoCacheDir, cfg.ValkeyDataTtl)
}

func testRun(cmd Cmd, cfg *Config) error {
	return repo.TestRun(cfg.repoCacheDir, cfg.ValkeyDataTtl)
}
