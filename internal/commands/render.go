package commands

import (
	"fmt"
	"regexp"
	"strings"

	"aks-cli/internal/config"
)

type Command struct {
	Executable string
	Args       []string
	Warning    string
}

type Section struct {
	Title   string
	Content string
	Warning string
	Command Command
}

func (c Command) Destructive() bool {
	return len(c.Args) > 1 && (c.Args[1] == "delete" || len(c.Args) > 2 && c.Args[1] == "nodepool" && c.Args[2] == "delete")
}

var safeWord = regexp.MustCompile(`^[a-zA-Z0-9_./:=+-]+$`)

func Quote(value string) string {
	if safeWord.MatchString(value) {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func (c Command) String() string {
	var out strings.Builder
	out.WriteString(Quote(c.Executable))
	expectValue := false
	for _, arg := range c.Args {
		if !expectValue && strings.HasPrefix(arg, "--") {
			out.WriteString(" \\\n  ")
			expectValue = !strings.Contains(arg, "=") &&
				arg != "--enable-managed-identity" && arg != "--yes" && arg != "--no-wait"
		} else {
			out.WriteByte(' ')
			expectValue = false
		}
		out.WriteString(Quote(arg))
	}
	return out.String()
}

func Commands(c config.Cluster) []Command {
	az := func(args ...string) Command {
		return Command{Executable: "az", Args: append(args, "--subscription", c.Subscription)}
	}
	deleteGroup := az("group", "delete", "--name", c.ResourceGroup, "--yes", "--no-wait")
	deleteGroup.Warning = "WARNING: Deleting the resource group deletes ALL resources in that group."
	return []Command{
		az("group", "create", "--name", c.ResourceGroup, "--location", c.Location),
		az("aks", "create", "--resource-group", c.ResourceGroup, "--name", c.ClusterName,
			"--location", c.Location, "--node-vm-size", "Standard_DS2_v2", "--enable-managed-identity"),
		az("aks", "nodepool", "add", "--resource-group", c.ResourceGroup,
			"--cluster-name", c.ClusterName, "--name", c.GPUPool, "--node-count", "1",
			"--node-vm-size", c.GPUVMSize, "--enable-managed-gpu=true"),
		deleteGroup,
		az("aks", "delete", "--resource-group", c.ResourceGroup,
			"--name", c.ClusterName, "--yes", "--no-wait"),
		az("aks", "nodepool", "delete", "--resource-group", c.ResourceGroup,
			"--cluster-name", c.ClusterName, "--name", c.GPUPool, "--no-wait"),
	}
}

func Sections(c config.Cluster) []Section {
	var out strings.Builder
	fmt.Fprintf(&out, "export SUBSCRIPTION=%s\nexport LOCATION=%s\nexport RESOURCE_GROUP=%s\nexport GPU_POOL=%s\nexport CLUSTER_NAME=%s\nexport GPU_VM_SIZE=%s",
		Quote(c.Subscription), Quote(c.Location), Quote(c.ResourceGroup), Quote(c.GPUPool), Quote(c.ClusterName), Quote(c.GPUVMSize))
	variables := out.String()
	sections := []Section{{
		Title: "Variables", Content: variables,
		Command: Command{Executable: "/bin/sh", Args: []string{"-c", variables +
			"\nprintf '%s\\n' \"SUBSCRIPTION=$SUBSCRIPTION\" \"LOCATION=$LOCATION\" \"RESOURCE_GROUP=$RESOURCE_GROUP\" \"GPU_POOL=$GPU_POOL\" \"CLUSTER_NAME=$CLUSTER_NAME\" \"GPU_VM_SIZE=$GPU_VM_SIZE\""}},
	}}
	for _, command := range Commands(c) {
		content := command.String()
		title, _, _ := strings.Cut(content, " \\\n")
		sections = append(sections, Section{Title: title, Content: content, Warning: command.Warning, Command: command})
	}
	return sections
}
