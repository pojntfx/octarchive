package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"time"

	"github.com/go-git/go-git/v5"
	gtransport "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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

	verbose := flag.Int("verbose", 5, "Verbosity level (0 is disabled, default is info, 7 is trace)")
	orgs := flag.Bool("orgs", false, "Also clone repos of all orgs that the user is part of")
	api := flag.String("api", "https://api.github.com/", "GitHub/Gitea API endpoint to use (can also be set using the GITHUB_API env variable)")
	token := flag.String("token", "", "GitHub/Gitea API access token (can also be set using the GITHUB_TOKEN env variable)")
	dst := flag.String("dst", filepath.Join(home, ".local", "share", "octarchive", "var", "lib", "octarchive", "data"), "Base directory to clone repos into")
	timestamp := flag.String("timestamp", fmt.Sprintf("%v", time.Now().Unix()), "Timestamp to use as the directory for this clone session")
	concurrency := flag.Int("concurrency", runtime.NumCPU(), "Maximum amount of repositories to clone concurrently")

	flag.Parse()

	switch *verbose {
	case 0:
		zerolog.SetGlobalLevel(zerolog.Disabled)
	case 1:
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	case 2:
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	case 3:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case 4:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case 5:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case 6:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	}

	if *api == "" {
		*api = os.Getenv("GITHUB_API")
	}

	if *token == "" {
		*token = os.Getenv("GITHUB_TOKEN")
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

	log.Info().Msg("Getting user")

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

	log.Debug().
		Str("user", user.Login).
		Msg("Got user")

	log.Info().Msg("Getting organizations for user")

	slugs := []string{user.Login}
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
			slugs = append(slugs, organization.Login)
		}
	}

	log.Debug().
		Strs("organizations", slugs).
		Msg("Got organizations for user")

	var repos []struct {
		filePath string
		cloneURL string
	}
	for _, slug := range slugs {
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
			log.Debug().
				Str("organization", slug).
				Str("fullName", repo.FullName).
				Str("cloneURL", repo.CloneURL).
				Msg("Got repo for organization")

			username, repoName := path.Split(repo.FullName)

			repos = append(repos, struct {
				filePath string
				cloneURL string
			}{
				filePath: filepath.Join(*dst, *timestamp, username, repoName),
				cloneURL: repo.CloneURL,
			})
		}

	}

	sem := make(chan int, *concurrency)
	for _, repo := range repos {
		sem <- 1

		go func(repo struct {
			filePath string
			cloneURL string
		}) {
			log.Info().
				Str("cloneURL", repo.cloneURL).
				Str("filePath", repo.filePath).
				Msg("Cloning repo")
			if err := os.RemoveAll(repo.filePath); err != nil {
				panic(err)
			}
			if err := os.MkdirAll(repo.filePath, os.ModePerm); err != nil {
				panic(err)
			}
			if _, err := git.PlainClone(repo.filePath, false, &git.CloneOptions{
				Progress: os.Stderr,
				URL:      repo.cloneURL,
				Auth: &gtransport.BasicAuth{
					Username: user.Login,
					Password: *token,
				},
			}); err != nil {
				panic(err)
			}
			log.Info().
				Str("cloneURL", repo.cloneURL).
				Str("filePath", repo.filePath).
				Msg("Cloned repo")

			<-sem
		}(repo)
	}
}
