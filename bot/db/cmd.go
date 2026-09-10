package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/valkey-io/valkey-go"
)

type VersionData struct {
	Distfiles map[string][]string `json:"distfiles"`
}

const viewPrefix = "ruyindex\x00"
const viewHash = viewPrefix + "hash"
const viewGroups = viewPrefix + "%s\x00groups"
const viewPackagesGroup = viewPrefix + "%s\x00%s\x00groups"
const viewPackages = viewPrefix + "%s\x00%s\x00packages"
const viewVersions = viewPrefix + "%s\x00%s\x00%s\x00versions"

// AddViews list groups
// list packages\00groups
// group -> packages
// packages\00groups -> version
// packages\00groups\00version -> distfiles -> urls
func AddViews(ctx context.Context, hash string, ttlDays int64, data map[string]map[string]map[string]VersionData) error {
	if valkeyClient == nil {
		return errors.New("valkey client is nil")
	}
	cHash, err := getCurrentHash(ctx)
	if err == nil && cHash == hash {
		slog.Info("skip same hash in database", "hash", hash)
		return nil
	}

	ttl := ttlDays * 24 * 60 * 60

	groups := make([]string, 0, len(data))

	cmds := make([]valkey.Completed, 0)

	// groups view
	for group, packages := range data {

		groups = append(groups, group)

		// group -> packages
		pkgList := make([]string, 0, len(packages))

		for pkg, versions := range packages {

			pkgList = append(pkgList, pkg)

			// package -> groups
			packageGroupKey := fmt.Sprintf(viewPackagesGroup, hash, pkg)

			cmds = append(
				cmds,
				valkeyClient.B().
					Sadd().
					Key(packageGroupKey).
					Member(group).
					Build(),
			)

			cmds = append(
				cmds,
				valkeyClient.B().
					Expire().
					Key(packageGroupKey).
					Seconds(ttl).
					Build(),
			)

			// package + group -> versions
			versionKey := fmt.Sprintf(viewVersions, hash, pkg, group)

			hset := valkeyClient.B().
				Hset().
				Key(versionKey).
				FieldValue()

			for version, value := range versions {
				v, err := json.Marshal(value)
				if err != nil {
					return err
				}
				hset = hset.FieldValue(version, string(v))
			}

			cmds = append(cmds, hset.Build())

			cmds = append(
				cmds,
				valkeyClient.B().
					Expire().
					Key(versionKey).
					Seconds(ttl).
					Build(),
			)
		}

		// group -> packages
		packageKey := fmt.Sprintf(viewPackages, hash, group)

		cmds = append(
			cmds,
			valkeyClient.B().
				Sadd().
				Key(packageKey).
				Member(pkgList...).
				Build(),
		)

		cmds = append(
			cmds,
			valkeyClient.B().
				Expire().
				Key(packageKey).
				Seconds(ttl).
				Build(),
		)
	}

	// list groups
	groupKey := fmt.Sprintf(viewGroups, hash)

	cmds = append(
		cmds,
		valkeyClient.B().
			Sadd().
			Key(groupKey).
			Member(groups...).
			Build(),
	)

	cmds = append(
		cmds,
		valkeyClient.B().
			Expire().
			Key(groupKey).
			Seconds(ttl).
			Build(),
	)

	results := valkeyClient.DoMulti(
		ctx,
		cmds...,
	)

	for _, result := range results {
		if err := result.Error(); err != nil {
			return err
		}
	}

	// update hash after all views are written
	return valkeyClient.Do(
		ctx,
		valkeyClient.B().Set().
			Key(viewHash).
			Value(hash).
			Build(),
	).Error()
}

// getCurrentHash get index hash
func getCurrentHash(ctx context.Context) (string, error) {
	return valkeyClient.Do(
		ctx,
		valkeyClient.B().Get().
			Key(viewHash).
			Build(),
	).ToString()
}

// ListGroups list groups
func ListGroups(ctx context.Context) ([]string, error) {
	hash, err := getCurrentHash(ctx)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf(viewGroups, hash)

	return valkeyClient.Do(
		ctx,
		valkeyClient.B().Smembers().
			Key(key).
			Build(),
	).AsStrSlice()
}

// ListyPackagesGroup package -> groups
func ListyPackagesGroup(ctx context.Context, pkg string) ([]string, error) {

	hash, err := getCurrentHash(ctx)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf(viewPackagesGroup, hash, pkg)

	return valkeyClient.Do(
		ctx,
		valkeyClient.B().Smembers().
			Key(key).
			Build(),
	).AsStrSlice()
}

// ListPackages group -> package
func ListPackages(ctx context.Context, group string) ([]string, error) {

	hash, err := getCurrentHash(ctx)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf(viewPackages, hash, group)

	return valkeyClient.Do(
		ctx,
		valkeyClient.B().Smembers().
			Key(key).
			Build(),
	).AsStrSlice()
}

// ListVersions package/group -> version
func ListVersions(ctx context.Context, pkg string, group string) ([]string, error) {

	hash, err := getCurrentHash(ctx)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf(viewVersions, hash, pkg, group)

	return valkeyClient.Do(
		ctx,
		valkeyClient.B().Hkeys().
			Key(key).
			Build(),
	).AsStrSlice()
}

// GetVersionData group/package/version -> data
func GetVersionData(ctx context.Context, pkg string, group string, version string) (string, error) {

	hash, err := getCurrentHash(ctx)
	if err != nil {
		return "", err
	}

	key := fmt.Sprintf(viewVersions, hash, pkg, group)

	return valkeyClient.Do(
		ctx,
		valkeyClient.B().Hget().
			Key(key).
			Field(version).
			Build(),
	).ToString()
}

// GetAllVersions package/group -> version/data
func GetAllVersions(ctx context.Context, pkg string, group string) (map[string]string, error) {

	hash, err := getCurrentHash(ctx)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf(viewVersions, hash, pkg, group)

	values, err := valkeyClient.Do(
		ctx,
		valkeyClient.B().Hgetall().
			Key(key).
			Build(),
	).AsStrMap()

	if err != nil {
		return nil, err
	}

	return values, nil
}
