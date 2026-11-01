# aks-cli

A Go terminal UI for saving AKS configurations and running Azure CLI commands,
inspired by k9s. Built with upstream [tview](https://github.com/rivo/tview) and
[tcell](https://github.com/gdamore/tcell).

**Execute runs real Azure operations.** Previewing, copying, saving, and removing
local configurations do not run Azure commands. Delete commands require confirmation.
Saved entries are local configurations, not discovered or verified Azure clusters.

## Build and run

Requires Go 1.25 or newer and an interactive terminal, at least 80 columns by 24
rows. Intended for macOS and Linux.

```sh
make
./aks-cli
```

`make run` builds and launches the UI. Without Make, use `go build -o aks-cli .`
or `go run .` during development. `./aks-cli --help` prints usage.
Azure CLI on `PATH` and an existing Azure login are required for Execute on Azure
commands. Browsing, saving, and copying configurations work without Azure CLI.

## Using the UI

The left pane lists saved clusters with their subscription and resource group.
Select a row to display its parameters and command previews on the right.
Click **New**, fill the six fields, then **Save**. Saving records the configuration
only; it does not provision a cluster.

Click the **x** at the right edge of a cluster's name row to remove that saved
configuration from `~/.config/aks-cli.json`. The list and preview refresh after
the save succeeds. This does **not** delete any Azure resources. If saving fails,
the entry remains and an error is shown.

The right pane has eight bordered sections: **Variables** and one for each of the
seven Azure commands. Every section has **Execute** and **Copy** buttons.
**Variables** appears immediately after `az aks nodepool add`, before
`az aks get-credentials`.
Each section is an editable multiline text box, initially filled with the
generated default. Click the text or focus it with Tab to edit it. A `*` in the
section title marks a modified command. **Copy and Execute use the current text**,
including any added flags, changed values, or additional shell commands.
Edits are kept separately for each saved cluster during the current app session.
They are not written to `aks-cli.json` and reset to generated defaults on restart.
Execute opens a terminal pane across the bottom, beneath both existing panes,
and runs that section. Copy sends only that section's data to the
system clipboard (no borders, buttons, or warnings). Variables are copied as six
shell-quoted `export NAME=value` statements; commands retain their `\` continuations.
Clipboard success or failure is shown below the sections. Command output and
exit status appear in the terminal pane.

| Input | Action |
| --- | --- |
| `n` outside editors, the form, and terminal / New button | Open a new configuration |
| `Tab` / `Shift-Tab` | Move focus between panes/buttons or form fields |
| Ctrl-A / double-click a field in New | Select all text; typing or pasting replaces the selection |
| Up / Down in the list | Select a configuration |
| Arrow keys in a command editor | Move the editing cursor |
| Enter in a command editor | Insert a newline |
| Ctrl-A in a command editor | Select all; typing or pasting replaces the selection |
| Ctrl-Z / Ctrl-Y in a command editor | Undo / redo |
| Up / Down outside command editors, or Page Up / Page Down anywhere in the preview | Scroll between sections |
| Escape in a command editor | Leave the editor and focus section navigation |
| Mouse click | Select a cluster, field, or button |
| Mouse wheel over the saved list or command sections | Scroll that pane |
| Enter | Activate a button or advance a form field |
| Escape / Cancel in the form | Discard the unsaved draft |
| F2 | Show/hide the bottom terminal; showing it also focuses it |
| Escape in the terminal | Return focus to the command sections |
| Ctrl-C | Interrupt the running command; quit when no command is running |
| Ctrl-Q | Quit and terminate any remaining local command processes |

Commands use POSIX-shell `\` line continuations, with each option and its value
on an indented line. Exceptionally long values can still be inspected by focusing
that section's text and scrolling right. Tab traverses each section's text,
Execute, and Copy controls, automatically bringing the focused section into view.

In the New form, Command-A also selects all when your terminal forwards it as
Meta-A/Alt-A or Ctrl-A. Some macOS terminals reserve Command-A for their own
selection: map Command-A to send `0x01` (Ctrl-A), or use Ctrl-A/double-click instead.
These bindings do not change Ctrl-A behavior inside the execution terminal.

Clipboard support uses `pbcopy` on macOS. On Linux, install `wl-clipboard` for
Wayland or `xclip`/`xsel` for X11, with access to the desktop display session.
Helpers receive the section text through standard input, never as executable
shell code. Missing helpers, unavailable clipboard sessions, and timeouts are
reported visibly. Over SSH, the helper accesses the remote host's clipboard,
not automatically the clipboard on your local computer.

### Execution terminal

The bottom pane uses a real pseudo-terminal (`creack/pty`) and a VT terminal
emulator (`vt10x`). It displays stdout/stderr, ANSI colors, progress updates,
interactive prompts, and the final exit status. Click it to focus it; normal
typing, Enter, Tab, and arrow keys are sent to the child process while focused.
Escape returns focus to the UI. F2 provides the terminal-panel toggle analogous
to VS Code's Command-J; terminal apps cannot reliably receive macOS Command-J.

Each Execute starts a fresh command session and replaces the previous terminal
screen. The screen remains available after exit and while hidden. Only one
command can run at a time; hiding the pane or selecting another saved cluster
does not cancel or retarget it. The pane displays the current terminal screen,
not a persistent scrollback log or a general-purpose interactive shell.

Unmodified Azure commands run directly with the exact generated argument list.
**Edited sections run their current text with `/bin/sh -c`**, supporting multiline
commands, pipes, and redirects. Every edited command requires confirmation,
even if its original section was not destructive; a scrollable review shows the
exact text before execution. Empty commands are rejected. Edits can remove
subscription scoping or perform arbitrary operations, so review them carefully.

Unmodified **Variables** runs its quoted exports
in a short-lived `/bin/sh` process and prints the resulting values. This does not
change your parent shell's environment; Azure commands already include their
saved configuration values explicitly. Edited Variables runs only the edited
text, without appending a printout.

The three delete commands open a confirmation dialog with **Cancel selected by
default**. Check the resource names and subscription before confirming. An
`Exit 0` for a `--no-wait` deletion means the CLI accepted the request, not that
Azure has completed deletion. Interrupting or quitting the local CLI does not
undo Azure operations already submitted. Local command processes are terminated
when you quit; detached/background jobs are not supported in this pane.

The New form has these editable defaults:

| Field | Default |
| --- | --- |
| `SUBSCRIPTION` | Empty; enter a subscription name or ID |
| `LOCATION` | `westus2` |
| `RESOURCE_GROUP` | `$USER-gpu-rg`, or empty if `USER` is unavailable |
| `GPU_POOL` | `gpunp` |
| `CLUSTER_NAME` | `$USER-gpu`, or empty if `USER` is unavailable |
| `GPU_VM_SIZE` | `Standard_NV36ads_A10_v5` |

All fields are required. Names are checked against Azure's resource-group,
AKS cluster, and Linux node-pool naming rules. Leading/trailing spaces are
trimmed, and control/formatting characters are rejected. Subscription authorization,
region/VM-size availability, quotas, and Azure resource existence are not checked.

## Configuration

Configurations are stored at exactly `~/.config/aks-cli.json`, including on macOS.
`XDG_CONFIG_HOME` does not override this location. For example:

```json
{
  "version": 1,
  "clusters": [
    {
      "subscription": "Development",
      "location": "westus2",
      "resourceGroup": "alice-gpu-rg",
      "gpuPool": "gpunp",
      "clusterName": "alice-gpu",
      "gpuVmSize": "Standard_NV36ads_A10_v5"
    }
  ]
}
```

The file is created on the first successful save, with owner-only permissions.
Writes use a flushed temporary file and atomic replacement. A missing file starts
an empty list; invalid JSON, unsupported schema versions, unknown fields, invalid
entries, and filesystem failures produce errors instead of silently resetting data.
On a form error, input is retained for correction or retry.

Duplicate subscription/resource-group/cluster-name tuples are rejected
case-insensitively. The same cluster name is allowed in different subscriptions
or resource groups. Subscription aliases are not resolved: a name and ID for the
same subscription cannot be recognized as equivalent offline.

**Use only one running instance.** Cross-process editing/merging is not supported.
To edit saved entries in v1, quit the app, back up the JSON file, and edit it
manually. Entries can be removed with the list's **x** control.

## Command behavior

The preview keeps the six configuration variables and displays exactly these
commands once each: `az group create`, `az aks create`, `az aks nodepool add`,
`az aks get-credentials`, `az group delete`, `az aks delete`, and
`az aks nodepool delete`.
There are no duplicate `gpu-aks` sections. Commands follow the original script
with these deliberate differences:

- All resource commands include the saved `--subscription`. No `az account set`
  is generated, so commands do not depend on or modify the active subscription.
- Saved resource-group, cluster, and GPU pool names are used directly.
  The design's `GPU_POOL` corresponds to the script's `NODE_POOL`.
- GPU creation follows the script's actual `create-np` behavior: one node and
  `--enable-managed-gpu=true`, without the taint that appears only in its `print`
  output. System-node size stays `Standard_DS2_v2`.
- Deletion commands are alternatives for the chosen scope, not a sequence to run
  together. **Deleting a resource group deletes ALL resources in it**, not just
  the AKS cluster. Review destructive commands before copying them elsewhere.
- Generated shell-sensitive values are quoted as literal arguments, and editor
  text is rendered without interpreting terminal color tags.
- `az aks get-credentials` retrieves standard user credentials for the selected
  cluster and subscription, updates your local kubeconfig, and changes its current
  context. It does not request `--admin` or force `--overwrite-existing`; answer
  any Azure CLI overwrite prompt in the terminal pane.

`gpu-aks` itself is unchanged. Automatic discovery/status,
automatic changes to your parent shell environment, configurable node counts, and
editing entries inside the UI are outside v1.

## Development

```sh
make check      # Tests, vet, and build
make test-race  # Tests with the race detector
make fmt        # Format Go source
```

Use `make help` for all targets. `make clean` removes only `./aks-cli`, leaving
saved configurations untouched. Override the Go executable with `make GO=/path/to/go`.

Tests use temporary configuration paths, simulated terminal screens, and local
PTY fixtures. They do not read your real configuration, overwrite your clipboard,
or make Azure calls.
