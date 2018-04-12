# dep2nix

`dep2nix` converts a `Gopkgs.lock` file into a `deps.nix` file which is understood by nixpkgs's go abstraction thanks to [go2nix](https://github.com/kamilchm/go2nix) effort.

In other words: For go projects using [golang dep](https://github.com/golang/dep) already, it is fairly trivial to get these projects to compile with Nix/NixOS.

# Usage

## Installation

    git clone https://github.com/nixcloud/dep2nix
    cd dep2nix
    nix-env -f default.nix -i dep2nix
    nix-env -i nix-prefetch-git    (this step might not be needed anymore soon)

## Using dep2nix

    cd yourproject   (contains the Gopkg.lock)
    
    dep2nix
    Found 14 libraries to process: 
    github.com/Masterminds/semver github.com/Masterminds/vcs github.com/armon/go-radix github.com/boltdb/bolt github.com/golang/dep github.com/golang/protobuf github.com/jmank88/nuts github.com/nightlyone/lockfile github.com/pelletier/go-toml github.com/pkg/errors github.com/sdboyer/constext golang.org/x/net golang.org/x/sync golang.org/x/sys 

    * Processing: "github.com/Masterminds/semver"
    * Processing: "github.com/Masterminds/vcs"
    * Processing: "github.com/armon/go-radix"
    * Processing: "github.com/boltdb/bolt"
    * Processing: "github.com/golang/dep"
    * Processing: "github.com/golang/protobuf"
    * Processing: "github.com/jmank88/nuts"
    * Processing: "github.com/nightlyone/lockfile"
    * Processing: "github.com/pelletier/go-toml"
    * Processing: "github.com/pkg/errors"
    * Processing: "github.com/sdboyer/constext"
    * Processing: "golang.org/x/net"
    * Processing: "golang.org/x/sync"
    * Processing: "golang.org/x/sys"

    -> Wrote deps.nix, everything fine!

    
`dep2nix` has created a `deps.nix` file similar to the one originally created by `go2nix` tool. If you wonder how to compile your GO based project with nixpkgs just look into the source code of this project itself at https://github.com/nixcloud/dep2nix

Note: It is good practice to keep the `Gopkg.lock`, `Gopkg.toml` as well as the `deps.nix` in the GIT repository of your project. 
    
## Using dep

    nix-shell -p go dep
    cd yourproject
    dep init
    dep2nix
    
# Issues

## Building the source

On one test machine dep2nix wouldn't build with the error that lockfile had a 'wrong' sha256:

    {
      goPackagePath  = "github.com/nightlyone/lockfile";
      fetch = {
        type = "git";
        url = "https://github.com/nightlyone/lockfile";
        rev =  "6a197d5ea61168f2ac821de2b7f011b250904900";
        sha256 = "0z3bdl5hb7nq2pqx7zy0r47bcdvjw0y11jjphv7k0s09ahlnac29";
      };
    }

But running `nix-prefetch-git --url https://github.com/nightlyone/lockfile --rev 6a197d5ea61168f2ac821de2b7f011b250904900` showed it was actually correct.

After running `nix-build` it showed the `/nix/store/krjm0nanydmyyddx222vn443hq13fsis-dep2nix.drv` target and then this call:

    nix-store --delete /nix/store/krjm0nanydmyyddx222vn443hq13fsis-dep2nix.drv
    
Resets the complete build artifacts and afterwards it was working. No clue what is going on there...

## nix-prefetch-git 

returns bogus for not existent git repos as golang.org/x/net for example, this needs to return at least erropr code != 0

 * https://github.com/NixOS/nixpkgs/pull/35017

## golang dep (git repo hash extension)

 * add hashes to dependencies (hash per git repo)

    * this is similar to yarn2nix / yarn.lock way):
    
      there they are using tar.bz2 bundles but from the npm registry IIRC
    
      * https://github.com/moretea/yarn2nix/blob/master/yarn.lock
      * https://github.com/moretea/yarn2nix/blob/master/yarn.nix
      
      note: since npm is used they don't use github for binary downloads
            this works but there are lots of issues as pointed out in this issue report for example https://github.com/easybuilders/easybuild-easyconfigs/issues/5151
    
    and is discussed for `golang dep` here:

      * https://github.com/golang/dep/issues/121 
      * https://github.com/golang/dep/issues/278

    conclusion: if we had hashes in the Gopkgs.lock, we wouldn't need `go2nix` and no more duplicated git clones as well as maintainer work to update the `Gopkgs.lock` -> `deps.nix` at all.
    
    this would require:
    
      * https://lethalman.blogspot.de/2014/08/nix-pill-9-automatic-runtime.html (NAR discussion)
      * https://github.com/NixOS/nixpkgs/blob/master/pkgs/build-support/fetchgit/nix-prefetch-git
      * https://stackoverflow.com/questions/1713214/how-to-use-c-in-go
      * https://github.com/NixOS/nix/blob/master/src/libutil/hash.cc

# License

See [LICENSE](LICENSE) file.

# Todo(s) left

- integrate into nixpkgs, add nix-prefetch-url as dependency for this tool

    - https://blog.golang.org/generate
 
            joachim@lenovo-t530 ~/D/p/n/nixpkgs> git grep 'go generate'
            pkgs/applications/networking/gopher/gopherclient/default.nix:    PATH="$(pwd):$PATH" go generate ${goPackagePath}
            pkgs/applications/version-management/git-lfs/1.nix:      go generate ./commands
            pkgs/applications/version-management/git-lfs/default.nix:    go generate ./commands
            pkgs/development/interpreters/joker/default.nix:  preBuild = "go generate ./...";
            pkgs/development/tools/continuous-integration/drone/default.nix:    go generate github.com/drone/drone/server/template
            pkgs/development/tools/continuous-integration/drone/default.nix:    go generate github.com/drone/drone/store/datastore/ddl
            pkgs/development/tools/go2nix/default.nix:  preBuild = ''go generate ./...'';
            pkgs/development/tools/kube-aws/default.nix:    go generate ./core/controlplane/config
            pkgs/development/tools/kube-aws/default.nix:    go generate ./core/nodepool/config
            pkgs/development/tools/kube-aws/default.nix:    go generate ./core/root/config
            pkgs/servers/monitoring/mtail/default.nix:  preBuild = "go generate -x ./go/src/github.com/google/mtail/vm/";
 
