package testbot

import testagent "github.com/ruyisdk-test/ruyi-index-test-bot/bot/agent"

func ModelHello(config *Config) error {
	return testagent.ModelHello(config.Model.Provider, config.Model.ModelName, config.Model.BaseUrl, config.Model.ApiKey)
}
