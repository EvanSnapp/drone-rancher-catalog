package github

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/LeanKit-Labs/drone-rancher-catalog/types"
	"github.com/asaskevich/govalidator"
	"github.com/google/go-github/github"
)

type CatalogInfo struct {
	CatalogRepo string `yaml:"catalogRepo"`
	Version     int    `yaml:"version"`
	Branch      string `yaml:"branch"`
}

//Template a data struture to store the generic catalog template
type GenericTemplate struct {
	Config         string
	DockerCompose  string
	RancherCompose string
	Icon           []byte
}

//BuiltTemplate a data struture to store the catalog template with agruments
type BuiltTemplate struct {
	branch         string
	tag            string
	Config         string
	DockerCompose  string
	RancherCompose string
	Icon           []byte
}

type folder struct {
	Name string `json:"name"`
	Url  string `json:"url"`
}

type tmplArguments struct {
	Branch string
	Tag    string
	Count  int
}

type templateFile struct {
	Name        string `json:"name"`
	DownloadURL string `json:"download_url"`
}

func getBytesFromURL(client *http.Client, url string, token string) ([]byte, int, error) {
	//build request
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, -1, err
	}
	request.SetBasicAuth(token, "x-oauth-basic")
	request.Close = true

	//run request
	response, err := client.Do(request)
	if err != nil {
		return nil, response.StatusCode, err
	}

	//parse response
	contents, err := ioutil.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		return nil, response.StatusCode, err
	}
	return contents, response.StatusCode, nil
}

//NewGenericTemplate gets the Template from github
func NewGenericTemplate(owner string, repo string, token string) (*GenericTemplate, error) {
	//build url
	templateFolderURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/base", owner, repo)
	if !govalidator.IsURL(templateFolderURL) {
		return nil, errors.New("Github Owner and Repo invalid")
	}

	//get directory
	client := &http.Client{
		Timeout: time.Second * 60,
	}
	var templateDir []templateFile
	contents, _, err := getBytesFromURL(client, templateFolderURL, token)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(contents, &templateDir)

	//download files
	var result GenericTemplate
	for _, file := range templateDir {
		switch file.Name {
		case "catalogIcon.png":
			fileContents, _, err := getBytesFromURL(client, file.DownloadURL, token)
			if err != nil {
				return nil, err
			}
			result.Icon = fileContents
		case "config.tmpl":
			fileContents, _, err := getBytesFromURL(client, file.DownloadURL, token)
			if err != nil {
				return nil, err
			}
			result.Config = string(fileContents)
		case "docker-compose.tmpl":
			fileContents, _, err := getBytesFromURL(client, file.DownloadURL, token)
			if err != nil {
				return nil, err
			}
			result.DockerCompose = string(fileContents)
		case "rancher-compose.tmpl":
			fileContents, _, err := getBytesFromURL(client, file.DownloadURL, token)
			if err != nil {
				return nil, err
			}
			result.RancherCompose = string(fileContents)
		}
	}
	if len(result.Icon) == 0 {
		return nil, errors.New("catalogIcon.png not found")
	}
	if len(result.DockerCompose) == 0 {
		return nil, errors.New("docker-compose.tmpl not found")
	}
	if len(result.RancherCompose) == 0 {
		return nil, errors.New("rancher-compose.tmpl not found")
	}
	if len(result.Config) == 0 {
		return nil, errors.New("config.tmpl not found")
	}

	return &result, nil

}

func fixTemplate(args *tmplArguments, name string, templatedString string) (string, error) {
	tmpl, err := template.New(name).Parse(templatedString)
	if err != nil {
		return "", err
	}

	var doc bytes.Buffer
	if err := tmpl.Execute(&doc, *args); err != nil {
		return "", err
	}
	return doc.String(), nil
}

//SubBuildInfo replaces the templated values in the file
func (t *GenericTemplate) SubBuildInfo(p *types.Plugin, tag string) (*BuiltTemplate, error) {

	var final BuiltTemplate
	final.branch = p.Branch
	final.tag = tag

	final.Icon = t.Icon

	var args tmplArguments
	args.Branch = p.Branch
	args.Count = p.Build.Number
	args.Tag = tag

	val1, err1 := fixTemplate(&args, "docker-compose.yml", t.DockerCompose)
	if err1 != nil {
		return nil, err1
	}
	final.DockerCompose = val1
	val2, err2 := fixTemplate(&args, "rancher-compose.yml", t.RancherCompose)
	if err2 != nil {
		return nil, err2
	}
	final.RancherCompose = val2
	val3, err3 := fixTemplate(&args, "config.yml", t.Config)
	if err3 != nil {
		return nil, err3
	}
	final.Config = val3
	return &final, nil
}

//get template returns the next catalog number for a branch
// IF the catalog already exists: i.e build restarted
//	return -1 and and error of nil
// If there is an error
//  return -1 and the error
func getTemplateNum(client *http.Client, url string, token string, tag string) (int, error) {
	folderBytes, code, err := getBytesFromURL(client, url, token)
	if err != nil {
		return -1, err
	}
	if code == 404 {
		return 0, nil
	}
	var folders []folder
	currentTemplate := -1 //empty folder
	err = json.Unmarshal(folderBytes, &folders)
	if err != nil {
		return -1, err
	}

	for _, folder := range folders {
		if number, err := strconv.Atoi(folder.Name); err == nil {

			data, _, err := getBytesFromURL(client, folder.Url, token)
			if err != nil {
				return -1, err
			}

			//check if catalog is already built
			var files []templateFile
			json.Unmarshal(data, &files)
			for _, file := range files {
				if file.Name == "docker-compose.yml" {
					yamlData, _, err := getBytesFromURL(client, file.DownloadURL, token)
					if err != nil {
						return -1, err
					}
					if strings.Contains(string(yamlData), tag) {
						return -1, nil
					}
					break
				}
			}

			if number > currentTemplate {
				currentTemplate = number
			}
		}
	}
	return currentTemplate + 1, nil

}

func commitFile(githubClient *github.Client, owner string, repo string, path string, contents []byte, message string) error {
	branch := "master"
	opts := github.RepositoryContentFileOptions{
		Message: &message,
		Content: contents,
		Branch:  &branch,
	}
	_, _, err := githubClient.Repositories.CreateFile(owner, repo, path, &opts)
	if err != nil {
		return err
	}
	return nil
}

//Commit commits the file to github
func (t *BuiltTemplate) Commit(accessToken string, owner string, repo string, buildNum int) (*CatalogInfo, error) {

	token := oauth2.Token{AccessToken: accessToken}
	tokenSource := oauth2.StaticTokenSource(&token)
	oauthClient := oauth2.NewClient(oauth2.NoContext, tokenSource)
	githubClient := github.NewClient(oauthClient)

	client := &http.Client{
		Timeout: time.Second * 60,
	}
	number, err := getTemplateNum(client, fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/templates/%s", owner, repo, t.branch), accessToken, t.tag)
	if err != nil {
		return nil, err
	}
	if number != -1 { //check to make sure catalog is not already there: build resart
		if number == 0 { //on a new branch push the template
			if err = commitFile(githubClient, owner, repo, fmt.Sprintf("templates/%s/catalogIcon.png", t.branch), t.Icon, fmt.Sprintf("Drone Build #%d: Adding Icon", buildNum)); err != nil {
				return nil, err
			}
			if err = commitFile(githubClient, owner, repo, fmt.Sprintf("templates/%s/config.yml", t.branch), []byte(t.Config), fmt.Sprintf("Drone Build #%d: Adding config.yml", buildNum)); err != nil {
				return nil, err
			}
		}
		if err = commitFile(githubClient, owner, repo, fmt.Sprintf("templates/%s/%d/docker-compose.yml", t.branch, number), []byte(t.DockerCompose), fmt.Sprintf("Drone Build #%d: Changing docker-compose.yml", buildNum)); err != nil {
			return nil, err
		}
		if err = commitFile(githubClient, owner, repo, fmt.Sprintf("templates/%s/%d/rancher-compose.yml", t.branch, number), []byte(t.RancherCompose), fmt.Sprintf("Drone Build #%d: Changing rancher-compose.yml", buildNum)); err != nil {
			return nil, err
		}
	} else {
		fmt.Printf("Catalog found for Tag: %s, No catalog will be built.\n", t.tag)
	}
	var info CatalogInfo
	info.CatalogRepo = fmt.Sprintf("%s/%s", owner, repo)
	info.Version = number
	info.Branch = t.branch
	return &info, nil
}
