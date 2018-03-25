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
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml"
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
func FindRealPath(url string) (string, error) {
	// golang http client will follow redirects, so if http don't work should query https if 301 redirect
	var resp *http.Response
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
func IsCommonPath(url string) bool {
	// from `go help importpath`
	commonPaths := [...]string{
		"golang.org",
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
		}

		// special case: exception for golang.org/x based dependencies
		if strings.Contains(t.Name, "golang.org/x/") {
			url = "https://" + strings.Replace(t.Name, "golang.org/x/", "go.googlesource.com/", 1)
		}

		if url == "" {
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
