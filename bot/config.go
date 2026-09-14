package bot

import (
	"log/slog"
	"os"
	"path"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

const filename = "config.yaml"

type Config struct {
	Server struct {
		ListenAddr        string `yaml:"listen_addr"`
		ControlListenAddr string `yaml:"control_addr"`
	} `yaml:"server"`

	Cache struct {
		CacheDir string `yaml:"cache_dir"`
	} `yaml:"cache"`

	Repo struct {
		Remote   string `yaml:"remote"`
		Branch   string `yaml:"branch"`
		cacheDir string `yaml:"-"`
	} `yaml:"repo"`

	Valkey struct {
		Addr    string `yaml:"addr"`
		DataTtl int64  `yaml:"data_ttl"`
	} `yaml:"valkey"`

	BeforeServe Cmd          `yaml:"before_serve"`
	Cron        []CronConfig `yaml:"cron"`

	configPath string     `yaml:"-"`
	configInit bool       `yaml:"-"`
	node       *yaml.Node `yaml:"-"`
}

func CfgLoad() (*Config, error) {
	pathCurrent, err := filepath.Abs(".")
	if err != nil {
		return nil, err
	}
	pathConfig := filepath.Join(pathCurrent, filename)

	var config Config
	config.configPath = pathConfig

	if _, err := os.Stat(pathConfig); os.IsNotExist(err) {
		config.configInit = true
		slog.Warn("config file not found, using defaults")
	} else {

		slog.Info("loading config file:", "path", config.configPath)

		var data []byte
		data, err = os.ReadFile(pathConfig)
		if err != nil {
			return nil, err
		}

		var node yaml.Node
		if err = yaml.Unmarshal(data, &node); err != nil {
			slog.Error("error parsing config file", "error", err)
			return nil, err
		}

		if err = node.Decode(&config); err != nil {
			slog.Error("error decode content", "error", err)
			return nil, err
		}

		config.node = &node
	}

	if config.Server.ControlListenAddr == "" {
		config.Server.ControlListenAddr = "127.0.0.1:9876"
	}
	if config.Server.ListenAddr == "" {
		config.Server.ListenAddr = "127.0.0.1:9877"
	}
	if config.Cache.CacheDir == "" {
		config.Cache.CacheDir = filepath.Join(pathCurrent, "cache")
	}
	config.Repo.cacheDir = path.Join(config.Cache.CacheDir, "repo", "ruyisdk")
	if config.Repo.Remote == "" {
		config.Repo.Remote = "https://github.com/ruyisdk/packages-index.git"
	}
	if config.Repo.Branch == "" {
		config.Repo.Branch = "main"
	}
	if config.Valkey.Addr == "" {
		config.Valkey.Addr = "127.0.0.1:6379"
	}
	if config.Valkey.DataTtl == 0 {
		config.Valkey.DataTtl = 7 // days
	}

	slog.Info("service listening on address:", "addr", config.Server.ListenAddr)
	slog.Info("control listening on address:", "addr", config.Server.ControlListenAddr)
	slog.Info("use cache dir:", "path", config.Cache.CacheDir)
	slog.Info("connect valkey address:", "addr", config.Valkey.Addr)
	slog.Info("connect valkey data TTL:", "days", config.Valkey.DataTtl)

	if _, err := os.Stat(config.Cache.CacheDir); os.IsNotExist(err) {
		if err := os.Mkdir(config.Cache.CacheDir, 0755); err != nil {
			slog.Error("error creating cache dir", "error", err)
			return nil, err
		}
	}

	// save config here for we have CfgSave TODOs
	// when we solve that TODO, we can save all config on bot exit
	if config.configInit {
		if err = CfgSave(&config); err != nil {
			slog.Warn("error saving init config", "error", err)
		}
	}

	return &config, nil
}

// CfgSave save bot config
// TODO: save without damage file format
func CfgSave(config *Config) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(config.configPath, data, 0644)
}
