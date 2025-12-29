package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	gtransport "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/oauth2"
	"golang.org/x/sync/semaphore"
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
	api := flag.String("api", "https://api.github.com/", "GitHub/Forgejo API endpoint to use (can also be set using the FORGE_API env variable)")
	token := flag.String("token", "", "GitHub/Forgejo API access token (can also be set using the FORGE_TOKEN env variable)")
	dst := flag.String("dst", filepath.Join(home, ".local", "share", "octarchive", "var", "lib", "octarchive", "data"), "Base directory to clone repos into")
	timestamp := flag.String("timestamp", fmt.Sprintf("%v", time.Now().Unix()), "Timestamp to use as the directory for this clone session")
	fresh := flag.Bool("fresh", false, "Clear timestamp directory before starting to clone")
	concurrency := flag.Int64("concurrency", int64(runtime.NumCPU()), "Maximum amount of repositories to clone concurrently")

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

	if apiEnv := os.Getenv("FORGE_API"); apiEnv != "" {
		*api = apiEnv
	}

	if tokenEnv := os.Getenv("FORGE_TOKEN"); tokenEnv != "" {
		*token = tokenEnv
	}

	if strings.TrimSpace(*token) == "" {
		panic("missing token")
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

	u, err := url.JoinPath(*api, "user")
	if err != nil {
		panic(err)
	}

	res, err := ghttp.Get(u)
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
		page := 1
		for {
			u, err := url.JoinPath(*api, "user", "orgs")
			if err != nil {
				panic(err)
			}

			parsed, err := url.Parse(u)
			if err != nil {
				panic(err)
			}

			q := parsed.Query()
			q.Set("per_page", "100")
			q.Set("page", fmt.Sprintf("%v", page))
			parsed.RawQuery = q.Encode()

			res, err := ghttp.Get(parsed.String())
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

			if len(organizations) < 100 {
				break
			}

			page++
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
		page := 1
		for {
			res, err := ghttp.Get(fmt.Sprintf("%vusers/%v/repos?per_page=100&page=%v", *api, slug, page))
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

			if len(orgRepos) < 100 {
				break
			}

			page++
		}
	}

	bar := progressbar.NewOptions(
		len(repos),
		progressbar.OptionSetDescription("Cloning"),
		progressbar.OptionSetItsString("repo"),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionFullWidth(),
		// VT-100 compatibility
		progressbar.OptionUseANSICodes(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	if *fresh {
		if err := os.RemoveAll(filepath.Join(*dst, *timestamp)); err != nil {

			panic(err)
		}
	}

	sem := semaphore.NewWeighted(*concurrency)
	for _, repo := range repos {
		sem.Acquire(ctx, 1)

		go func(repo struct {
			filePath string
			cloneURL string
		}) {
			defer func() {
				bar.Add(1)

				sem.Release(1)
			}()

			log.Info().
				Str("cloneURL", repo.cloneURL).
				Str("filePath", repo.filePath).
				Msg("Cloning repo")

			bar.RenderBlank()

			if err := os.RemoveAll(repo.filePath); err != nil {
				panic(err)
			}

			if err := os.MkdirAll(repo.filePath, os.ModePerm); err != nil {
				panic(err)
			}

			if _, err := git.PlainClone(repo.filePath, false, &git.CloneOptions{
				Progress: func() io.Writer {
					if *verbose > 5 {
						return os.Stderr
					}

					return nil
				}(),
				URL: repo.cloneURL,
				Auth: &gtransport.BasicAuth{
					Username: user.Login,
					Password: *token,
				},
			}); err != nil {
				if err.Error() == "remote repository is empty" {
					log.Info().
						Str("cloneURL", repo.cloneURL).
						Str("filePath", repo.filePath).
						Msg("Skipped empty repo")

					return
				}

				panic(err)
			}

			log.Info().
				Str("cloneURL", repo.cloneURL).
				Str("filePath", repo.filePath).
				Msg("Cloned repo")
		}(repo)
	}

	if err := sem.Acquire(ctx, *concurrency); err != nil {
		panic(err)
	}
}
