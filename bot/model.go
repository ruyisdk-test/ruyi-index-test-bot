package testbot

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

var botGenKit *genkit.Genkit = nil
var botModel *ai.ModelRef = nil

func ModelHello(config *Config) error {

	ctx := context.Background()

	g := genkit.Init(ctx, genkit.WithPlugins(&compat_oai.OpenAICompatible{
		Provider: config.Model.Provider,
		APIKey:   config.Model.ApiKey,
		BaseURL:  config.Model.BaseUrl,
		Opts: []option.RequestOption{
			option.WithHeader("Custom-Header", "value"),
		},
	}))

	modelName := fmt.Sprintf("%s/%s", config.Model.Provider, config.Model.ModelName)
	model := ai.NewModelRef(
		modelName,
		&openai.ChatCompletionNewParams{
			Temperature:         openai.Float(0.7),
			MaxCompletionTokens: openai.Int(1024),
		})

	resp, err := genkit.Generate(ctx, g,
		ai.WithModel(model),
		ai.WithPrompt("用一行文字简单介绍你自己"),
	)

	if err != nil {
		return err
	}

	slog.Info(resp.Message.Text())

	botGenKit = g
	botModel = &model

	return nil
}

func ModelAsk(msg string) (*ai.ModelResponse, error) {
	return genkit.Generate(
		context.Background(),
		botGenKit,
		ai.WithModel(botModel),
		ai.WithPrompt(msg),
	)
}

type UpstreamVersionResult struct {
	Version []string `json:"version"`
}

type PackageResults struct {
	Results []PackageResult `json:"results"`
}

type PackageResult struct {
	Name    string   `json:"name"`
	Version string   `json:"version"`
	URLs    []string `json:"urls"`
}

func ModelAskUpstreamVersion(msg string) (*UpstreamVersionResult, *ai.ModelResponse, error) {
	return ModelAskData[UpstreamVersionResult](msg)
}

func ModelAskData[T any](msg string) (*T, *ai.ModelResponse, error) {
	return genkit.GenerateData[T](
		context.Background(),
		botGenKit,
		ai.WithModel(botModel),
		ai.WithPrompt(msg),
	)
}
