{
  description = "Myncer Dev environment";

  inputs = {
    nixpkgs = { url = "github:NixOS/nixpkgs/nixos-24.05"; };
    nixpkgs-unstable = { url = "github:NixOS/nixpkgs/nixpkgs-unstable"; };
    flake-utils = { url = "github:numtide/flake-utils"; };
  };

  outputs = { self, nixpkgs, nixpkgs-unstable, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        stable = import nixpkgs { inherit system; };
        unstable = import nixpkgs-unstable { inherit system; };
      in {
        devShells.default = stable.mkShell {
          buildInputs = [
            stable.go_1_23
            stable.nodejs_20
            stable.pnpm
            stable.docker
            unstable.buf
            unstable.protobuf
          ];

          shellHook = ''
            go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
            go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest

            export PATH=$PWD/myncer-web/node_modules/.bin:$HOME/go/bin:$PATH

            echo "🧪 Myncer flake shell ready"
          '';
        };
      });
}
