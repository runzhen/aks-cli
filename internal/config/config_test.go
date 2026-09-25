package config

import (
	"strings"
	"testing"
)

func exampleCluster() Cluster {
	c := Defaults("alice")
	c.Subscription = "Development"
	return c
}

func TestDefaults(t *testing.T) {
	c := Defaults("alice")
	if c.Subscription != "" || c.ResourceGroup != "alice-gpu-rg" || c.ClusterName != "alice-gpu" ||
		c.Location != "westus2" || c.GPUPool != "gpunp" || c.GPUVMSize != "Standard_NV36ads_A10_v5" {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if c := Defaults(""); c.ResourceGroup != "" || c.ClusterName != "" {
		t.Fatalf("missing username produced names: %+v", c)
	}
}

func TestClusterValidation(t *testing.T) {
	tests := []struct {
		name  string
		edit  func(*Cluster)
		valid bool
	}{
		{"defaults", func(c *Cluster) {}, true},
		{"subscription name", func(c *Cluster) { c.Subscription = "My team's [dev] subscription" }, true},
		{"trim spaces", func(c *Cluster) { c.ClusterName = "  cluster  " }, true},
		{"single cluster character", func(c *Cluster) { c.ClusterName = "1" }, true},
		{"max cluster length", func(c *Cluster) { c.ClusterName = strings.Repeat("a", 63) }, true},
		{"cluster underscore", func(c *Cluster) { c.ClusterName = "a_b" }, true},
		{"long cluster", func(c *Cluster) { c.ClusterName = strings.Repeat("a", 64) }, false},
		{"cluster leading hyphen", func(c *Cluster) { c.ClusterName = "-cluster" }, false},
		{"cluster trailing hyphen", func(c *Cluster) { c.ClusterName = "cluster-" }, false},
		{"cluster trailing underscore", func(c *Cluster) { c.ClusterName = "cluster_" }, false},
		{"cluster space", func(c *Cluster) { c.ClusterName = "cluster name" }, false},
		{"single pool character", func(c *Cluster) { c.GPUPool = "a" }, true},
		{"max pool length", func(c *Cluster) { c.GPUPool = "a12345678901" }, true},
		{"long pool", func(c *Cluster) { c.GPUPool = "a123456789012" }, false},
		{"uppercase pool", func(c *Cluster) { c.GPUPool = "GPU" }, false},
		{"pool starting digit", func(c *Cluster) { c.GPUPool = "1gpu" }, false},
		{"pool hyphen", func(c *Cluster) { c.GPUPool = "gpu-pool" }, false},
		{"group punctuation", func(c *Cluster) { c.ResourceGroup = "dev_(gpu).rg-1" }, true},
		{"unicode group", func(c *Cluster) { c.ResourceGroup = "équipe-gpu" }, true},
		{"max group length", func(c *Cluster) { c.ResourceGroup = strings.Repeat("a", 90) }, true},
		{"long group", func(c *Cluster) { c.ResourceGroup = strings.Repeat("a", 91) }, false},
		{"group trailing period", func(c *Cluster) { c.ResourceGroup = "dev." }, false},
		{"group slash", func(c *Cluster) { c.ResourceGroup = "dev/gpu" }, false},
		{"group space", func(c *Cluster) { c.ResourceGroup = "dev gpu" }, false},
		{"escape sequence", func(c *Cluster) { c.Subscription = "dev\x1b[31m" }, false},
		{"newline", func(c *Cluster) { c.Location = "westus2\n" }, false},
		{"null", func(c *Cluster) { c.GPUVMSize = "size\x00" }, false},
		{"bidi", func(c *Cluster) { c.Subscription = "dev\u202e" }, false},
		{"invalid utf8", func(c *Cluster) { c.Subscription = "\xff" }, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := exampleCluster()
			tc.edit(&c)
			got, err := c.Clean()
			if (err == nil) != tc.valid {
				t.Fatalf("Clean() = %+v, %v; valid=%v", got, err, tc.valid)
			}
			if err == nil && got.ClusterName != strings.TrimSpace(c.ClusterName) {
				t.Fatal("cluster name not trimmed")
			}
		})
	}
	for _, name := range []string{"subscription", "location", "group", "pool", "cluster", "size"} {
		t.Run("missing "+name, func(t *testing.T) {
			c := exampleCluster()
			fields := map[string]*string{"subscription": &c.Subscription, "location": &c.Location,
				"group": &c.ResourceGroup, "pool": &c.GPUPool, "cluster": &c.ClusterName, "size": &c.GPUVMSize}
			*fields[name] = "   "
			if _, err := c.Clean(); err == nil {
				t.Fatal("accepted empty field")
			}
		})
	}
}

func TestDocumentValidation(t *testing.T) {
	c := exampleCluster()
	duplicate := c
	duplicate.Subscription = " DEVELOPMENT "
	duplicate.ClusterName = strings.ToUpper(c.ClusterName)
	if _, err := (Document{Version, []Cluster{c, duplicate}}).Clean(); err == nil {
		t.Fatal("accepted duplicate identity")
	}
	for _, field := range []string{"subscription", "group"} {
		other := c
		if field == "subscription" {
			other.Subscription = "Production"
		} else {
			other.ResourceGroup = "other-rg"
		}
		if _, err := (Document{Version, []Cluster{c, other}}).Clean(); err != nil {
			t.Fatalf("rejected same cluster name with distinct %s: %v", field, err)
		}
	}
	for _, doc := range []Document{{Version: 99, Clusters: []Cluster{}}, {Version: Version}} {
		if _, err := doc.Clean(); err == nil {
			t.Fatalf("accepted invalid document: %+v", doc)
		}
	}
	c.Subscription = " Development "
	doc := Document{Version, []Cluster{c}}
	clean, err := doc.Clean()
	if err != nil || clean.Clusters[0].Subscription != "Development" || doc.Clusters[0].Subscription != " Development " {
		t.Fatalf("normalization modified input or failed: %+v, %v", clean, err)
	}
}
