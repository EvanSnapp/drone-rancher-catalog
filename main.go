package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/LeanKit-Labs/drone-rancher-catalog/docker"
	"github.com/LeanKit-Labs/drone-rancher-catalog/github"
	"github.com/LeanKit-Labs/drone-rancher-catalog/tag"
	"github.com/LeanKit-Labs/drone-rancher-catalog/types"
	"github.com/drone/drone-go/drone"
	dronePlugin "github.com/drone/drone-go/plugin"
)

var version string

/*
	Steps:
		-generate a docker image tag from build meta-data
		-build and publish  the docker images (to docker hub)
		-generate a catalog entry and push to github
*/
func main() {
	fmt.Println("starting drone-rancher-catalog")
	/*
	   Drone pkg types are abstracted into "plugin" in order
	   to make the migration to Drone's 0.5 way of getting
	   plugin args easier (i.e. via env vars)
	*/
	workspace := drone.Workspace{}
	repo := drone.Repo{}
	build := drone.Build{}

	plugin := types.Plugin{}

	dronePlugin.Param("workspace", &workspace)
	dronePlugin.Param("repo", &repo)
	dronePlugin.Param("build", &build)
	dronePlugin.Param("vargs", &plugin)
	dronePlugin.MustParse()

	plugin.Repo = types.Repo{
		Owner: repo.Owner,
		Name:  repo.Name,
	}

	plugin.Build = types.Build{
		Number:    build.Number,
		Branch:    build.Branch,
		Commit:    build.Commit,
		Workspace: workspace.Path,
	}

	if err := exec(plugin); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("plugin completed, exiting")
	os.Exit(0)
}

func exec(p types.Plugin) error {

	//build tag
	//doing this outside of subpackage to support potential use cases where the
	//docker hub repo and docker hub repo are not the same
	imageTags, err := tag.CreateDockerImageTags(p)

	if err != nil {
		return err
	}
	//publish docker image
	if err := docker.PublishImage(p.DockerHubRepo, imageTags, p); err != nil {
		return err
	}

	if p.DryRun { ///exit if dry run
		return nil
	}
	rancherCatalogInfo := strings.Split(p.RancherCatalogRepo, "/")
	//generate and publish catalog entry
	template, err := github.NewGenericTemplate(rancherCatalogInfo[0], rancherCatalogInfo[1], p.GithubAccessToken)
	if err != nil {
		return err
	}
	var buildCatalogs []github.CatalogInfo

	for _, tag := range imageTags {
		if tag != "latest" {
			completedTemplate, err := template.SubBuildInfo(&p, tag)
			if err != nil {
				return err
			}
			info, err2 := completedTemplate.Commit(p.GithubAccessToken, rancherCatalogInfo[0], rancherCatalogInfo[1], p.Build.Number)
			if err2 != nil {
				return err2
			}
			buildCatalogs = append(buildCatalogs, *info)
		}
	}
	//output catalog entry info to temp file for downstream deployment plugin
	data, err := yaml.Marshal(&buildCatalogs)
	if err != nil {
		return err
	}
	if err = ioutil.WriteFile("/drone/.CatalogData.yml", []byte(data), 0644); err != nil {
		return err
	}

	return nil
}
