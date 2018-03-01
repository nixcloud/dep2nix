// Copyright 2018 The nixcloud.io Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pelletier/go-toml"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
)

var (
	data   *os.File
	part   []byte
	err    error
	count  int
	buffer *bytes.Buffer
)

func main() {
	inFile := "Gopkg.lock"
	outFile := "deps.nix"

	data, err = os.Open(inFile)
	if err != nil {
		log.Fatal(err)
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
		log.Fatal("Error Reading ", inFile, ": ", err)
	} else {
		err = nil
	}

	raw := rawLock{}
	err = toml.Unmarshal(buffer.Bytes(), &raw)
	if err != nil {
		//return nil, errors.Wrap(err, "Unable to parse the lock as TOML")
	}
	//fmt.Println(raw.Projects)

	fmt.Printf("Found %d libraries to process: \n", len(raw.Projects))

	for i := 0; i < len(raw.Projects); i++ {
		t := raw.Projects[i]
		fmt.Printf(t.Name + " ")
	}
	fmt.Printf("\n\n")

	var godepnix string

	godepnix += `
  # file automatically generated from Gopkg.lock with https://github.com/nixcloud/dep2nix (golang dep)
  [
  `

	for i := 0; i < len(raw.Projects); i++ {

		t := raw.Projects[i]

		// special case: exception for golang.org/x based dependencies
		url := "https://" + strings.Replace(t.Name, "golang.org/x/", "go.googlesource.com/", 1)

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
