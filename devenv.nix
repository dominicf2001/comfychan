{ pkgs, lib, config, inputs, ... }:

{
  packages = with pkgs; [
    sqlite
    templ
    air
  ];

  languages.go.enable = true;
}

