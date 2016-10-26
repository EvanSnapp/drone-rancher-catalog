package tag

import (
	"testing"

	"github.com/LeanKit-Labs/drone-rancher-catalog/types"
	"github.com/franela/goblin"
)

var repo = types.Repo{Owner: "owner", Name: "repo"}
var build = types.Build{Number: 0, Workspace: ".", Commit: "01234567890", Branch: "my_Branch"}
var masterbuild = types.Build{Number: 0, Workspace: ".", Commit: "01234567890", Branch: "master"}

var pluginDev = types.Plugin{
	Repo: repo, Build: build, ProjectType: "node",
	DockerStorageDriver: "overlay",
	DockerHubRepo:       "repo",
	DockerHubUser:       "user",
	DockerHubPass:       "secret",
	DockerHubEmail:      "example@example.com",
	GithubAccessToken:   "supersecret",
	RancherCatalogRepo:  "catalog",
	RancherCatalogName:  "repo",
	DryRun:              false,
}
var pluginMaster = types.Plugin{
	Repo: repo, Build: masterbuild, ProjectType: "node",
	DockerStorageDriver: "overlay",
	DockerHubRepo:       "repo",
	DockerHubUser:       "user",
	DockerHubPass:       "secret",
	DockerHubEmail:      "example@example.com",
	GithubAccessToken:   "supersecret",
	RancherCatalogRepo:  "catalog",
	RancherCatalogName:  "repo",
	DryRun:              false,
}

func TestHookImage(t *testing.T) {
	g := goblin.Goblin(t)
	g.Describe("Tag", func() {
		g.Describe("Node", func() {
			g.It("Development", func() {
				if tags, err := CreateDockerImageTags(pluginDev, "1.0.0"); true {
					g.Assert(err).Equal(nil)
					g.Assert(tags).Equal([]string{"owner_repo_my-Branch_1.0.0_0_0123456"})
				}
			})

			g.It("Master", func() {
				if tags, err := CreateDockerImageTags(pluginMaster, "1.0.0"); true {
					g.Assert(err).Equal(nil)
					g.Assert(tags).Equal([]string{"v1.0.0", "v1.0.0-drone.build.0", "latest"})
				}
			})
		})
	})
}
