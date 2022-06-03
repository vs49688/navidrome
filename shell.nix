{ pkgs ? (import <nixpkgs> {})}:
pkgs.mkShell {
  pname   = "navidrome";
  version = "dev";

  hardeningDisable = [ "all" ];

  nativeBuildInputs = with pkgs; [
    go_1_17
    (golangci-lint.override { buildGoModule = pkgs.buildGo117Module; })
    pkg-config
    nodejs-16_x
    sqlite-interactive
  ];

  buildInputs = with pkgs; [
    taglib
    zlib
  ];

  shellHook = ''
    export GOROOT=${pkgs.go_1_17}/share/go
    export PATH=${pkgs.go_1_17}/bin:$PATH
  '';
}
