package commands

import (
	"reflect"
	"strings"
	"testing"

	"aks-cli/internal/config"
)

func TestQuote(t *testing.T) {
	for input, want := range map[string]string{
		"": "''", "westus2": "westus2", "--enable-managed-gpu=true": "--enable-managed-gpu=true",
		"Dev subscription": "'Dev subscription'", "O'Brien": "'O'\"'\"'Brien'",
		"$(touch /tmp/nope); *": "'$(touch /tmp/nope); *'", "a`whoami`": "'a`whoami`'",
		`a\b"c`: `'a\b"c'`,
	} {
		if got := Quote(input); got != want {
			t.Errorf("Quote(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestExactCommandArguments(t *testing.T) {
	c := config.Cluster{
		Subscription: "Team's dev subscription", Location: "eastus2", ResourceGroup: "custom-rg",
		GPUPool: "customgpu", ClusterName: "custom-cluster", GPUVMSize: "Standard_NC24ads_A100_v4",
	}
	want := [][]string{
		{"group", "create", "--name", c.ResourceGroup, "--location", c.Location, "--subscription", c.Subscription},
		{"aks", "create", "--resource-group", c.ResourceGroup, "--name", c.ClusterName, "--location", c.Location,
			"--node-vm-size", "Standard_DS2_v2", "--enable-managed-identity", "--subscription", c.Subscription},
		{"aks", "nodepool", "add", "--resource-group", c.ResourceGroup, "--cluster-name", c.ClusterName,
			"--name", c.GPUPool, "--node-count", "1", "--node-vm-size", c.GPUVMSize, "--enable-managed-gpu=true",
			"--subscription", c.Subscription},
		{"group", "delete", "--name", c.ResourceGroup, "--yes", "--no-wait", "--subscription", c.Subscription},
		{"aks", "delete", "--resource-group", c.ResourceGroup, "--name", c.ClusterName,
			"--yes", "--no-wait", "--subscription", c.Subscription},
		{"aks", "nodepool", "delete", "--resource-group", c.ResourceGroup, "--cluster-name", c.ClusterName,
			"--name", c.GPUPool, "--no-wait", "--subscription", c.Subscription},
	}
	commands := Commands(c)
	if len(commands) != 6 {
		t.Fatalf("expected exactly six commands: %+v", commands)
	}
	for i, command := range commands {
		if command.Executable != "az" || !reflect.DeepEqual(command.Args, want[i]) {
			t.Errorf("command %d: %+v, want %v", i, command, want[i])
		}
	}
	sections := Sections(c)
	if len(sections) != 7 || sections[0].Title != "Variables" {
		t.Fatalf("expected variables and six command sections: %+v", sections)
	}
	wantVariables := "export SUBSCRIPTION='Team'\"'\"'s dev subscription'\nexport LOCATION=eastus2\nexport RESOURCE_GROUP=custom-rg\nexport GPU_POOL=customgpu\nexport CLUSTER_NAME=custom-cluster\nexport GPU_VM_SIZE=Standard_NC24ads_A100_v4"
	if sections[0].Content != wantVariables {
		t.Fatalf("unexpected variables: %q", sections[0].Content)
	}
	var contents []string
	for i, section := range sections {
		contents = append(contents, section.Content)
		if i > 0 && section.Content != commands[i-1].String() {
			t.Errorf("section %d does not match command", i)
		}
		if i > 0 && !reflect.DeepEqual(section.Command, commands[i-1]) {
			t.Errorf("section %d execution differs from preview", i)
		}
		if section.Command.Destructive() != (i >= 4) {
			t.Errorf("section %d has incorrect deletion confirmation requirement", i)
		}
	}
	preview := strings.Join(contents, "\n")
	if strings.Count(preview, "\naz ") != 6 || strings.Contains(preview, "gpu-aks") {
		t.Fatal("preview contains duplicate commands or obsolete section headers")
	}
	for _, command := range commands {
		if strings.Count(preview, command.String()) != 1 {
			t.Errorf("expected command exactly once: %s", command)
		}
	}
	for _, value := range []string{Quote(c.Subscription), c.Location, c.ResourceGroup, c.GPUPool, c.ClusterName,
		c.GPUVMSize} {
		if !strings.Contains(preview, value) {
			t.Errorf("preview missing %q", value)
		}
	}
	if strings.Contains(preview, "az account set") || strings.Contains(preview, "--node-taints") {
		t.Fatal("preview changes active account or adds a taint")
	}
	if strings.Count(preview, "--subscription "+Quote(c.Subscription)) != 6 {
		t.Fatal("not all commands include the quoted subscription")
	}
	wantFirst := "az group create \\\n  --name custom-rg \\\n  --location eastus2 \\\n  --subscription 'Team'\"'\"'s dev subscription'"
	if got := commands[0].String(); got != wantFirst {
		t.Fatalf("rendered command = %q, want %q", got, wantFirst)
	}

	deleteGroup := commands[3]
	if sections[4].Warning != deleteGroup.Warning || !strings.Contains(sections[4].Warning, "ALL resources") {
		t.Fatal("resource group deletion section is missing its warning")
	}
}

func TestMultilineCommands(t *testing.T) {
	c := config.Defaults("alice")
	c.Subscription = "--subscription-like-value"
	for _, command := range Commands(c) {
		rendered := command.String()
		lines := strings.Split(rendered, "\n")
		if len(lines) < 2 || !strings.HasPrefix(lines[0], "az ") {
			t.Fatalf("expected multiline Azure command: %q", rendered)
		}
		for i, line := range lines {
			if i > 0 && !strings.HasPrefix(line, "  --") {
				t.Errorf("option line is not indented: %q", line)
			}
			if i < len(lines)-1 && !strings.HasSuffix(line, " \\") {
				t.Errorf("missing end-of-line continuation: %q", line)
			}
		}
		if strings.HasSuffix(rendered, "\\") || strings.HasSuffix(rendered, "\n") {
			t.Errorf("command has trailing continuation or newline: %q", rendered)
		}
		if !strings.Contains(rendered, "--subscription --subscription-like-value") {
			t.Errorf("option value was split from its flag: %q", rendered)
		}
		words := []string{Quote(command.Executable)}
		for _, arg := range command.Args {
			words = append(words, Quote(arg))
		}
		if strings.ReplaceAll(rendered, " \\\n  ", " ") != strings.Join(words, " ") {
			t.Errorf("line splitting changed command arguments: %q", rendered)
		}
	}
}
