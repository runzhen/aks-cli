package config

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestDefaultPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "ignored"))
	path, err := DefaultPath()
	if err != nil || path != filepath.Join(home, ".config", "aks-cli.json") {
		t.Fatalf("DefaultPath() = %q, %v", path, err)
	}
}

func TestStoreRoundTrip(t *testing.T) {
	store := Store{filepath.Join(t.TempDir(), ".config", "aks-cli.json")}
	doc, err := store.Load()
	if err != nil || !reflect.DeepEqual(doc, Empty()) {
		t.Fatalf("missing config: %+v, %v", doc, err)
	}
	first, second := exampleCluster(), exampleCluster()
	second.Subscription = "Production"
	doc.Clusters = []Cluster{first, second}
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.Load()
	if err != nil || !reflect.DeepEqual(doc, reloaded) {
		t.Fatalf("round trip: %+v, %v", reloaded, err)
	}
	info, err := os.Stat(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("config permissions: %o", info.Mode().Perm())
	}
	if err := store.Save(Empty()); err != nil {
		t.Fatal(err)
	}
	reloaded, err = store.Load()
	if err != nil || !reflect.DeepEqual(reloaded, Empty()) {
		t.Fatalf("empty document round trip: %+v, %v", reloaded, err)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(store.Path), ".aks-cli-*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary files left behind: %v, %v", matches, err)
	}
}

func TestLoadInvalidPreservesData(t *testing.T) {
	for _, data := range []string{
		"", "{", "null", `{"version":2,"clusters":[]}`, `{"version":1}`, `{"version":1,"clusters":null}`,
		`{"version":1,"clusters":[{}]}`, `{"version":1,"clusters":[],"typo":true}`,
		`{"version":1,"clusters":[]} {}`, `{"version":1,"clusters":[]} garbage`,
	} {
		t.Run(data, func(t *testing.T) {
			store := Store{filepath.Join(t.TempDir(), "aks-cli.json")}
			if err := os.WriteFile(store.Path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Load(); err == nil {
				t.Fatal("accepted invalid config")
			}
			after, err := os.ReadFile(store.Path)
			if err != nil || string(after) != data {
				t.Fatalf("load modified file: %q, %v", after, err)
			}
		})
	}
	if _, err := (Store{t.TempDir()}).Load(); err == nil {
		t.Fatal("accepted directory as config")
	}
}

func TestFailedSavePreservesFile(t *testing.T) {
	store := Store{filepath.Join(t.TempDir(), "aks-cli.json")}
	if err := store.Save(Document{Version, []Cluster{exampleCluster()}}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(Document{Version: 42}); err == nil {
		t.Fatal("saved invalid document")
	}
	after, err := os.ReadFile(store.Path)
	if err != nil || string(before) != string(after) {
		t.Fatalf("failed save modified file: %v", err)
	}
	if runtime.GOOS != "windows" && os.Geteuid() != 0 {
		dir := filepath.Dir(store.Path)
		if err := os.Chmod(dir, 0500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(dir, 0700); err != nil {
				t.Error(err)
			}
		})
		if err := store.Save(Empty()); err == nil {
			t.Fatal("save succeeded in read-only directory")
		}
		after, err = os.ReadFile(store.Path)
		if err != nil || string(before) != string(after) {
			t.Fatalf("write failure changed previous file: %v", err)
		}
	}
}

func TestSaveFilesystemErrors(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "directory")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err := (Store{target}).Save(Empty()); err == nil {
		t.Fatal("rename over directory succeeded")
	}
	matches, err := filepath.Glob(filepath.Join(parent, ".aks-cli-*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary files left on rename failure: %v, %v", matches, err)
	}
	file := filepath.Join(parent, "file")
	if err := os.WriteFile(file, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := (Store{filepath.Join(file, "config.json")}).Save(Empty()); err == nil {
		t.Fatal("accepted file as parent directory")
	}
}
