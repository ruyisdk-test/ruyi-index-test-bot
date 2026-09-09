package db

import (
	"context"
	"log/slog"

	"github.com/valkey-io/valkey-go"
)

var valkeyClient valkey.Client = nil

func Connect(valkeyAddr string) error {
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{
			valkeyAddr,
		},
	})
	if err != nil {
		return err
	}

	valkeyClient = client

	if err = clientPing(); err != nil {
		Close()
		return err
	}

	return nil
}

func clientPing() error {
	ctx := context.Background()

	if err := valkeyClient.Do(
		ctx,
		valkeyClient.B().Ping().Build(),
	).Error(); err != nil {
		return err
	}

	slog.Info("valkey client ping pass")

	return nil
}

func Close() {
	if valkeyClient != nil {
		valkeyClient.Close()
	}
}
