package main

import testbot "github.com/ruyisdk-test/ruyi-index-test-bot/bot"

func main() {
	cfg, err := testbot.CfgLoad()
	if err != nil {
		panic(err)
	}

	testbot.Serve(cfg)
}
