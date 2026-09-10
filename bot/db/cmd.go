package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/sahilm/fuzzy"
	"github.com/valkey-io/valkey-go"
)

type VersionData struct {
	Distfiles map[string][]string `json:"distfiles"`
}

type TestStatus struct {
	Code int        `json:"code"`
	Msg  string     `json:"message"`
	Prev *time.Time `json:"prev"`
}

const viewPrefix = "ruyindextextbot\x00"
const viewHash = viewPrefix + "hash"
const viewGroups = viewPrefix + "%s\x00groups"
const viewPackages = viewPrefix + "%s\x00packages"
const valuePackages = "%s\x00%s"
const viewPackagesGroup = viewPrefix + "%s\x00%s\x00groups"
const viewGroupsPackages = viewPrefix + "%s\x00%s\x00packages"
const viewVersions = viewPrefix + "%s\x00%s\x00%s\x00versions"
const viewUrlPackages = viewPrefix + "%s\x00%s\x00packages"

const viewUrlTestStatus = viewPrefix + "%s\x00status"
const viewUrlFailure = viewPrefix + "urlfailed"

var packagesGroups []string = nil
var urlTestList []string = nil

// AddViews list groups
// list packages\00groups
// group -> packages
// package -> groups
// packages\00groups -> version
// packages\00groups\00version -> distfiles -> urls
func AddViews(ctx context.Context, hash string, ttlDays int64, data map[string]map[string]map[string]VersionData) error {
	if valkeyClient == nil {
		return errors.New("valkey client is nil")
	}
	if ttlDays <= 0 {
		return errors.New("ttl days must be greater than zero")
	}

	ttl := ttlDays * 24 * 60 * 60

	groups := make([]string, 0, len(data))
	newPackagesGroups := make([]string, 0)
	urlPkgs := make(map[string][]string)

	cmds := make([]valkey.Completed, 0)

	// groups view
	for group, packages := range data {

		groups = append(groups, group)

		// group -> packages
		pkgList := make([]string, 0, len(packages))

		for pkg, versions := range packages {

			pkgList = append(pkgList, pkg)
			valuePkg := fmt.Sprintf(valuePackages, pkg, group)
			newPackagesGroups = append(newPackagesGroups, valuePkg)

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

			// package + group -> versions + data
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

			// url -> packages
			for _, value := range versions {
				for _, urls := range value.Distfiles {
					for _, url := range urls {
						urlPkgs[url] = append(urlPkgs[url], valuePkg)
					}
				}
			}
		}

		// group -> packages
		packageKey := fmt.Sprintf(viewGroupsPackages, hash, group)

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

	// list packages
	packageKey := fmt.Sprintf(viewPackages, hash)

	zadd := valkeyClient.B().
		Zadd().
		Key(packageKey).
		ScoreMember()

	for _, pkg := range newPackagesGroups {
		zadd = zadd.ScoreMember(0, pkg)
	}

	cmds = append(
		cmds,
		zadd.Build(),
	)

	cmds = append(
		cmds,
		valkeyClient.B().
			Expire().
			Key(packageKey).
			Seconds(ttl).
			Build(),
	)

	// url -> packages
	newUrlTestList := make([]string, 0, len(urlPkgs))
	for u, pkgs := range urlPkgs {
		newUrlTestList = append(newUrlTestList, u)
		urlKey := fmt.Sprintf(viewUrlPackages, hash, u)
		cmds = append(
			cmds,
			valkeyClient.B().
				Sadd().
				Key(urlKey).
				Member(pkgs...).
				Build(),
		)

		cmds = append(
			cmds,
			valkeyClient.B().
				Expire().
				Key(urlKey).
				Seconds(ttl).
				Build(),
		)
	}

	// do all
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
	err := valkeyClient.Do(
		ctx,
		valkeyClient.B().Set().
			Key(viewHash).
			Value(hash).
			Build(),
	).Error()

	if err != nil {
		return err
	}

	packagesGroups = newPackagesGroups
	urlTestList = newUrlTestList
	return nil
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
func ListGroups(ctx context.Context) (map[string]any, error) {
	hash, err := getCurrentHash(ctx)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf(viewGroups, hash)

	v, err := valkeyClient.Do(
		ctx,
		valkeyClient.B().Smembers().
			Key(key).
			Build(),
	).AsStrSlice()

	if err != nil {
		return nil, err
	}

	vv := make(map[string]any)
	vv["groups"] = v
	vv["hash"] = hash

	return vv, nil
}

func ListPackages(ctx context.Context, page int, size int) (map[string]any, error) {
	hash, err := getCurrentHash(ctx)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf(viewPackages, hash)

	pageM := int(math.Ceil(float64(len(packagesGroups)) / float64(size)))
	if page < 0 || page >= pageM {
		return nil, errors.New("page out of range")
	}

	start := page * size
	stop := start - 1 + size

	pkgs, err := valkeyClient.Do(
		ctx,
		valkeyClient.B().
			Zrange().
			Key(key).
			Min(strconv.Itoa(start)).
			Max(strconv.Itoa(stop)).
			Build(),
	).AsStrSlice()

	if err != nil {
		return nil, err
	}

	vv := make(map[string]any)
	p := make([]map[string]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		k := strings.Split(pkg, "\x00")
		p = append(p, map[string]string{
			"package": k[0],
			"group":   k[1],
		})
	}
	vv["packages"] = p
	vv["pages"] = pageM
	vv["hash"] = hash

	return vv, nil
}

// SearchPackages fuzzy search packages
func SearchPackages(pattern string) (map[string]any, error) {
	matches := fuzzy.Find(pattern, packagesGroups)
	if len(matches) > 50 {
		matches = matches[:50]
	}

	vv := make(map[string]any)
	p := make([]map[string]string, 0, len(matches))
	for _, match := range matches {
		k := strings.Split(match.Str, "\x00")
		p = append(p, map[string]string{
			"package": k[0],
			"group":   k[1],
			"score":   strconv.Itoa(match.Score),
		})
	}
	vv["packages"] = p

	return vv, nil
}

// GetGroupsByPkg package -> groups
func GetGroupsByPkg(ctx context.Context, pkg string) (map[string]any, error) {

	hash, err := getCurrentHash(ctx)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf(viewPackagesGroup, hash, pkg)

	v, err := valkeyClient.Do(
		ctx,
		valkeyClient.B().Smembers().
			Key(key).
			Build(),
	).AsStrSlice()

	if err != nil {
		return nil, err
	}

	vv := make(map[string]any)
	vv["groups"] = v
	vv["package"] = pkg
	vv["hash"] = hash

	return vv, nil
}

// GetPackagesByGroup group -> package
func GetPackagesByGroup(ctx context.Context, group string) (map[string]any, error) {

	hash, err := getCurrentHash(ctx)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf(viewGroupsPackages, hash, group)

	v, err := valkeyClient.Do(
		ctx,
		valkeyClient.B().Smembers().
			Key(key).
			Build(),
	).AsStrSlice()

	if err != nil {
		return nil, err
	}

	vv := make(map[string]any)
	vv["group"] = group
	vv["packages"] = v
	vv["hash"] = hash

	return vv, nil
}

// GetPackageVersionData group/package/version -> data
func GetPackageVersionData(ctx context.Context, pkg string, group string, version string) (map[string]any, error) {

	hash, err := getCurrentHash(ctx)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf(viewVersions, hash, pkg, group)

	v, err := valkeyClient.Do(
		ctx,
		valkeyClient.B().Hget().
			Key(key).
			Field(version).
			Build(),
	).ToString()

	if err != nil {
		return nil, err
	}

	vd := VersionData{Distfiles: nil}
	err = json.Unmarshal([]byte(v), &vd)

	if err != nil {
		return nil, err
	}

	vv := make(map[string]any)
	vv["versions"] = vd.Distfiles
	vv["group"] = group
	vv["package"] = pkg
	vv["hash"] = hash

	return vv, nil
}

// GetPackageVersions package/group -> version/data
func GetPackageVersions(ctx context.Context, pkg string, group string) (map[string]any, error) {

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

	vv := make(map[string]any)
	vv["versions"] = values
	vv["group"] = group
	vv["package"] = pkg
	vv["hash"] = hash

	return vv, nil
}

func GetPackagesByUrl(ctx context.Context, url string) (map[string]any, error) {
	hash, err := getCurrentHash(ctx)
	if err != nil {
		return nil, err
	}

	urlKey := fmt.Sprintf(viewUrlPackages, hash, url)

	pkgs, err := valkeyClient.Do(
		ctx,
		valkeyClient.B().
			Smembers().
			Key(urlKey).
			Build(),
	).AsStrSlice()

	if err != nil {
		return nil, err
	}

	vv := make(map[string]any)
	p := make([]map[string]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		k := strings.Split(pkg, "\x00")
		p = append(p, map[string]string{
			"package": k[0],
			"group":   k[1],
		})
	}
	vv["packages"] = p
	vv["hash"] = hash

	return vv, nil
}

func AddUrlTestStatus(ctx context.Context, ttlDay int64, url string, status int, err error) error {
	msg := "pass"
	if err != nil {
		msg = fmt.Sprintf("fail: %s", err.Error())
	}

	prev := time.Now()
	stat := TestStatus{
		Msg:  msg,
		Code: status,
		Prev: &prev,
	}

	stats, err := json.Marshal(&stat)
	if err != nil {
		return err
	}

	ttl := ttlDay * 24 * 60 * 60

	urlStatusKey := fmt.Sprintf(viewUrlTestStatus, url)
	urlStatusAdd := valkeyClient.B().Sadd().Key(urlStatusKey).Member(string(stats)).Build()
	urlStatusExp := valkeyClient.B().Expire().Key(urlStatusKey).Seconds(ttl).Build()

	cmds := []valkey.Completed{urlStatusAdd, urlStatusExp}
	urlFailureKey := viewUrlFailure
	if stat.Code != 200 {
		urlFailureAdd := valkeyClient.B().Sadd().Key(urlFailureKey).Member(url).Build()
		cmds = append(cmds, urlFailureAdd)
	}

	result := valkeyClient.DoMulti(ctx, cmds...)
	for _, r := range result {
		if err := r.Error(); err != nil {
			return err
		}
	}

	return nil
}

func ListUrlFailures(ctx context.Context) (map[string][]string, error) {
	urlFailureKey := viewUrlFailure

	urls, err := valkeyClient.Do(
		ctx,
		valkeyClient.B().
			Smembers().
			Key(urlFailureKey).
			Build(),
	).AsStrSlice()

	if err != nil {
		return nil, err
	}

	vv := make(map[string][]string)
	vv["urls"] = urls

	return vv, nil
}

func CleanUrlFailures(ctx context.Context) error {
	urlFailureKey := viewUrlFailure

	urlFailures, err := ListUrlFailures(ctx)
	if err != nil {
		return err
	}

	delUrl := make([]string, 0)
	for _, url := range urlFailures["urls"] {
		status, err := GetUrlTestStatus(ctx, url)
		if err != nil {
			delUrl = append(delUrl, url)
		}
		stat := status["status"]
		if reflect.TypeOf(stat).Kind() == reflect.Slice && len(stat.([]string)) == 0 {
			delUrl = append(delUrl, url)
		}
	}

	if len(delUrl) == 0 {
		return nil
	}

	sdel := valkeyClient.B().
		Srem().
		Key(urlFailureKey).
		Member(delUrl...).
		Build()

	return valkeyClient.Do(ctx, sdel).Error()
}

func GetUrlTestList() []string {
	return urlTestList
}

func GetUrlTestStatus(ctx context.Context, url string) (map[string]any, error) {

	urlKey := fmt.Sprintf(viewUrlTestStatus, url)

	status, err := valkeyClient.Do(
		ctx,
		valkeyClient.B().
			Smembers().
			Key(urlKey).
			Build(),
	).AsStrSlice()

	if err != nil {
		return nil, err
	}

	vv := make(map[string]any)
	vv["status"] = status
	vv["url"] = url

	return vv, nil
}
