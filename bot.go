package main

import "github.com/ruyisdk-test/ruyi-index-test-bot/bot"

func main() {
	cfg, err := bot.CfgLoad()
	if err != nil {
		panic(err)
	}

	bot.Serve(cfg)
}
