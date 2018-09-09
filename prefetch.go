package main

import (
	"bytes"
	"encoding/json"
	"github.com/Masterminds/vcs"
	"os/exec"
	"strings"
)

type Prefetcher interface {
	fetchHash(url string, revision string) (string, error)
}

func PrefetcherFor(typ vcs.Type) Prefetcher {
	switch typ {
	case vcs.Git:
		return &gitPrefetcher{}
	case vcs.Hg:
		return &hgPrefetcher{}
	default:
		return nil
	}
}

func cmdStdout(command string, arguments ...string) (string, error) {
	cmd := exec.Command(command, arguments...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}

	return out.String(), nil
}

type gitPrefetcher struct{}

func (p *gitPrefetcher) fetchHash(url string, revision string) (string, error) {
	out, err := cmdStdout("nix-prefetch-git", "--url", url, "--rev", revision, "--quiet")
	if err != nil {
		return "", err
	}

	// extract hash from response
	res := &struct {
		SHA256 string `json:"sha256"`
	}{}
	if err := json.Unmarshal([]byte(out), res); err != nil {
		return "", err
	}

	return res.SHA256, nil
}

type hgPrefetcher struct{}

func (p *hgPrefetcher) fetchHash(url string, revision string) (string, error) {
	out, err := cmdStdout("nix-prefetch-hg", url, revision)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(out), nil
}
