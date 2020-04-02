// This package contains tests related to dnf-json and rpmmd package.

// +build integration

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora30"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora31"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/test"
)

func TestFetchChecksum(t *testing.T) {
	dir, err := test.SetUpTemporaryRepository()
	defer func(dir string) {
		err := test.TearDownTemporaryRepository(dir)
		assert.Nil(t, err, "Failed to clean up temporary repository.")
	}(dir)
	assert.Nilf(t, err, "Failed to set up temporary repository: %v", err)

	repoCfg := rpmmd.RepoConfig{
		Id:        "repo",
		BaseURL:   fmt.Sprintf("file://%s", dir),
		IgnoreSSL: true,
	}
	rpmMetadata := rpmmd.NewRPMMD(path.Join(dir, "rpmmd"))
	_, c, err := rpmMetadata.FetchMetadata([]rpmmd.RepoConfig{repoCfg}, "platform:f31", "x86_64")
	assert.Nilf(t, err, "Failed to fetch checksum: %v", err)
	assert.NotEqual(t, "", c["repo"], "The checksum is empty")
}

// This test loads all the repositories available in /repositories directory
// and tries to run depsolve for each architecture. With N architectures available
// this should run cross-arch dependency solving N-1 times.
func TestCrossArchDepsolve(t *testing.T) {
	// Set up temporary directory for rpm/dnf cache
	dir, err := ioutil.TempDir("/tmp", "rpmmd-test-")
	require.Nilf(t, err, "Failed to create tmp dir for depsolve test: %v", err)
	defer os.RemoveAll(dir)
	rpm := rpmmd.NewRPMMD(dir)

	// Load repositories from the definition we provide in the RPM package
	repositories := "/usr/share/osbuild-composer"

	// NOTE: we can add RHEL, but don't make it hard requirement because it will fail outside of VPN
	for _, distroStruct := range []distro.Distro{fedora30.New(), fedora31.New(), fedora32.New()} {
		repoConfig, err := rpmmd.LoadRepositories([]string{repositories}, distroStruct.Name())
		assert.Nilf(t, err, "Failed to LoadRepositories from %v for %v: %v", repositories, distroStruct.Name(), err)
		if err != nil {
			// There is no point in running the tests without having repositories, but we can still run tests
			// for the remaining distros
			continue
		}
		for _, archStr := range distroStruct.ListArches() {
			arch, err := distroStruct.GetArch(archStr)
			assert.Nilf(t, err, "Failed to GetArch from %v structure: %v", distroStruct.Name(), err)
			if err != nil {
				continue
			}
			for _, imgTypeStr := range arch.ListImageTypes() {
				imgType, err := arch.GetImageType(imgTypeStr)
				assert.Nilf(t, err, "Failed to GetImageType for %v on %v: %v", distroStruct.Name(), arch.Name(), err)
				if err != nil {
					continue
				}

				buildPackages := imgType.BuildPackages()
				_, _, err = rpm.Depsolve(buildPackages, []string{}, repoConfig[archStr], distroStruct.ModulePlatformID(), archStr)
				assert.Nilf(t, err, "Failed to Depsolve build packages for %v %v %v image: %v", distroStruct.Name(), imgType.Name(), arch.Name(), err)

				basePackagesInclude, basePackagesExclude := imgType.BasePackages()
				_, _, err = rpm.Depsolve(basePackagesInclude, basePackagesExclude, repoConfig[archStr], distroStruct.ModulePlatformID(), archStr)
				assert.Nilf(t, err, "Failed to Depsolve base packages for %v %v %v image: %v", distroStruct.Name(), imgType.Name(), arch.Name(), err)
			}
		}
	}
}

// TestRepoMetadataExpire tests that updates to a repository will be properly depsolved
// It uses a temporary repository and a couple of fake rpm files by setting the timestamp
// to 72 hours in the past and then adding a new rpm and updating the repodata
func TestRepoMetadataExpire(t *testing.T) {
	var reply struct {
		Checksums    map[string]string   `json:"checksums"`
		Dependencies []rpmmd.PackageSpec `json:"dependencies"`
	}

	dir, err := test.SetUpTemporaryRepository()
	require.NoError(t, err, "Failed to set up temporary repository")
	defer func(dir string) {
		err := test.TearDownTemporaryRepository(dir)
		require.NoError(t, err, "Failed to clean up temporary repository.")
	}(dir)

	// Make a fake rpm file in the repo (this also runs createrepo)
	err = test.MakeFakeRPM(dir, "oldmeta", "1.0.0", "1")
	require.NoError(t, err, "Failed to create fake rpm")

	// Cache directory for dnf
	cacheDir, err := ioutil.TempDir("/tmp", "osbuild-composer-test-")
	require.NoError(t, err, "Failed to create temporary cache dir")
	defer func(dir string) {
		err := os.RemoveAll(dir)
		require.NoError(t, err, "Failed to clean up temporary repository.")
	}(cacheDir)

	// Construct RepoConfig to point to it
	repo := rpmmd.RepoConfig{"mdexpire", "file://" + dir, "", "", "", true}
	var arguments = struct {
		PackageSpecs     []string           `json:"package-specs"`
		ExcludSpecs      []string           `json:"exclude-specs"`
		Repos            []rpmmd.RepoConfig `json:"repos"`
		CacheDir         string             `json:"cachedir"`
		ModulePlatformID string             `json:"module_platform_id"`
		Arch             string             `json:"arch"`
	}{[]string{"oldmeta"}, nil, []rpmmd.RepoConfig{repo}, cacheDir, "platform:f31", "x86_64"}

	// Depsolve the package and check the results
	err = rpmmd.RunDNFTestOnly("depsolve", arguments, &reply)
	//	reply.Dependencies, reply.Checksums, err
	require.NoError(t, err, "Failed to depsolve the 1st time")
	require.Equal(t, len(reply.Dependencies), 1)
	require.Equal(t, "oldmeta", reply.Dependencies[0].Name)
	require.Equal(t, "1.0.0", reply.Dependencies[0].Version)
	require.Equal(t, "1", reply.Dependencies[0].Release)

	// Set the timestamp on all of the cache files back 72 hours
	dt, _ := time.ParseDuration("-72h")
	err = test.BackdateDirTree(cacheDir, dt)
	require.NoError(t, err, "Failed to turn back time")

	// Add a new version of the fake package
	err = test.MakeFakeRPM(dir, "oldmeta", "2.0.0", "1")
	require.NoError(t, err, "Failed to create fake rpm")

	// depsolve it again and check for the new package and a checksum change
	err = rpmmd.RunDNFTestOnly("depsolve", arguments, &reply)
	//	reply.Dependencies, reply.Checksums, err
	require.NoError(t, err, "Failed to depsolve the 2nd time")
	require.Equal(t, len(reply.Dependencies), 1)
	require.Equal(t, "oldmeta", reply.Dependencies[0].Name)
	require.Equal(t, "2.0.0", reply.Dependencies[0].Version)
	require.Equal(t, "1", reply.Dependencies[0].Release)
}
