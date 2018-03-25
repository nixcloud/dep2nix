// Copyright 2018 The nixcloud.io Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/pelletier/go-toml"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

var (
	data   *os.File
	part   []byte
	count  int
	buffer *bytes.Buffer
)

var (
	inputFileFlag  = flag.String("i", "Gopkg.lock", "input lock file")
	outputFileFlag = flag.String("o", "deps.nix", "output nix file")
)

// FindRealPath queries url to try to locate real vcs path
// from `go help importpath`
// ...
// A few common code hosting sites have special syntax:
//
//         Bitbucket (Git, Mercurial)
//
//                 import "bitbucket.org/user/project"
//                 import "bitbucket.org/user/project/sub/directory"
//
//         GitHub (Git)
//
//                 import "github.com/user/project"
//                 import "github.com/user/project/sub/directory"
//
//         Launchpad (Bazaar)
//
//                 import "launchpad.net/project"
//                 import "launchpad.net/project/series"
//                 import "launchpad.net/project/series/sub/directory"
//
//                 import "launchpad.net/~user/project/branch"
//                 import "launchpad.net/~user/project/branch/sub/directory"
//
//         IBM DevOps Services (Git)
//
//                 import "hub.jazz.net/git/user/project"
//                 import "hub.jazz.net/git/user/project/sub/directory"
//
// ...
// If the import path is not a known code hosting site and also lacks a
// version control qualifier, the go tool attempts to fetch the import
// over https/http and looks for a <meta> tag in the document's HTML
// <head>.
//
// The meta tag has the form:
//
//         <meta name="go-import" content="import-prefix vcs repo-root">
// ...
// The repo-root is the root of the version control system
// containing a scheme and not containing a .vcs qualifier.
//
// For example,
//
//         import "example.org/pkg/foo"
//
// will result in the following requests:
//
//         https://example.org/pkg/foo?go-get=1 (preferred)
//         http://example.org/pkg/foo?go-get=1  (fallback, only with -insecure)
//
// If that page contains the meta tag
//
//         <meta name="go-import" content="example.org git https://code.org/r/p/exproj">b
//
func FindRealPath(url string) (string, error) {
	// golang http client will follow redirects, so if http don't work should query https if 301 redirect
	resp, err := http.Get("http://" + url + "?go-get=1")
	if err != nil {
		return "", fmt.Errorf("Failed to query %v", url)
	}
	defer resp.Body.Close()

	z := html.NewTokenizer(resp.Body)
	for {
		tt := z.Next()

		switch {
		case tt == html.ErrorToken:
			// End of the document, we're done
			return "", fmt.Errorf("end of body")
		case tt == html.StartTagToken:
			t := z.Token()

			// Check if the token is an <meta> tag
			isMeta := t.Data == "meta"
			if !isMeta {
				continue
			}

			// Extract vcs url
			for _, a := range t.Attr {
				if a.Key == "name" && a.Val == "go-import" {
					var content []string
					for _, b := range t.Attr {
						if b.Key == "content" {
							content = strings.Fields(b.Val)
						}
					}

					if len(content) < 3 {
						return "", fmt.Errorf("could not find content attribute for meta tag")
					}

					// go help importpath
					// content[0] : original import path
					// content[1] : vcs type
					// content[2] : vcs url

					// expand for non git vcs
					if content[1] == "git" {
						return content[2], nil
					}
					return "", fmt.Errorf("could not find git url")
				}
			}
		}
	}

}

// IsCommonPath checks to see if it's one of the common vcs locations go get supports
// see `go help importpath`
func IsCommonPath(url string) bool {
	// from `go help importpath`
	commonPaths := [...]string{
		"bitbucket.org",
		"github.com",
		"launchpad.net",
		"hub.jazz.net",
	}
	for _, path := range commonPaths {
		if strings.Split(url, "/")[0] == path {
			return true
		}
	}
	return false
}

func main() {
	flag.Parse()

	inFile, err := filepath.Abs(*inputFileFlag)
	if err != nil {
		log.Fatalln("Invalid input file path:", err.Error())
	}

	outFile, err := filepath.Abs(*outputFileFlag)
	if err != nil {
		log.Fatalln("Invalid output file path:", err.Error())
	}

	data, err = os.Open(inFile)
	if err != nil {
		log.Fatalln("Error opening input file:", err.Error())
	}
	defer data.Close()

	reader := bufio.NewReader(data)
	buffer = bytes.NewBuffer(make([]byte, 0))
	part = make([]byte, 1024)

	for {
		if count, err = reader.Read(part); err != nil {
			break
		}
		buffer.Write(part[:count])
	}
	if err != io.EOF {
		log.Fatalln("Error reading input file:", err.Error())
	}

	raw := rawLock{}
	err = toml.Unmarshal(buffer.Bytes(), &raw)
	if err != nil {
		log.Fatalln("Error parsing lock file:", err.Error())
	}
	//fmt.Println(raw.Projects)

	fmt.Printf("Found %d libraries to process: \n", len(raw.Projects))

	for i := 0; i < len(raw.Projects); i++ {
		t := raw.Projects[i]
		fmt.Println(t.Name)
	}
	fmt.Print("\n\n")

	var godepnix string

	godepnix += `
  # file automatically generated from Gopkg.lock with https://github.com/nixcloud/dep2nix (golang dep)
  [
  `

	for i := 0; i < len(raw.Projects); i++ {

		t := raw.Projects[i]

		var url string
		// check if it's a common git path `go get` supports and if not find real path
		if !IsCommonPath(t.Name) {
			realURL, err := FindRealPath(t.Name)

			if err != nil {
				//fmt.Printf("could not find real git url for import path %v: %+v\n", t.Name, err)
				log.Fatal(err)
			}
			url = realURL
		} else {
			url = "https://" + t.Name
		}

		fmt.Println(" * Processing: \"" + t.Name + "\"")

		cmd := exec.Command("nix-prefetch-git", url, "--rev", t.Revision, "--quiet")
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		if err != nil {
			log.Fatal(err)
		}

		type response struct {
			Url             string `json:"url"`
			Rev             string `json:"rev"`
			Date            string `json:"date"`
			SHA256          string `json:"sha256"`
			FetchSubmodules bool   `json:"fetchSubmodules"`
		}

		var jsonStr = out.String()
		var res response
		err1 := json.Unmarshal([]byte(jsonStr), &res)

		if err != nil {
			fmt.Println("There was a problem in decoding the result from nix-prefetch-git returned JSON:")
			fmt.Println(jsonStr)
			fmt.Println(err1)
			os.Exit(1)
		}

		//fmt.Println(res)

		godepnix += `
    {
      goPackagePath  = "` + t.Name + `";
      fetch = {
        type = "git";
        url = "` + url + `";
        rev =  "` + res.Rev + `";
        sha256 = "` + res.SHA256 + `";
      };
    }
    `
	}

	godepnix += "\n]"
	//fmt.Println(godepnix)

	f, _ := os.Create(outFile)
	defer f.Close()

	_, _ = f.WriteString(godepnix)
	fmt.Printf("\n -> Wrote %s, everything fine!\n", outFile)

	os.Exit(0)
}
