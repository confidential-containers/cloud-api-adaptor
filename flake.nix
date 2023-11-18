{
  description = "Cloud API Adaptor for Confidential Containers";

  inputs = {
    nixpkgsUnstable = {
      url = "github:nixos/nixpkgs/nixos-unstable";
    };
    flake-utils = {
      url = "github:numtide/flake-utils";
    };
  };

  outputs =
    { self
    , nixpkgsUnstable
    , flake-utils
    }:
    flake-utils.lib.eachDefaultSystem
      (system:
      let
        pkgsUnstable = import nixpkgsUnstable { inherit system; };
      in
      {
        devShells = {
          # Shell for building podvm images with mkosi.
          podvm-mkosi = pkgsUnstable.mkShell {
            nativeBuildInputs = with pkgsUnstable; [
              btrfs-progs
              cryptsetup
              dnf5
              dosfstools
              mkosi-full
              mtools
              rpm
              squashfsTools
              util-linux
              zstd
              e2fsprogs # remove when switching to squashFS
            ];
          };
        };

        formatter = nixpkgsUnstable.legacyPackages.${system}.nixpkgs-fmt;
      });
}
