**English** · [Русский](README.ru.md) · [Oʻzbekcha](README.uz.md)

# bdv

**bdv is a fast, minimal terminal UI (TUI) viewer for the [beads](https://github.com/steveyegge/beads) (`bd`) issue tracker: watch the task tree that you and your AI coding agents (Claude Code, Codex, Cursor and others) keep in beads, live, from a second terminal.**

[![CI](https://github.com/nizom333/bdv/actions/workflows/ci.yml/badge.svg)](https://github.com/nizom333/bdv/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/nizom333/bdv)](https://github.com/nizom333/bdv/releases/latest)
[![Go version](https://img.shields.io/github/go-mod/go-version/nizom333/bdv)](go.mod)
[![License: MIT](https://img.shields.io/github/license/nizom333/bdv)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/nizom333/bdv)](https://goreportcard.com/report/github.com/nizom333/bdv)

```text
demo-app  ◐ 2 in progress · ○ 8 open · ● 0 blocked · ⊘ 2 waiting
 Tasks │ Specs 2 │ Decisions 0 │ Memory │ Formulas
╭─ ◆ Tasks (10) ────────────────────────╮╭─ demo-app-k3f ──────────────────────────────────────────────╮
│ ▾ P0 · 1                              ││ Checkout flow                                               │
│     ○ x2b Prices rounded wrong for JPY││  IN PROGRESS   P1  [epic]                                   │
│ ▾ P1 · 7                              ││ #shop #payments                                             │
│   ▾ ◐ k3f Checkout flow               ││ created 2026-11-18 · updated 2026-11-20                     │
│       ◐ k3f.2 Payment provider integr…││                                                             │
│       ○ P2 k3f.3 ⊘ Order confirmation…││ Description ─────────────────────────────────────────────── │
│       ○ P2 k3f.4 ⊘ Retry failed payme…││ Customers can pay for the cart in one step.                 │
│   ▾ ○ p7a Product search              ││                                                             │
│       ○ p7a.1 Search index for produc…││ ## Scope                                                    │
│       ○ P2 p7a.2 Typo-tolerant queries││                                                             │
│ ▾ P3 · 1                              ││ • Card payments through the payment provider                │
│     ○ m4e Upgrade Go toolchain        ││ • Order confirmation email                                  │
│ ▾ P4 · 1                              ││ • Retry on network errors                                   │
│     ○ q8r Dark mode for admin panel   ││                                                             │
│                                       ││ ## Out of scope                                             │
│                                       ││                                                             │
│                                       ││ Gift cards and split payments.                              │
│                                       ││                                                             │
│                                       ││ Children ────────────────────────────────────────────────── │
╰───────────────────────────────────────╯╰─────────────────────────────────────────────────────────────╯
updated 02:11:12                                                [ ] tabs · / search · m comment · ? help
```

The top line shows the project name and a status summary, the line under it the tabs. Below them, the task tree built from parent-child links is on the left and the full card of the selected task is on the right; a status line is at the bottom. Data refreshes on its own every 10 seconds, so you can keep bdv open while an agent creates, updates and closes tasks.

bdv is careful with your data. The only writes are: adding a comment (`m`), sent as `bd comments add <id> -f <temp file>`; `bd init` in a project without a database; `bd config set issue_id_mode counter` right after that `bd init` if you chose sequential IDs; and `bd config set` / `bd config unset` from the settings screen (`,`), each confirmed with `y`. Every other `bd` call runs with `--readonly`.

## Features

- **Task tree** from parent-child links: open tasks on top, closed tasks and events hidden until you ask (`c`, `e`), IDs shown without the shared project prefix.
- **Priority groups**: root tasks under `P0`…`P4` headers (`p` toggles); a group card lists its tasks.
- **Tabs**: Tasks, Specs (epics with `███░░░░░ 3/8` progress), Decisions, Memory (`bd memories`) and Formulas (`bd formula list` and templates).
- **Task card**: markdown description, notes, spec fields (Spec, Design, Acceptance), parent, children, blockers, related tasks and comments; `1`–`9` follow links.
- **Search** that filters as you type, Cyrillic included.
- **Blocked tasks** marked with a red `⊘` and counted in the top line; `R` shows only tasks that are ready to work on.
- **Comments**: write one with `m`, send with `Ctrl+S`.
- **bd settings** screen (`,`): read and change the project's `bd config` with a confirmation for every change.
- **`bd init`** offer in a directory without a database, with hash or sequential IDs.
- **Copy** ID, title, description or the whole card as markdown (`y`, `Y`, `D`, `M`).
- **Languages**: English and Russian interface (`BDV_LANG`).
- Mouse wheel, narrow mode for small terminals, light and dark terminal themes.

## Install

bdv reads everything through the `bd` CLI, so install [beads](https://github.com/steveyegge/beads) first and make sure `bd` is in your `PATH`.

### Homebrew (macOS, Linux)

```sh
brew install nizom333/tap/bdv
```

The tap ships bdv as a Homebrew cask with pre-built binaries for macOS and Linux (amd64, arm64). Update with `brew upgrade bdv`.

### Scoop (Windows)

```powershell
scoop bucket add nizom333 https://github.com/nizom333/homebrew-tap
scoop install nizom333/bdv
```

### Debian, Ubuntu, Fedora, RHEL, Alpine

Download the package for your system from the [latest release](https://github.com/nizom333/bdv/releases/latest). Asset names (`VERSION` without the leading `v`, for example `0.1.0`):

| Format | amd64 | arm64 |
|---|---|---|
| deb | `bdv_VERSION_amd64.deb` | `bdv_VERSION_arm64.deb` |
| rpm | `bdv-VERSION-1.x86_64.rpm` | `bdv-VERSION-1.aarch64.rpm` |
| apk | `bdv_VERSION_x86_64.apk` | `bdv_VERSION_aarch64.apk` |

```sh
VERSION=0.1.0
curl -LO "https://github.com/nizom333/bdv/releases/download/v${VERSION}/bdv_${VERSION}_amd64.deb"
sudo apt install "./bdv_${VERSION}_amd64.deb"                 # Debian, Ubuntu
# sudo dnf install "./bdv-${VERSION}-1.x86_64.rpm"            # Fedora, RHEL
# sudo apk add --allow-untrusted "./bdv_${VERSION}_x86_64.apk" # Alpine
```

### Arch Linux (AUR)

Coming soon: `bdv-bin` (`yay -S bdv-bin`). Until then, use a release binary or `go install`.

### Go

```sh
go install github.com/nizom333/bdv@latest
```

This puts `bdv` into `$(go env GOPATH)/bin`; `bdv --version` reports the module version.

### Binaries

Every [release](https://github.com/nizom333/bdv/releases) has archives for macOS, Linux and Windows on amd64 and arm64: `bdv_VERSION_OS_ARCH.tar.gz` (`.zip` on Windows), for example `bdv_0.1.0_darwin_arm64.tar.gz`, plus `checksums.txt`:

```sh
sha256sum --ignore-missing -c checksums.txt   # macOS: shasum -a 256 --ignore-missing -c checksums.txt
tar xzf bdv_0.1.0_linux_amd64.tar.gz && sudo mv bdv /usr/local/bin/
```

### From source

You need Go (see `go.mod` for the version).

```sh
git clone https://github.com/nizom333/bdv && cd bdv
make install                     # into ~/.local/bin
make install PREFIX=/usr/local   # somewhere else
```

If `~/.local/bin` is not in your `PATH`, add `export PATH="$HOME/.local/bin:$PATH"` to `~/.zshrc`.

## Usage

```sh
cd <project> && bdv         # .beads is looked up in the current directory and above (up to $HOME)
bdv <id>                    # open on a specific task
bdv --db <path>             # explicit database (same as bd)
bdv --help | --version
```

A typical setup: an AI coding agent such as Claude Code works through the beads tasks in one terminal, and bdv shows the tree in another, so you see what is in progress, what is blocked and what is done without asking.

### Projects without a database

If there is no beads database yet (no `.beads` in the current directory or above, or `bd` reports `no beads database found`), bdv offers to create one instead of exiting: press `i`, check the directory and prefix in the confirmation, then `y` to run plain `bd init` in the current directory with bd's defaults (the prefix is the directory name). The confirmation also shows the ID mode, `IDs: hash (bd default)`; `c` switches it to `IDs: sequential 1, 2, 3` and back. With sequential IDs, a successful `bd init` is followed by `bd config set issue_id_mode counter`; if that command fails, its error is shown and the tree still opens, since the database already exists. Any other key cancels. On success the tree opens, empty. `~/.beads` is bd's global data directory, not a project, so it never counts as a database here. The offer only appears when `--db`, `BEADS_DIR` and `BEADS_DB` are all unset, since otherwise it is unclear where the database should go.

### Settings

`,` opens the project's bd settings, grouped into sections: IDs (prefix, ID mode, hash lengths), statuses and types (custom lists), creating issues (required description, validation, nesting depth) and storage (Dolt auto-commit and auto-push, JSONL export, backups, compaction). Values are read with `bd --readonly config list --json`, `bd --readonly config get <key> --json` and `bd --readonly types --json`; a value you have not set is shown dimmed with `(default)`. `j k` move, the line under the selected setting describes it and names its `bd` key, `Esc` goes back to the tree.

`Enter` (or `Space`) changes the selected value: booleans toggle, choices cycle, numbers open an input that checks the range, and the custom status and type lists open an editor where `a` adds a name (`a-z`, `0-9`, `_`), `x` (or `d`) removes the selected one and `Enter` saves the list; built-in statuses and types are shown but cannot be removed. `u` resets a value you have set (`bd config unset`). Every change first shows `<key>: <old> → <new>`, with a warning for risky keys (ID mode, Dolt auto-commit and auto-push, validation `error`, and removing a status or type that tasks still use), and runs only on `y`. The ID prefix is read-only: change it with `bd rename-prefix`. Secrets, `schema_version`, `db`, `actor`, `identity`, `sync.remote` and `federation.*` never appear. A `config.yaml` setting is shown only when `bd config get` reports it as one (`location: config.yaml`); otherwise this bd does not treat it as a file setting and writing it would only leave an unused key in the database, so it is hidden (in bd 0.59 that is `export.auto`). Custom types are read either way. If some values cannot be read, a line says how many, and `r` reads them again. The values shown are the ones bd reports, so `BD_*` environment variables or the global config (`~/.config/bd/config.yaml`) may override what the project file says.

### Tabs

`[` and `]` switch tabs in a cycle: `Tasks │ Specs N │ Decisions N │ Memory N │ Formulas N`. The active tab is highlighted, empty ones are dimmed. Each tab remembers its own selection, scroll position and search; the tree's expanded branches and groups stay as they were. `Tab`, card scrolling, copying, comments and `1`–`9` work the same everywhere; `R`, `p` and `e` only act in Tasks, and `c` and `/` act in Tasks, Specs and Decisions. The status line shows the keys of the current tab.

```text
╭─ ◆ Specs (2) ─────────────────────────╮╭─ demo-app-k3f ──────────────────────────────────────────────╮
│ ◐ k3f Checkout flow       ██░░░░░░ 1/4││ Checkout flow                                               │
│ ○ p7a Product search      ░░░░░░░░ 0/2││  IN PROGRESS   P1  [epic]  ██░░░░░░ 1/4                     │
```

- **Tasks** is the tree described above.
- **Specs** lists epics, except templates (epics labelled `template`): this is how the to-spec skill publishes a spec, with the spec in the epic's description and its tickets as children. Each line has the status icon, ID, title and progress over the direct children, `███░░░░░ 3/8` (closed of all; event records are not counted). The card shows the whole description, the progress among the chips and a `Tickets` section with every ticket and its status; `1`–`9` switch to Tasks and select that ticket. Priority groups do not apply here.
- **Decisions** lists tasks of type `decision` as a flat list with the usual card.
- **Memory** shows `bd memories` (key on the left, text as markdown on the right), `No memories` if there are none.
- **Formulas** shows `bd formula list` and templates (epics labelled `template` and bd template molecules, from `bd list --include-templates`): names on the left, the description on the right. bdv never runs formulas.

Memory and Formulas are read with `bd --readonly memories --json`, `bd --readonly formula list --json` and `bd --readonly list --include-templates --flat --json --all -n 0` the first time you open the tab, and again when you press `r` there. A failed read is shown in the panel; the other tabs keep working.

## Keys

| Context | Keys |
|---|---|
| Tree | `↑↓`/`j k` move · `→`/`l` expand · `←`/`h` collapse or go to parent · `Enter`/`Space` expand or collapse · `g`/`G` top or bottom |
| Card | `↑↓`/`j k` scroll · `PgUp`/`PgDn` page · `1`–`9` follow link (in any focus; from Specs and Decisions it opens Tasks) |
| Search | `/` open · typing filters at once · `Enter` pin · `Esc` clear |
| Comment | `m` write a comment on the selected task · `Ctrl+S` send · `Esc` close, the draft is kept |
| Global | `Tab` focus · `[`/`]` tabs · `r` refresh (in Memory and Formulas: reload the tab) · `c` closed · `e` events · `p` priority groups · `R` ready only · `,` bd settings · `?` help · `q`/`Ctrl+C` quit |
| Settings | `,` open · `j k` move · `Enter`/`Space` change · `u` reset to default · `y` confirm · `Esc` back to the tree |
| Copy | `y` ID · `Y` title · `D` description (raw markdown) · `M` the card as markdown (title, ID, status, priority, type, description, notes) |
| Mouse | wheel moves the cursor or scrolls · `S` turns mouse capture off and on |
| Narrow mode | `Enter` open card · `Esc` back to tree |
| No database | `i` offer `bd init` · `c` switch IDs: hash or sequential · `y` run it · `r` retry · `q` quit |

`?` shows the same list inside bdv, and `bdv --help` prints it.

The mouse wheel scrolls the pane under the cursor (over the tree it moves the selection); clicks are ignored.

A task is waiting (blocked) when it is not closed itself and has a `blocks` dependency on a task that is not closed; a blocker missing from `bd list` counts as open. A closed task never waits, whatever its blockers. Waiting tasks get a red `⊘` before the title in the tree, the top line adds `⊘ N waiting` (tasks visible under the `c`/`e` toggles), and the card says `⊘ waiting on N open blockers` under the status chips. `R` keeps only tasks that are ready to work on (not blocked); parents of ready tasks stay as dimmed context. It combines with search, groups and `c`/`e`, and the status line shows `ready`. Blocked tasks are not nested under their blockers: the card's `Blocked by`/`Blocks` sections show the links.

The card shows the title, status chips, labels and dates, then the sections that have content: `Description`, `Notes`, and the spec fields from `bd show` — `Spec` (the `--spec-id` path or link), `Design` and `Acceptance` (markdown) — followed by `Parent`, `Children`, `Blocked by`, `Blocks`, `Related` and `Comments`. Section names stay in English in every language.

Priority groups (`p`) collect root tasks under `P0`…`P4` headers. With a header selected, the card shows how many tasks the group has, a status summary and a `Tasks` section listing them in tree order as `[n] icon id  title` (a red `⊘` marks waiting ones); the list follows `c`, `e`, `R` and search, and `1`–`9` jump to the first nine.

Copying puts the text on the clipboard as `bd` stores it, markdown included; only control characters are removed. A toast says what was copied (`copied ID: …`, `copied title`, `copied description`, `copied card`), or `nothing to copy` when the description is empty or a group header is selected.

To select text with the mouse while bdv captures it, hold Shift and drag (Ghostty, iTerm2) or Fn and drag (Terminal.app). Or press `S` to turn mouse capture off: the terminal's normal selection works, the wheel no longer scrolls the panes, and the status line shows `mouse off` until you press `S` again. While capture is off bdv also turns off the terminal's alternate scroll mode (DECSET 1007), so the wheel is not turned into arrow keys that would move the cursor; it is turned back on by the next `S` and on exit.

## Configuration and language

bdv has no config file of its own. It finds the database the way `bd` does: `--db <path>`, then `BEADS_DIR` / `BEADS_DB`, then `.beads` in the current directory and above. Project settings live in bd and are edited from the settings screen (`,`).

The interface is in English by default. It switches to Russian when the locale starts with `ru`: the first non-empty of `LC_ALL`, `LC_MESSAGES`, `LANG`. `BDV_LANG=en` or `BDV_LANG=ru` overrides the locale.

```sh
BDV_LANG=ru bdv
```

## Why not bv?

[beads_viewer](https://github.com/Dicklesworthstone/beads_viewer) (`bv`) is a great, feature-rich viewer for beads, with graph analytics and more views than bdv will ever have. If you want those, use bv. bdv aims at something smaller: one screen, a tree and a card, a fast start, and a strictly read-only view of your `bd` data apart from the few confirmed writes listed above. It is meant to sit in a split next to your agent and stay out of the way. The two work fine side by side.

## Contributing

Bug reports, ideas and pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for how to build, test and send changes.

## License

[MIT](LICENSE)
