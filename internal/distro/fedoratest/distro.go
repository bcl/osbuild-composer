package fedoratest

import (
	"errors"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type FedoraTestDistro struct{}

const ModulePlatformID = "platform:f30"

func New() *FedoraTestDistro {
	return &FedoraTestDistro{}
}

func (d *FedoraTestDistro) Name() string {
	return "fedora-30"
}

func (d *FedoraTestDistro) Distribution() common.Distribution {
	return common.Fedora30
}

func (d *FedoraTestDistro) ModulePlatformID() string {
	return ModulePlatformID
}

func (d *FedoraTestDistro) Repositories(arch string) []rpmmd.RepoConfig {
	return []rpmmd.RepoConfig{
		{
			Id:      "test-id",
			Name:    "Test Name",
			BaseURL: "http://example.com/test/os/" + arch,
		},
	}
}

func (d *FedoraTestDistro) ListOutputFormats() []string {
	return []string{"qcow2"}
}

func (d *FedoraTestDistro) FilenameFromType(outputFormat string) (string, string, error) {
	if outputFormat == "qcow2" {
		return "test.img", "application/x-test", nil
	} else {
		return "", "", errors.New("invalid output format: " + outputFormat)
	}
}

func (r *FedoraTestDistro) GetSizeForOutputType(outputFormat string, size uint64) uint64 {
	return 0
}

func (d *FedoraTestDistro) BasePackages(outputFormat string, outputArchitecture string) ([]string, []string, error) {
	return nil, nil, nil
}

func (d *FedoraTestDistro) BuildPackages(outputArchitecture string) ([]string, error) {
	return nil, nil
}

func (d *FedoraTestDistro) Pipeline(b *blueprint.Blueprint, additionalRepos []rpmmd.RepoConfig, checksums map[string]string, outputArch, outputFormat string, size uint64) (*osbuild.Pipeline, error) {
	if outputFormat == "qcow2" && outputArch == "x86_64" {
		return &osbuild.Pipeline{}, nil
	} else {
		return nil, errors.New("invalid output format or arch: " + outputFormat + " @ " + outputArch)
	}
}

func (d *FedoraTestDistro) Runner() string {
	return "org.osbuild.test"
}