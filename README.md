# propelauth-impersonate

`impersonate` is a terminal UI to impersonate a [PropelAuth](https://www.propelauth.com) user: pick an environment, an organization, then a user, and the tool opens the PropelAuth dashboard on that user and clicks Impersonate.

![impersonate: picking staging, then Acme Corp, then one of its users](docs/demo.gif)

It lists the organizations and users through the PropelAuth backend API, with an API key read from AWS Secrets Manager. The impersonation itself has to go through the dashboard, which only lets a signed-in employee start one, so the tool drives a dedicated Chrome profile: the main browser keeps its own sessions, and the dashboard sign-in happens there once.

## Requirements

- macOS and Google Chrome.
- An AWS profile allowed to read the secrets that hold the PropelAuth API keys.
- A PropelAuth dashboard account allowed to impersonate users.

## Install

```sh
mise use -g github:vmaerten/propelauth-impersonate
```

It installs the `impersonate` command. mise hides releases younger than its `minimum_release_age`; to get a new one right away, exclude the tool in `~/.config/mise/config.toml`:

```toml
[settings]
minimum_release_age_excludes = ["github:vmaerten/propelauth-impersonate"]
```

From a clone, `task build` writes `bin/impersonate` and `go run .` runs the tool. Prefer them to `go install .`, whose binary in Go's `bin` directory can shadow the mise install in `PATH`.

## Quick start

1. Declare the environments in the config file, which `impersonate config edit` opens:

   ```toml
   [[envs]]
   name = "staging"
   auth_hostname = "auth.staging.example.com"
   secret = "myapp-staging"
   ```

2. Run `impersonate`, then pick the environment, the organization and the user.
3. The first time, sign in to the PropelAuth dashboard in the Chrome window that opens: the tool waits up to three minutes for it, then clicks Impersonate. The dashboard session lasts 14 days, and each impersonation extends it.

## Usage

```sh
impersonate                 # picks the environment, then the organization and the user
impersonate -e prod         # skips the environment pick
impersonate acme            # pre-fills the organization search, picks it when only one matches
impersonate -- config       # a search named like a subcommand goes after --
```

### Flags

| Flag | Default | Setting | Description |
| --- | --- | --- | --- |
| `-e`, `--env` | | `env` | Environment to start on, picked in the TUI when empty. |
| `-p`, `--profile` | the active AWS profile | `profile` | AWS profile that reads the PropelAuth API key. |
| `--per-page` | `10` | `per_page` | Items per page, `0` to fill the terminal. |
| `--show-disabled` | `false` | `show_disabled` | Lists the disabled and locked users from the start. |
| `--chrome-profile` | the dedicated profile | `chrome_profile` | One of your Chrome profiles to open the impersonation in, without the automatic click, see [Chrome profiles](#chrome-profiles). |

Each flag starts from its setting in the config file and overrides it: `--show-disabled=false` hides the users that `show_disabled = true` would list.

### Keys

| Screen | Key | Action |
| --- | --- | --- |
| Environment picker | ↑ ↓, k j | Move |
| | enter | Pick |
| | q, esc | Quit |
| Organizations and users | / | Filter |
| | enter | Pick |
| | esc | Clear the filter, or go back |
| | ? | List the other keys |
| | q | Quit |
| Users | d | Show or hide the disabled and locked users |

## Configuration

The file is `~/.config/impersonate/config.toml`, under `$XDG_CONFIG_HOME` when it is set; `impersonate config path` prints it. Only the environments are required.

### Settings

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `env` | string | | Environment to start on, among the declared ones. |
| `profile` | string | the active AWS profile | AWS profile that reads the PropelAuth API keys. |
| `per_page` | integer | `10` | Items per page, `0` to fill the terminal. |
| `show_disabled` | boolean | `false` | Lists the disabled and locked users from the start. |
| `chrome_profile` | string | the dedicated profile | One of your Chrome profiles to open the impersonation in, without the automatic click. |

### Environments

One `[[envs]]` table per PropelAuth environment, in the picker's order:

| Field | Required | Description |
| --- | --- | --- |
| `name` | yes | Name in the picker and for `--env`, unique. |
| `auth_hostname` | yes | Auth URL of the environment, without `https://`. |
| `secret` | yes | AWS Secrets Manager secret that holds its API key, see [AWS](#aws). |
| `color` | no | Background of its titles, `#RRGGBB` or an ANSI 256 color number, `62` by default. |

### Example

```toml
env = "staging"   # skips the environment pick, -e "" brings it back
per_page = 0      # fills the terminal

[[envs]]
name = "staging"
auth_hostname = "auth.staging.example.com"
secret = "myapp-staging"
color = "#B9770E"

[[envs]]
name = "prod"
auth_hostname = "auth.example.com"
secret = "myapp-prod"
color = "#C0392B"
```

### `config` subcommands

| Command | Description |
| --- | --- |
| `impersonate config list` | Prints the effective settings, defaults and environments included. |
| `impersonate config get <key>` | Prints one of them, `envs.<name>.<field>` for an environment. |
| `impersonate config set <key> <value>` | Writes a setting, changing only its line, so comments and formatting stay. |
| `impersonate config unset <key>` | Removes the line of a setting. |
| `impersonate config edit` | Opens the file in `$VISUAL`, `$EDITOR` or `vi`, then checks it. |
| `impersonate config path` | Prints the path of the file. |

`set` and `unset` only write the settings: the environments change through `edit`. An invalid file stops the TUI and `list`, `get`, `set` and `unset`, while `edit` still opens it to fix it.

## AWS

The `secret` of an environment is a JSON secret of AWS Secrets Manager whose `PROPELAUTH_API_KEY` field holds the backend API key of that PropelAuth environment:

```json
{ "PROPELAUTH_API_KEY": "…" }
```

It is read with the profile of `--profile` or `profile`, else the active one (`AWS_PROFILE`, else `default`), in that profile's region from `~/.aws/config` (or `AWS_REGION`). When the SSO session has expired, run `aws sso login --profile <profile>`.

## Chrome profiles

By default, the impersonation runs in a dedicated Chrome profile:

- It lives in `~/Library/Application Support/impersonate/chrome`, and listens on `127.0.0.1:9333` for the tool. It opens on the first impersonation and stays open.
- For the picked user, the tool opens `app.propelauth.com/impersonate-user`, clicks Impersonate, waits for the redirect to the environment's auth domain, then leaves the tab open. The impersonation lasts PropelAuth's default hour.

`--chrome-profile` (or `chrome_profile`) opens the same page in one of your own Chrome profiles instead, by its name as Chrome shows it (`Work`) or its directory (`Profile 2`). Chrome lets no tool drive a regular profile, so you click Impersonate there yourself. The impersonated session then replaces that profile's own session on the app's auth domain, and stopping the impersonation signs the profile out: the reason why the dedicated profile is the default.

## Launcher commands

Each step of the TUI is also a command of its own, for a launcher such as the [Raycast extension](#raycast-and-tinycast), or a script. The first three print JSON on stdout:

| Command | Prints |
| --- | --- |
| `impersonate envs` | The environments of the config file, with their `name`, `auth_hostname`, `secret` and `color`. |
| `impersonate orgs -e <env>` | The organizations of the environment by name, with their `org_id` and `name`. |
| `impersonate users -e <env> <org-id>` | The users of the organization by email, disabled and locked ones included, with their `user_id`, `email`, `first_name`, `last_name`, `role` in it, `enabled` and `locked`. |
| `impersonate open -e <env> <user-id>` | Nothing: it impersonates the user, as picking them in the TUI does. |

`-e` defaults to the `env` setting, `orgs` and `users` take `-p` and `open` takes `--chrome-profile`, as the TUI does. `open --notify` also reports a failure in a macOS notification, for a launcher that does not wait for the command to end. A failure prints its error on stderr and exits with 1.

## Raycast and Tinycast

`raycast/` holds a Raycast extension, which Tinycast runs too. Its Impersonate User command lists the organizations of an environment, picked in the search bar and remembered, then the users of the one picked, and impersonates a user through the [launcher commands](#launcher-commands):

| Key | Action |
| --- | --- |
| enter | Show the users of the organization, or impersonate the user |
| ⌘D | Show or hide the disabled and locked users |
| ⌘K | Copy the organization ID, the user's email or ID |

To install it, with Node.js and pnpm:

- Tinycast: run `task raycast:build`, then install the `raycast/dist` folder with Install Extension, in the Extensions settings.
- Raycast: run `task raycast:dev` once, which adds it to the development extensions of Raycast.

The launcher does not see the shell's `PATH` and environment:

- The extension runs `~/.local/share/mise/shims/impersonate`, which mise installs: change it in the extension's preferences for another install.
- When `AWS_PROFILE` picks the AWS profile, set it in the config with `impersonate config set profile <profile>`.

## Shell completion

`impersonate completion zsh` (or `bash`, `fish`, `powershell`) prints a completion script, which also completes the setting keys and the declared environments. For zsh:

```sh
impersonate completion zsh > "${fpath[1]}/_impersonate"
```

## Development

| Task | Description |
| --- | --- |
| `task build` | Builds `bin/impersonate`. |
| `task run -- <args>` | Runs the tool with the arguments after `--`. |
| `task lint` | Runs `go vet` and checks the formatting. |
| `task install` | Runs `go install .`, see the shadowing note in [Install](#install). |
| `task raycast:build` | Builds the Raycast extension into `raycast/dist`. |
| `task raycast:dev` | Runs the Raycast extension in development mode. |
| `task release:notes VERSION=x.y.z` | Prints the changelog section of a version. |
| `task release:snapshot` | Builds the release archives into `dist/`, without publishing. |

`go test ./...` runs the tests.

### Releasing

1. In a pull request, move the `Unreleased` entries of `CHANGELOG.md` under a `## x.y.z - YYYY-MM-DD` section. Its subsections are, in this order and when not empty, 💥 Breaking changes, ✨ Features, 🐛 Fixes and 🔧 Changes.
2. Once it is merged, tag the merge commit `vx.y.z` and push the tag.
3. The release workflow builds the macOS archives with GoReleaser and publishes them, with that changelog section as the release body. It fails when the section is missing.
