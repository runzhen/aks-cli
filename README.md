# aks-cli

A Go terminal UI for saving AKS configurations and previewing Azure CLI commands,
inspired by k9s. Built with upstream [tview](https://github.com/rivo/tview) and
[tcell](https://github.com/gdamore/tcell).

**Preview only:** nothing in this application executes `az`, `gpu-aks`, or a shell.
Saved entries are local configurations, not discovered or verified Azure clusters.

## Build and run

Requires Go 1.25 or newer and an interactive terminal, at least 80 columns by 24
rows. Intended for macOS and Linux.

```sh
go build -o aks-cli .
./aks-cli
```

Alternatively, run `go run .` during development. `./aks-cli --help` prints usage.
Azure CLI and Azure login are not required to use this UI; they are required if
you separately run its generated commands.

## Using the UI

The left pane lists saved clusters with their subscription and resource group.
Select a row to display its parameters and command previews on the right.
Click **New**, fill the six fields, then **Save**. Saving records the configuration
only; it does not provision a cluster.

The right pane has seven bordered sections: **Variables** and one for each of the
six Azure commands. Every section has **Execute** and **Copy** buttons.
**Execute intentionally does nothing.** Copy sends only that section's data to the
system clipboard (no borders, buttons, or warnings). Variables are copied as six
shell-quoted `export NAME=value` statements; commands retain their `\` continuations.
Success or failure is shown below the sections.

| Input | Action |
| --- | --- |
| `n` outside the form / New button | Open a new configuration |
| `Tab` / `Shift-Tab` | Move focus between panes/buttons or form fields |
| Up / Down in the list | Select a configuration |
| Up / Down on the sections container | Scroll between sections |
| Page Up / Page Down anywhere in the preview | Scroll between sections |
| Left / Right on a section's text | Inspect long values horizontally |
| Mouse click | Select a cluster, field, or button |
| Mouse wheel | Scroll the pane under the pointer |
| Enter | Activate a button or advance a form field |
| Escape / Cancel in the form | Discard the unsaved draft |
| Ctrl-C | Quit |

Commands use POSIX-shell `\` line continuations, with each option and its value
on an indented line. Exceptionally long values can still be inspected by focusing
that section's text and scrolling right. Tab traverses each section's text,
Execute, and Copy controls, automatically bringing the focused section into view.

Clipboard support uses `pbcopy` on macOS. On Linux, install `wl-clipboard` for
Wayland or `xclip`/`xsel` for X11, with access to the desktop display session.
Helpers receive the section text through standard input, never as executable
shell code. Missing helpers, unavailable clipboard sessions, and timeouts are
reported visibly. Over SSH, the helper accesses the remote host's clipboard,
not automatically the clipboard on your local computer.

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
To edit or remove saved entries in v1, quit the app, back up the JSON file, and
edit it manually.

## Command behavior

The preview keeps the six configuration variables and displays exactly these
commands once each: `az group create`, `az aks create`, `az aks nodepool add`,
`az group delete`, `az aks delete`, and `az aks nodepool delete`.
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
- Shell-sensitive values are quoted as literal arguments, and preview text is
  rendered without interpreting terminal color tags.

`gpu-aks` itself is unchanged. Live Azure operations, discovery/status, credentials
retrieval, automatic changes to your shell environment, configurable node counts, and editing/deleting entries
inside the UI are outside v1.

## Development

```sh
go test ./...
go vet ./...
go build ./...
```

Tests use temporary configuration paths and simulated terminal screens. They do
not read your real configuration, overwrite your clipboard, or make Azure calls.
