// Copyright 2018 The nixcloud.io Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/golang/dep"
	"github.com/golang/dep/gps"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var (
	inputFileFlag  = flag.String("i", "Gopkg.lock", "input lock file")
	outputFileFlag = flag.String("o", "deps.nix", "output nix file")
)

func main() {
	flag.Parse()
	logger := log.New(os.Stdout, "", 0)

	defer func(start time.Time) {
		logger.Printf("Finished execution in %s.\n", time.Since(start).Round(time.Second).String())
	}(time.Now())

	// parse input file path
	inFile, err := filepath.Abs(*inputFileFlag)
	if err != nil {
		logger.Fatalln("Invalid input file path:", err.Error())
	}

	// parse output file path
	outFile, err := filepath.Abs(*outputFileFlag)
	if err != nil {
		logger.Fatalln("Invalid output file path:", err.Error())
	}

	// parse lock file
	f, err := os.Open(inFile)
	if err != nil {
		logger.Fatalln("Error opening input file:", err.Error())
	}
	defer f.Close()

	lock, err := dep.ReadLock(f)
	if err != nil {
		logger.Fatalln("Error parsing lock file:", err.Error())
	}

	logger.Printf("Found %d projects to process.\n", len(lock.Projects()))

	// create temporary directory for source manager cache
	cachedir, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		logger.Fatalln(err)
	}
	defer os.RemoveAll(cachedir)

	// create source manager
	sm, err := gps.NewSourceManager(gps.SourceManagerConfig{
		Cachedir: cachedir,
		Logger:   logger,
	})
	if err != nil {
		logger.Fatalln(err)
	}

	// Process all projects, converting them into deps
	var deps Deps
	for _, project := range lock.Projects() {
		fmt.Printf("* Processing: \"%s\"\n", project.Ident().ProjectRoot)

		// get repository for project
		src, err := sm.SourceFor(project.Ident())
		if err != nil {
			logger.Fatalln(err)
		}
		repo := src.Repo()

		// get vcs type
		typ := string(repo.Vcs())
		if typ != "git" {
			logger.Fatalln("non-git repositories are not supported yet")
		}

		// check out repository
		if err := repo.Get(); err != nil {
			logger.Fatalln("error fetching project:", err.Error())
		}

		// get resolved revision
		rev, _, _ := gps.VersionComponentStrings(project.Version())

		// use locally fetched repository as remote for nix-prefetch-git
		// to it being downloaded from the remote again
		localUrl := fmt.Sprintf("file://%s", repo.LocalPath())
		// use nix-prefetch-git to get the hash of the checkout
		cmd := exec.Command("nix-prefetch-git", "--url", localUrl, "--rev", rev, "--quiet")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			logger.Fatal(err)
		}
		// extract hash from response
		res := &struct {
			SHA256 string `json:"sha256"`
		}{}
		json.Unmarshal(out.Bytes(), res)

		// create dep instance
		deps = append(deps, &Dep{
			PackagePath: string(project.Ident().ProjectRoot),
			VCS:         string(typ),
			URL:         src.Repo().Remote(),
			Revision:    rev,
			SHA256:      res.SHA256,
		})
	}

	// write deps to output file
	out, err := os.Create(outFile)
	if err != nil {
		logger.Fatalln("Error creating output file:", err.Error())
	}
	defer out.Close()

	if _, err := out.WriteString(deps.toNix()); err != nil {
		logger.Fatalln("Error writing output file:", err.Error())
	}

	fmt.Printf("\n -> Wrote to %s.\n", outFile)
}
