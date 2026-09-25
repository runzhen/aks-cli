package config

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const Version = 1

type Cluster struct {
	Subscription  string `json:"subscription"`
	Location      string `json:"location"`
	ResourceGroup string `json:"resourceGroup"`
	GPUPool       string `json:"gpuPool"`
	ClusterName   string `json:"clusterName"`
	GPUVMSize     string `json:"gpuVmSize"`
}

type Document struct {
	Version  int       `json:"version"`
	Clusters []Cluster `json:"clusters"`
}

func Empty() Document {
	return Document{Version: Version, Clusters: []Cluster{}}
}

func Defaults(username string) Cluster {
	c := Cluster{
		Location:  "westus2",
		GPUPool:   "gpunp",
		GPUVMSize: "Standard_NV36ads_A10_v5",
	}
	if username = strings.TrimSpace(username); username != "" {
		c.ResourceGroup = username + "-gpu-rg"
		c.ClusterName = username + "-gpu"
	}
	return c
}

var (
	clusterName = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9_-]{0,61}[A-Za-z0-9])?$`)
	poolName    = regexp.MustCompile(`^[a-z][a-z0-9]{0,11}$`)
)

func (c Cluster) Clean() (Cluster, error) {
	fields := []struct {
		name  string
		value *string
	}{
		{"SUBSCRIPTION", &c.Subscription},
		{"LOCATION", &c.Location},
		{"RESOURCE_GROUP", &c.ResourceGroup},
		{"GPU_POOL", &c.GPUPool},
		{"CLUSTER_NAME", &c.ClusterName},
		{"GPU_VM_SIZE", &c.GPUVMSize},
	}
	for _, field := range fields {
		if !utf8.ValidString(*field.value) {
			return Cluster{}, fmt.Errorf("%s must contain valid UTF-8", field.name)
		}
		for _, r := range *field.value {
			if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029' {
				return Cluster{}, fmt.Errorf("%s must not contain control or formatting characters", field.name)
			}
		}
		*field.value = strings.TrimSpace(*field.value)
		if *field.value == "" {
			return Cluster{}, fmt.Errorf("%s is required", field.name)
		}
	}
	if !clusterName.MatchString(c.ClusterName) {
		return Cluster{}, fmt.Errorf("CLUSTER_NAME must be 1-63 letters, digits, underscores or hyphens, starting and ending with a letter or digit")
	}
	if !poolName.MatchString(c.GPUPool) {
		return Cluster{}, fmt.Errorf("GPU_POOL must be 1-12 lowercase letters or digits, starting with a letter")
	}
	if utf8.RuneCountInString(c.ResourceGroup) > 90 || strings.HasSuffix(c.ResourceGroup, ".") {
		return Cluster{}, fmt.Errorf("RESOURCE_GROUP must be 1-90 characters and must not end with a period")
	}
	for _, r := range c.ResourceGroup {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("_-.()", r) {
			return Cluster{}, fmt.Errorf("RESOURCE_GROUP permits letters, digits, underscores, hyphens, periods and parentheses")
		}
	}
	return c, nil
}

func (c Cluster) identity() [3]string {
	return [3]string{
		strings.ToLower(c.Subscription),
		strings.ToLower(c.ResourceGroup),
		strings.ToLower(c.ClusterName),
	}
}

func (d Document) Clean() (Document, error) {
	if d.Version != Version {
		return Document{}, fmt.Errorf("unsupported config version %d (expected %d)", d.Version, Version)
	}
	if d.Clusters == nil {
		return Document{}, fmt.Errorf("config clusters must be an array, not null or missing")
	}
	result := Empty()
	seen := make(map[[3]string]bool, len(d.Clusters))
	for i, entry := range d.Clusters {
		c, err := entry.Clean()
		if err != nil {
			return Document{}, fmt.Errorf("cluster %d: %w", i+1, err)
		}
		if seen[c.identity()] {
			return Document{}, fmt.Errorf("cluster %d duplicates a saved subscription/resource-group/cluster-name", i+1)
		}
		seen[c.identity()] = true
		result.Clusters = append(result.Clusters, c)
	}
	return result, nil
}
