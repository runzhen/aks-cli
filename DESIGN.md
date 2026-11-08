aks-cli is a CLI tool to create, delete, manage multiple AKS instance, it is an enhance version to the shell script ./gpu-aks 

Functional requirement
1. design a CLI tool like [k9s](https://k9scli.io/), it has a simple UI in CLI.
2. basically, it has two columns, left side list all the AKS cluster it managed, when move cursor or mouse to one of the AKS cluster, use all the enviroments to render the commands (gpu-aks create, create-np, print) in gpu-aks and show in right side 
3. at the left top, create a "New" button, when click, show all the environments SUBSCRIPTION,  LOCATION, RESOURCE_GROUP, GPU_POOL, CLUSTER_NAME, GPU_VM_SIZE, and ask user to fill. 
4. after user fill up the enviroments, click "Save", save the all the config in ~/.config/aks-cli.json 


Non-Functional requirement
1. prefer using Go, but if you better option, feel free to propose. 

## Agreed v1 implementation

- Use Go with upstream tview/tcell for the keyboard/mouse terminal UI.
- Save local configurations and preview, copy, or explicitly execute commands.
  Do not discover live clusters. Selection changes with arrow keys or a
  mouse click, not mouse hover.
- Generate commands natively; leave `gpu-aks` unchanged. Its fixed resource-group,
  cluster and pool assignments would otherwise override saved values.
- Map `GPU_POOL` to Azure's node pool name (`NODE_POOL` in the script).
- Scope every resource command with `--subscription` instead of changing the
  active account. Leave subscription empty in the New form.
- Follow the actual `create-np` implementation: one GPU node, managed GPU enabled,
  and no taint. The existing script's `print` output differs by showing a taint.
- Show deletion commands with a resource-group deletion warning; require
  confirmation before executing each delete command, with Cancel selected by default.
- Keep the six parameter values and one copy each of `az group create`,
  `az aks create`, `az aks nodepool add`, `az aks get-credentials`,
  `az group delete`, `az aks delete`, and `az aks nodepool delete`, without duplicate
  `gpu-aks` sections. Credential retrieval uses standard user credentials and
  displays no extra warning text.
- Display Variables and each command in its own section, each with Execute and
  Copy buttons. Copy writes only that section's data to the system clipboard and
  reports errors visibly. Variables use shell-quoted export statements.
- Each section is a multiline editor initialized with its generated text. Copy
  and Execute use the current editor content. Retain per-cluster drafts only for
  the current session, without changing the saved configuration schema.
- Execute opens a third pane below the left and right panes, running the selected
  command in a real PTY with terminal emulation, live output, input, and exit status.
  Run one command at a time. F2 toggles the pane; Escape returns to the UI;
  Ctrl-C interrupts a running command; Ctrl-Q quits and cleans up local processes.
- Execute Variables applies exports only in a child shell and prints their values.
  Unmodified Azure commands use their exact structured argument lists. Modified
  sections require confirmation and run the exact edited text with `/bin/sh -c`.
  Modified Variables does not append an automatic printout.
- Store a versioned JSON document at `~/.config/aks-cli.json` using validated,
  atomic saves. Support one running instance at a time.
- Add a clickable `x` at the right end of each saved cluster name row. Remove
  the entry from the JSON file before refreshing the UI; retain it and display
  an error if saving fails. Never delete Azure resources through this control.
- See `README.md` for defaults, shortcuts, schema, and v1 limitations.
