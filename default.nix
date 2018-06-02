with import <nixpkgs>{};

buildGoPackage rec {
  name = "dep2nix";

  goPackagePath = "github.com/nixcloud/dep2nix";

  src = ./.;

  goDeps = ./deps.nix;
  
  meta = with stdenv.lib; {
    description = "Convert `Gopkg.lock` files from golang dep into `deps.nix`";
    license = licenses.bsd3;
    homepage = https://github.com/nixcloud.io/dep2nix;
  };
  
}
