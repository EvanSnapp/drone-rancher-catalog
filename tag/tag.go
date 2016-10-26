package tag

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/LeanKit-Labs/drone-rancher-catalog/types"
	"github.com/blang/semver"
)

//vars for reading semVer data
type projectJSON struct {
	Version string `json:"version"`
}

var fileInfo struct {
	Version string `json:"version"`
}

func getJSONVersionReader(fname string) func() (string, error) {
	return func() (string, error) {
		fileData, err := os.Open(fname)
		if err != nil {
			return "", err
		}

		jsonParser := json.NewDecoder(fileData)
		if err = jsonParser.Decode(&fileInfo); err != nil {
			return "", err
		}

		return fileInfo.Version, nil
	}
}

func replaceUnderscores(str string) string {
	return strings.Replace(str, "_", "-", -1)
}

//CreateDockerImageTags takes plugin information and returns a list
//of tags to use when publishing the project image to Docker Hub
//TODO: this function might not need to take an error
func CreateDockerImageTags(p types.Plugin, versionString string) ([]string, error) {

	releaseVersion, err := semver.Make(versionString)
	if err != nil {
		return []string{}, err
	}

	//handle master tag
	if p.Branch == "master" {
		//unfortnatnly docker does not support '+' character to seperate build data
		//This will use a dash to seperate the build metadata from the smver
		return []string{fmt.Sprintf("v%s", releaseVersion.String()), fmt.Sprintf("v%s-drone.build.%d", releaseVersion.String(), p.Build.Number), "latest"}, nil
	}

	//return the long tag
	//githubOwner_githubRepo_branch_semVer_build_commit
	return []string{fmt.Sprintf("%s_%s_%s_%s_%d_%s", replaceUnderscores(p.Repo.Owner), replaceUnderscores(p.Repo.Name), replaceUnderscores(p.Build.Branch), releaseVersion.String(), p.Build.Number, p.Build.Commit[:7])}, nil

}
