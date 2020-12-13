package main

import (
	"fmt"
)

const depNixFormat = `
  {
    goPackagePath  = "%s";
    fetch = {
      type = "%s";
      url = "%s";
      rev =  "%s";
      sha256 = "%s";
    };
  }`

// Dep represents a project dependency
// to write to deps.nix.
type Dep struct {
	PackagePath string
	VCS         string
	URL         string
	Revision    string
	SHA256      string
}

// toNix converts d into a nix set
// for use in the generated deps.nix.
func (d *Dep) toNix() string {
	return fmt.Sprintf(depNixFormat,
		d.PackagePath, d.VCS, d.URL,
		d.Revision, d.SHA256)
}

const depsFileHeader = `# file generated from Gopkg.lock using dep2nix (https://github.com/nixcloud/dep2nix)
[`
const depsFileFooter = `
]
`

type Deps []*Dep

// toNix converts d into a deps.nix file
// for use with pkgs.buildGoPackage.
func (d Deps) toNix() string {
	nix := depsFileHeader
	for _, dep := range d {
		nix += dep.toNix()
	}
	nix += depsFileFooter
	return nix
}
