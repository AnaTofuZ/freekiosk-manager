# FreeKiosk manager

[日本語](README.ja.md)

A lightweight Go CLI and Web UI for FreeKiosk tablets on a trusted home LAN.
Manage multiple devices from a configuration file, without a database.
The CLI uses Cobra with shell completion; errors use `k1LoW/errors`.
BarefootJS compiles the TSX interface into Go `html/template` templates.

![Device overview using mock tablets, not physical devices](docs/screenshot.png)

## Build and run

With Nix flakes enabled:

```sh
nix build
./result/bin/freekioskctl --help
./result/bin/freekioskd -config /path/to/freekiosk.json -listen 0.0.0.0:8080
```

Both binaries include everything needed at runtime; JavaScript, CSS and templates
are embedded in `freekioskd`. Node.js is only needed at build time.
The default listen address is `127.0.0.1:8080`; change it to allow LAN access.

Without Nix, use Go 1.27.1 and Node.js 24.12 or later:

```sh
npm ci
npm run build
go build -o bin/freekioskctl ./cmd/freekioskctl
go build -o bin/freekioskd ./cmd/freekioskd
```

Generated assets are committed, so Go-only changes can use `go build ./...`
directly. Rebuild assets after changing TSX, TypeScript or translations.

## Configuration

Create `freekiosk.json` using `config.example.json` as a starting point.
JSON avoids an additional TOML parser and supports strict decoding with Go's standard library.

```json
{
  "devices": {
    "living": {
      "name": "Living Room",
      "url": "http://192.168.1.50:8080",
      "api_key": "your-secret-key"
    },
    "desk": {
      "name": "Desk",
      "url": "http://192.168.1.51:8080",
      "api_key": "another-key"
    }
  }
}
```

Protect this file, for example with `chmod 600 freekiosk.json`.
Device IDs must match `[a-z][a-z0-9_]{0,63}`. Device URLs contain only the
HTTP(S) scheme, host and optional port. Names default to the device ID.
You may omit `api_key` for a tablet configured without one.

| Environment variable       | Purpose                                          |
| -------------------------- | ------------------------------------------------ |
| `FREEKIOSK_CONFIG`         | Configuration path; defaults to `freekiosk.json` |
| `FREEKIOSK_LISTEN`         | Web listen address                               |
| `FREEKIOSK_API_KEY_living` | Override the key for `living`                    |
| `FREEKIOSK_URL_living`     | Override the URL for `living`                    |

Use the configured device ID verbatim as the suffix. Flags take precedence over
environment variables. Restart the server after changing configuration.

## CLI

```sh
freekioskctl devices
freekioskctl status living
freekioskctl reload living
freekioskctl clear-cache living
freekioskctl url living
freekioskctl url living https://signage.home/
freekioskctl screen living on
freekioskctl screen living off
freekioskctl brightness living 30
freekioskctl volume living 20
freekioskctl screenshot living screenshot.png
freekioskctl toast living 'hello'
freekioskctl tts living '夕食の時間です' ja
freekioskctl --config /path/to/config.json --timeout 10s status living
```

Brightness and volume are integer percentages from 0 to 100.
The TTS language is optional; specify `ja` when Japanese text might be detected
as Chinese. Screenshot downloads replace the destination only after a successful
download, using a temporary file and rename. A successful download overwrites an
existing destination. Errors go to stderr with a nonzero exit status.

### Shell completion

Cobra completes commands, flags, configured device IDs, screen states and
screenshot filenames. Completion reads configuration but never contacts tablets.

```sh
# zsh, after enabling compinit
source <(freekioskctl completion zsh)

# bash
source <(freekioskctl completion bash)

# fish
freekioskctl completion fish | source

# PowerShell
freekioskctl completion powershell | Out-String | Invoke-Expression
```

The Nix package installs bash/zsh/fish completions. The NixOS module also installs
the CLI system-wide. Enable completion in your shell. For a custom configuration
path, set `FREEKIOSK_CONFIG` or pass `--config`.

## Web interface

```sh
nix develop
go run ./cmd/freekioskd -config ./freekiosk.json -listen 0.0.0.0:8080
```

Open `http://server:8080/` and select a device to reload, clear its WebView cache,
turn its screen on/off, adjust brightness or volume, change its URL, send toast/TTS
messages, or capture a screenshot.

Status refreshes every 20 seconds while the tab is visible. Polls do not overlap
in one browser; the server also coalesces concurrent polls and requests within
three seconds for each tablet. Offline devices do not block page rendering or
other devices. The last successful status and timestamp remain visible alongside
the latest failure and attempt time. This cache is in memory and resets on restart.

Screenshots are fetched only when requested. A failed refresh preserves the last
image and capture time. The default device timeout is eight seconds; `-timeout`
accepts up to one minute.

### Languages

Use **English / 日本語** in the header. English is the default; the choice is
remembered in a cookie for one year. Labels, status, operation results and errors
are translated. Device names, URLs and other device-provided values are unchanged.
The CLI and JSON API remain language-neutral/English.

Translations live in `web/locales/en.json` and `web/locales/ja.json`, shared by
Go and TypeScript without an i18n dependency. Keep both dictionaries' keys aligned,
reference `props.text` in TSX, and run `npm run build` after edits.

## Enable the FreeKiosk REST API

Follow the [official REST API documentation](https://github.com/RushB-fr/freekiosk/blob/main/docs/rest-api.md):

1. Tap the secret button five times and enter your settings PIN.
2. Under Advanced, enable the REST API.
3. Set the port (default 8080) and API key, then save.
4. Enable remote control too; otherwise command endpoints return 403.
5. Add the tablet's LAN address to the manager configuration. A DHCP reservation is recommended.

Screen OFF depends on Android permissions. Without Device Owner, Device Admin or
AccessibilityService privileges, FreeKiosk falls back to zero brightness.
`screen.on` describes the physical screen; `screensaverActive` is separate.
Brightness commands are ignored when App Brightness Control is disabled.

For screenshots outside the WebView, FreeKiosk v1.2.20+ requires Android 11+ and
AccessibilityService. In Lock Mode, also check Security → Allow Remote Screenshots.
Unavailable capture returns 503. TTS requires the appropriate Android voice data.

## Supported API

Verified on 2026-09-08 against the official
[REST API specification](https://github.com/RushB-fr/freekiosk/blob/main/docs/rest-api.md),
[KioskHttpServer.kt](https://github.com/RushB-fr/freekiosk/blob/main/android/app/src/main/java/com/freekiosk/api/KioskHttpServer.kt)
and [HttpServerModule.kt](https://github.com/RushB-fr/freekiosk/blob/main/android/app/src/main/java/com/freekiosk/api/HttpServerModule.kt).
Authentication uses the `X-Api-Key` header.

| Operation   | FreeKiosk request               | Body / response                                      |
| ----------- | ------------------------------- | ---------------------------------------------------- |
| Status      | `GET /api/status`               | JSON envelope: `success`, `data`, `timestamp`        |
| Current URL | `GET /api/status`               | `data.webview.currentUrl`; no `GET /api/url`         |
| Reload      | `POST /api/reload`              | No body                                              |
| Clear cache | `POST /api/clearCache`          | No body                                              |
| Screen      | `POST /api/screen/on` or `/off` | No body                                              |
| Brightness  | `POST /api/brightness`          | `{"value":30}`                                       |
| Volume      | `POST /api/volume`              | `{"value":20}`                                       |
| Change URL  | `POST /api/url`                 | `{"url":"https://signage.home/"}`                    |
| Screenshot  | `GET /api/screenshot`           | PNG bytes                                            |
| Toast       | `POST /api/toast`               | `{"text":"hello"}`                                   |
| TTS         | `POST /api/tts`                 | `{"text":"夕食","language":"ja"}`; optional language |

Commands check `success` and `data.executed`, not just HTTP status. Success means
the tablet accepted/executed the command, not that the WebView finished loading.
Missing status fields are omitted from the UI. Unknown fields are ignored, while
incorrect types in known fields are invalid responses.

The BFF exposes `GET /api/devices`, `GET /api/devices/{id}/status`,
`GET /api/devices/{id}/screenshot` and `POST /api/devices/{id}/{action}`.
POST requires JSON. Screen takes `{"state":"on"}`; other bodies match the table.
The status response contains `status` (last successful data), `online`,
`last_success`, `last_attempt` and optional `error`. This snapshot returns 200
even when the tablet is offline. Command/image upstream errors return 502,
invalid input 400, and unknown devices 404.

Error kinds distinguish `timeout`, `connection_refused`, `network`,
`authentication` (401), `forbidden` (403), `http`, `invalid_response`, `api`,
`unknown_device`, `validation` and `canceled`.
Both CLI and Web use the same internal client.

## Security

For trusted home LANs only. There is no login: anyone who can reach the manager
can control your tablets. Do not expose it to the internet. Restrict listening
addresses and firewall rules to trusted LANs/VLANs. FreeKiosk HTTP traffic is not encrypted.

API keys stay in the backend and never appear in HTML, JavaScript or device lists.
Raw upstream errors are not forwarded because they may contain credentials.
Avoid putting other services' secrets in displayed URLs or status data.

Only configured devices are reachable. Arbitrary proxy paths, redirects and
environment HTTP proxies are disabled. URL changes require HTTP(S); inputs and
responses have size/range limits. Cross-origin commands are rejected and CSP
blocks external scripts and framing. Arbitrary JavaScript, app launching and
reboot are intentionally not exposed.

## NixOS module

Add this repository as a flake input and import its module:

```nix
{
  imports = [ inputs.freekiosk-manager.nixosModules.default ];

  services.freekiosk-manager = {
    enable = true;
    listenAddress = "0.0.0.0";
    port = 8080;
    configFile = "/run/secrets/freekiosk.json";
    # Only enable on a trusted LAN.
    openFirewall = true;
  };
}
```

Keep secrets out of Nix expressions/the Nix store. Provision a runtime file with
sops-nix, agenix or similar. `configFile` is an absolute path string.
systemd `LoadCredential` makes a root-readable source available to the service's
DynamicUser. The service uses a read-only filesystem and restricted privileges.

## Development

`nix develop` provides Go 1.27.1, golangci-lint 2.13.2, TypeScript, Node.js,
oxfmt 0.67.0, oxlint 1.82.0 and Vite. Go, golangci-lint and Oxc versions were the
latest stable releases on 2026-09-08. Lockfiles pin reproducible dependencies;
check upstream releases and Go compatibility when updating.

```sh
nix develop
npm ci

# TSX → Go templates/types + browser JavaScript, including type checking
npm run build

# Format
golangci-lint fmt
oxfmt .

# Lint
golangci-lint run
oxlint . --deny-warnings
npm run lint

# Test and build
go test ./...
go build ./...
nix build

# All of the above
bash scripts/check.sh
```

Go formatting uses gofmt/goimports through golangci-lint. The generated
`components.go` is also gofmt-formatted during generation for reproducibility.
Lint adds noctx/bodyclose/nilerr to the standard linters. Error classification
uses `k1LoW/errors.As/Is`; new client errors use `WithStack`.
Stack traces are never serialized to the Web API.

### BarefootJS and TypeScript

`web/src/components/*.tsx` is the UI source of truth. Page defines the overview
and detail layout; Status defines the status fragment; Controls defines forms,
loading/results and screenshots. The official `@barefootjs/go-template/vite`
adapter generates Go templates, Go types and client JavaScript. The only
handwritten Go template is the document shell in `web/templates/layout.gohtml`.

BarefootJS lets us author TSX while Go renders the HTML, with signals/events only
where interaction is needed. There is no SPA router or large frontend store.
`main.ts` handles polling and replaces server-rendered, escaped Status fragments.

See [barefootjs.dev](https://barefootjs.dev/), the
[official repository](https://github.com/piconic-ai/barefootjs) and
[Go Template Adapter](https://github.com/piconic-ai/barefootjs/blob/main/docs/core/adapters/go-template-adapter.md).
The originally supplied barefootjs.com did not resolve when checked.

BarefootJS 0.35.1 is alpha, so its packages and Go runtime commit are pinned.
TypeScript 5.9.3 and Vite 6.4.3 match its compiler peer dependencies.
The Go runtime lives in the upstream monorepo under a different module declaration,
so `go.mod` uses a `replace` directive. React is not a dependency:
`react-jsx` in tsconfig only selects the TSX type-checking mode.

Oxfmt formats authored TS/TSX, CSS, JSON and Markdown. Oxlint checks TS/TSX;
`tsc --noEmit` checks types. Oxlint is not an HTML/CSS linter.
Handwritten Go templates and generated outputs are excluded from oxfmt to avoid
damaging Go template syntax. The corresponding UI markup is formatted in TSX.

Generated outputs are `web/generated/` (templates), `web/views/components.go`
(types) and `web/static/generated/` (JavaScript). Edit the source and rebuild,
not these generated files.

### Testing without a tablet

`go test ./...` uses httptest for authentication headers, status, commands,
timeouts, refused connections, HTTP/authentication errors, malformed JSON,
validation, redirect rejection, PNG handling, stale status, CSRF, CLI completion
and English/Japanese rendering.

To preview the UI with mock online/offline devices:

```sh
FREEKIOSK_PREVIEW=127.0.0.1:8099 go test ./internal/web -run TestPreview -v -timeout 0
```

Open `http://127.0.0.1:8099/`; stop with Ctrl+C. No real tablet is contacted.
The README screenshot uses this mock. Android permissions, TTS voices and actual
screen-off behavior still require testing on your physical device.

## References

- [Go stable downloads](https://go.dev/dl/?mode=json)
- [golangci-lint releases](https://github.com/golangci/golangci-lint/releases) / [configuration](https://golangci-lint.run/docs/configuration/file/)
- [Oxfmt](https://oxc.rs/docs/guide/usage/formatter) / [Oxlint](https://oxc.rs/docs/guide/usage/linter)
- [Cobra shell completion](https://cobra.dev/docs/how-to-guides/shell-completion/)
- [k1LoW/errors](https://github.com/k1LoW/errors)
