package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
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

	verbosity := flag.String("verbosity", "info", "Log level (debug, info, warn, error)")
	orgs := flag.Bool("orgs", false, "Also clone repos of all orgs that the user is part of")
	api := flag.String("api", "https://api.github.com/", "GitHub/Forgejo API endpoint to use (can also be set using the FORGE_API env variable)")
	token := flag.String("token", "", "GitHub/Forgejo API access token (can also be set using the FORGE_TOKEN env variable)")
	dst := flag.String("dst", filepath.Join(home, ".local", "share", "octarchive", "var", "lib", "octarchive", "data"), "Base directory to clone repos into")
	timestamp := flag.String("timestamp", fmt.Sprintf("%v", time.Now().Unix()), "Timestamp to use as the directory for this clone session")
	fresh := flag.Bool("fresh", false, "Clear timestamp directory before starting to clone")
	concurrency := flag.Int64("concurrency", int64(runtime.NumCPU()), "Maximum amount of repositories to clone concurrently")

	flag.Parse()

	var level slog.Level
	if err := level.UnmarshalText([]byte(*verbosity)); err != nil {
		panic(err)
	}

	log := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))

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

	u, err := url.JoinPath(*api, "user")
	if err != nil {
		panic(err)
	}

	log = log.With("url", u)

	log.Info("Getting user")

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

	log.Debug("Got user", "user", user.Login)

	log.Info("Getting organizations for user")

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

			log := log.With("url", parsed.String())

			log.Debug("Fetching organizations page", "page", page)

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

	log.Debug("Got organizations for user", "organizations", slugs)

	var repos []struct {
		filePath string
		cloneURL string
	}
	for _, slug := range slugs {
		page := 1
		for {
			u, err := url.JoinPath(*api, "users", slug, "repos")
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

			log := log.With("url", parsed.String())

			log.Debug("Fetching repos page", "slug", slug, "page", page)

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

			var orgRepos []struct {
				FullName string `json:"full_name"`
				CloneURL string `json:"clone_url"`
			}
			if err := json.NewDecoder(res.Body).Decode(&orgRepos); err != nil {
				panic(err)
			}

			for _, repo := range orgRepos {
				log.Debug("Got repo for organization", "organization", slug, "fullName", repo.FullName, "cloneURL", repo.CloneURL)

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

			log.Info("Cloning repo", "cloneURL", repo.cloneURL, "filePath", repo.filePath)

			bar.RenderBlank()

			if err := os.RemoveAll(repo.filePath); err != nil {
				panic(err)
			}

			if err := os.MkdirAll(repo.filePath, os.ModePerm); err != nil {
				panic(err)
			}

			if _, err := git.PlainClone(repo.filePath, false, &git.CloneOptions{
				Progress: func() io.Writer {
					if level == slog.LevelDebug {
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
					log.Info("Skipped empty repo", "cloneURL", repo.cloneURL, "filePath", repo.filePath)

					return
				}

				panic(err)
			}

			log.Info("Cloned repo", "cloneURL", repo.cloneURL, "filePath", repo.filePath)
		}(repo)
	}

	if err := sem.Acquire(ctx, *concurrency); err != nil {
		panic(err)
	}
}
