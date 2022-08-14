package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
)

var (
	errInvalidAPIResponse = errors.New("invalid API response")
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	orgs := flag.Bool("orgs", false, "Also clone repos of all orgs that the user is part of")
	api := flag.String("api", "https://api.github.com/", "GitHub/Gitea API endpoint to use (can also be set using the GITHUB_API env variable)")
	token := flag.String("token", "", "GitHub/Gitea API access token (can also be set using the GITHUB_TOKEN env variable)")
	dst := flag.String("dst", filepath.Join(home, ".local", "share", "octarchive", "var", "lib", "octarchive", "data"), "Directory to clone repos into")

	flag.Parse()

	if !*orgs {
		rawOrgs := os.Getenv("GITHUB_ORGS")

		if rawOrgs == "TRUE" {
			*orgs = true
		} else {
			*orgs = false
		}
	}

	if *api == "" {
		*api = os.Getenv("GITHUB_API")
	}

	if *token == "" {
		*token = os.Getenv("GITHUB_TOKEN")
	}

	if *dst == "" {
		*dst = os.Getenv("GITHUB_DST")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var ghttp *http.Client
	if *token != "" {
		ghttp = oauth2.NewClient(
			ctx,
			oauth2.StaticTokenSource(
				&oauth2.Token{
					AccessToken: *token,
				},
			),
		)
	}

	res, err := ghttp.Get(fmt.Sprintf("%vuser", *api))
	if err != nil {
		panic(err)
	}
	if res.Body == nil {
		panic(errInvalidAPIResponse)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		panic(errors.New(res.Status))
	}

	var user struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(res.Body).Decode(&user); err != nil {
		panic(err)
	}

	slugsToGetReposFor := []string{user.Login}
	if *orgs {
		res, err := ghttp.Get(fmt.Sprintf("%vuser/orgs", *api))
		if err != nil {
			panic(err)
		}
		if res.Body == nil {
			panic(errInvalidAPIResponse)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			panic(errors.New(res.Status))
		}

		var organizations []struct {
			Login string `json:"login"`
		}
		if err := json.NewDecoder(res.Body).Decode(&organizations); err != nil {
			panic(err)
		}

		for _, organization := range organizations {
			slugsToGetReposFor = append(slugsToGetReposFor, organization.Login)
		}
	}

	var repos []struct {
		path     string
		cloneURL string
	}
	for _, slug := range slugsToGetReposFor {
		res, err := ghttp.Get(fmt.Sprintf("%vusers/%v/repos", *api, slug))
		if err != nil {
			panic(err)
		}
		if res.Body == nil {
			panic(errInvalidAPIResponse)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			panic(errors.New(res.Status))
		}

		var orgRepos []struct {
			FullName string `json:"full_name"`
			CloneURL string `json:"clone_url"`
		}
		if err := json.NewDecoder(res.Body).Decode(&orgRepos); err != nil {
			panic(err)
		}

		for _, repo := range orgRepos {
			repos = append(repos, struct {
				path     string
				cloneURL string
			}{
				path:     repo.FullName,
				cloneURL: repo.CloneURL,
			})
		}

	}

	fmt.Println(repos)
}
