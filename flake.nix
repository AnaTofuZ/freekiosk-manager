{
  description = "Lightweight FreeKiosk LAN manager and CLI";
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  inputs.go-overlay.url = "github:purpleclay/go-overlay";
  inputs.go-overlay.inputs.nixpkgs.follows = "nixpkgs";

  outputs = { self, nixpkgs, go-overlay }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" "x86_64-darwin" ];
      eachSystem = nixpkgs.lib.genAttrs systems;
    in {
      packages = eachSystem (system:
        let
          pkgs = import nixpkgs { inherit system; overlays = [ go-overlay.overlays.default ]; };
          go = pkgs.go-bin.fromGoMod ./go.mod;
          src = pkgs.lib.cleanSourceWith {
            src = ./.;
            filter = path: type:
              let
                name = baseNameOf path;
                relative = pkgs.lib.removePrefix (toString ./. + "/") (toString path);
              in !(builtins.elem name [ ".git" "node_modules" "result" "bin" ".cache" "freekiosk.json" ])
                && !(builtins.elem relative [ "web/generated" "web/static/generated" "web/views/components.go" ])
                && pkgs.lib.cleanSourceFilter path type;
          };
          frontend = pkgs.buildNpmPackage {
            pname = "freekiosk-manager-frontend";
            version = "0.1.0";
            inherit src;
            npmDepsHash = "sha256-SSdqvySyOfy70ehVh6pTya80UsdKTl5/qmjoLn35ccw=";
            nativeBuildInputs = [ pkgs.makeWrapper ]
              ++ pkgs.lib.optionals pkgs.stdenv.hostPlatform.isLinux [ pkgs.autoPatchelfHook ];
            buildInputs = pkgs.lib.optionals pkgs.stdenv.hostPlatform.isLinux [ pkgs.stdenv.cc.cc.lib ];
            preBuild = pkgs.lib.optionalString pkgs.stdenv.hostPlatform.isLinux ''
              autoPatchelf node_modules
            '';
            installPhase = ''
              runHook preInstall
              mkdir -p "$out/share" "$out/lib" "$out/bin"
              cp -R web/static/generated "$out/share/static"
              cp -R web/generated "$out/share/templates"
              cp -R web/views "$out/share/views"
              cp -R node_modules "$out/lib/"
              for tool in tsc oxfmt oxlint vite; do
                makeWrapper "$out/lib/node_modules/.bin/$tool" "$out/bin/$tool" \
                  --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.nodejs ]}
              done
              runHook postInstall
            '';
          };
        in {
          inherit frontend;
          default = (pkgs.buildGoModule.override { inherit go; }) {
            pname = "freekiosk-manager";
            version = "0.1.0";
            inherit src;
            vendorHash = "sha256-Y1AJQKSltFSJolFYHcgn9EYU05n7T+0+T9nYHpFeVZY=";
            subPackages = [ "cmd/freekioskctl" "cmd/freekioskd" ];
            nativeBuildInputs = [ pkgs.installShellFiles ];
            preBuild = ''
              cp -R ${frontend}/share/static/. web/static/generated/
              cp -R ${frontend}/share/templates/. web/generated/
              cp -R ${frontend}/share/views/. web/views/
            '';
            ldflags = [ "-s" "-w" ];
            checkPhase = ''
              runHook preCheck
              go test ./...
              runHook postCheck
            '';
            postInstall = ''
              installShellCompletion --cmd freekioskctl \
                --bash <("$out/bin/freekioskctl" completion bash) \
                --zsh <("$out/bin/freekioskctl" completion zsh) \
                --fish <("$out/bin/freekioskctl" completion fish)
            '';
            meta = { description = "FreeKiosk LAN management CLI and Web UI"; mainProgram = "freekioskctl"; platforms = systems; };
          };
        });
      devShells = eachSystem (system:
        let
          pkgs = import nixpkgs { inherit system; overlays = [ go-overlay.overlays.default ]; };
          go = pkgs.go-bin.fromGoMod ./go.mod;
        in { default = pkgs.mkShell {
          packages = [ go go.tools.golangci-lint.latest pkgs.djlint pkgs.nodejs self.packages.${system}.frontend ];
        }; });
      nixosModules.default = { config, lib, pkgs, ... }:
        let cfg = config.services.freekiosk-manager;
        in {
          options.services.freekiosk-manager = {
            enable = lib.mkEnableOption "FreeKiosk LAN manager";
            package = lib.mkOption { type = lib.types.package; default = self.packages.${pkgs.stdenv.hostPlatform.system}.default; };
            listenAddress = lib.mkOption { type = lib.types.str; default = "127.0.0.1"; };
            port = lib.mkOption { type = lib.types.port; default = 8080; };
            configFile = lib.mkOption { type = lib.types.str; description = "Absolute path to a runtime JSON secret (kept outside the Nix store)"; };
            openFirewall = lib.mkOption { type = lib.types.bool; default = false; };
          };
          config = lib.mkIf cfg.enable {
            environment.systemPackages = [ cfg.package ];
            networking.firewall.allowedTCPPorts = lib.mkIf cfg.openFirewall [ cfg.port ];
            systemd.services.freekiosk-manager = {
              description = "FreeKiosk LAN manager";
              wantedBy = [ "multi-user.target" ];
              after = [ "network.target" ];
              serviceConfig = {
                ExecStart = "${cfg.package}/bin/freekioskd -config %d/config.json -listen ${lib.escapeShellArg "${if lib.hasInfix ":" cfg.listenAddress then "[${cfg.listenAddress}]" else cfg.listenAddress}:${toString cfg.port}"}";
                LoadCredential = "config.json:${cfg.configFile}";
                DynamicUser = true;
                Restart = "on-failure";
                RestartSec = 5;
                NoNewPrivileges = true;
                ProtectSystem = "strict";
                ProtectHome = true;
                PrivateTmp = true;
                PrivateDevices = true;
                RestrictAddressFamilies = [ "AF_INET" "AF_INET6" "AF_UNIX" ];
              };
            };
          };
        };
    };
}
