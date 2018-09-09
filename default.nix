with import <nixpkgs>{};

buildGoPackage rec {
  name = "dep2nix";

  goPackagePath = "github.com/nixcloud/dep2nix";

  src = ./.;
  
  buildInputs = [ makeWrapper ];
  binPath = lib.makeBinPath [ nix-prefetch-git nix-prefetch-hg ];

  goDeps = ./deps.nix;

  postInstall = ''
    wrapProgram $bin/bin/dep2nix --prefix PATH ':' ${binPath}
  '';
  
  meta = with stdenv.lib; {
    description = "Convert `Gopkg.lock` files from golang dep into `deps.nix`";
    license = licenses.bsd3;
    homepage = https://github.com/nixcloud.io/dep2nix;
  };
}
