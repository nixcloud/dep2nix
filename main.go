// Copyright 2018 The nixcloud.io Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"github.com/Masterminds/vcs"
	"github.com/golang/dep"
	"github.com/golang/dep/gps"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	inputFileFlag  = flag.String("i", "Gopkg.lock", "input lock file")
	outputFileFlag = flag.String("o", "deps.nix", "output nix file")
)

func main() {
	logger := log.New(os.Stdout, "", 0)

	startTime := time.Now()
	if err := perform(logger); err != nil {
		logger.Fatalln(err.Error())
	}

	logger.Printf("Finished execution in %s.\n", time.Since(startTime).Round(time.Second).String())
}

func perform(logger *log.Logger) error {
	flag.Parse()

	// parse input file path
	inFile, err := filepath.Abs(*inputFileFlag)
	if err != nil {
		return fmt.Errorf("invalid input file path: %s", err.Error())
	}

	// parse output file path
	outFile, err := filepath.Abs(*outputFileFlag)
	if err != nil {
		return fmt.Errorf("invalid output file path: %s", err.Error())
	}

	// parse lock file
	f, err := os.Open(inFile)
	if err != nil {
		return fmt.Errorf("could not open input file: %s", err.Error())
	}
	defer f.Close()

	lock, err := dep.ReadLock(f)
	if err != nil {
		return fmt.Errorf("could not parse lock file: %s", err.Error())
	}

	logger.Printf("Found %d projects to process.\n", len(lock.Projects()))

	// create temporary directory for source manager cache
	cachedir, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		return fmt.Errorf("error creating cache directory: %s", err)
	}
	defer os.RemoveAll(cachedir)

	// create source manager
	sm, err := gps.NewSourceManager(gps.SourceManagerConfig{
		Cachedir: cachedir,
		Logger:   logger,
	})
	if err != nil {
		return fmt.Errorf("error creating source manager: %s", err)
	}

	// Process all projects, converting them into deps
	var deps Deps
	for _, project := range lock.Projects() {
		logger.Printf("* Processing: \"%s\"\n", project.Ident().ProjectRoot)

		// get repository for project
		src, err := sm.SourceFor(project.Ident())
		if err != nil {
			return fmt.Errorf("error deducing project source: %s", err.Error())
		}
		repo := src.Repo()

		// convert vcs type to vcs.Type to avoid
		// type mismatches caused by vendoring
		typ := vcs.Type(repo.Vcs())

		// get prefetcher for vcs type
		prefetcher := PrefetcherFor(typ)
		if prefetcher == nil {
			return fmt.Errorf("only repositories of type \"%s\" and \"%s\" are supported "+
				"- detected repository type \"%s\"\n", vcs.Git, vcs.Hg, typ)
		}

		// check out repository
		if err := repo.Get(); err != nil {
			return fmt.Errorf("error fetching project: %s", err.Error())
		}

		// get resolved revision
		rev, _, _ := gps.VersionComponentStrings(project.Version())

		// We intend to use this fetched version of the repository
		// with nix-prefetch-git. When cloning a repository in git,
		// all remote branches become tracking branches in the
		// clone. When cloning a checkout, the only remote branches
		// are the local branches in the checkout, and in our case
		// this is the default branch upstream. nix-prefetch-git will
		// clone our checkout, and only see a single branch
		// corresponding to HEAD. If `rev` is not an ancestor of the
		// default upstream branch, nix-prefetch-git will not be able
		// to find the revision. To fix this, we set the default
		// revision of the first checkout to `rev`.
		if err := repo.UpdateVersion(rev); err != nil {
			return fmt.Errorf("error checking out branch: %s", err.Error())
		}

		// use locally fetched repository as remote for nix-prefetch
		// to avoid it being downloaded from the remote again
		localUrl := fmt.Sprintf("file://%s", repo.LocalPath())

		// use nix-prefetch to get the hash of the checkout
		hash, err := prefetcher.fetchHash(localUrl, rev)
		if err != nil {
			return fmt.Errorf("error prefetching hash: %s", err.Error())
		}

		// create dep instance
		deps = append(deps, &Dep{
			PackagePath: string(project.Ident().ProjectRoot),
			VCS:         string(typ),
			URL:         src.Repo().Remote(),
			Revision:    rev,
			SHA256:      hash,
		})

		// clean the contents of the source manager cache directory,
		// as the downloaded project is no longer needed
		// and only takes up disk space.
		files, err := ioutil.ReadDir(cachedir)
		if err != nil {
			return fmt.Errorf("error reading cache dir: %s", err.Error())
		}

		for _, f := range files {
			os.RemoveAll(f.Name())
		}
	}

	// write deps to output file
	out, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("error creating output file: %s", err.Error())
	}
	defer out.Close()

	if _, err := out.WriteString(deps.toNix()); err != nil {
		return fmt.Errorf("error writing output file: %s", err.Error())
	}

	logger.Printf("\n -> Wrote to %s.\n", outFile)
	return nil
}
