# Changelog

## Unreleased

### 🐛 Fixes

- The Raycast extension explains that an `impersonate` older than 0.6.0 lacks the launcher commands, instead of showing its failure to open a terminal.

### 🔧 Changes

- The Raycast extension installs its dependencies with pnpm, which `task raycast:build` and `task raycast:dev` now need.

## 0.6.0 - 2026-10-25

### ✨ Features

- `impersonate envs`, `orgs`, `users` and `open` run each step of the TUI on its own, the first three printing JSON, for a launcher such as Raycast or a script.
- A Raycast extension in `raycast/`, which Tinycast runs too, picks an organization and a user from the launcher and impersonates them.

## 0.5.0 - 2026-10-19

### ✨ Features

- Pick an environment, an organization, then one of its users, through the PropelAuth API, and impersonate them from the dashboard in a dedicated Chrome profile where the tool clicks Impersonate, so the main browser keeps its own sessions.
- Environments declared in `[[envs]]` tables of `~/.config/impersonate/config.toml`, along with the defaults of the flags; `impersonate config list|get|set|unset|edit|path` manages the file, `set` and `unset` changing only the edited line.
- The PropelAuth API key read from an AWS Secrets Manager secret, with the given or the active AWS profile, in that profile's region.
- An organization search as argument, picked directly when only one organization matches.
- Disabled and locked users hidden until `d` toggles them, or listed from the start with `--show-disabled`.
- `--per-page` sets the number of items per page.
- `--chrome-profile` opens the impersonation in one of your own Chrome profiles instead, where you click Impersonate yourself.
- Shell completion with `impersonate completion`.
