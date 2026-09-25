package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Store struct {
	Path string
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "aks-cli.json"), nil
}

func (s Store) Load() (Document, error) {
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return Empty(), nil
	}
	if err != nil {
		return Document{}, fmt.Errorf("read %s: %w", s.Path, err)
	}
	var doc Document
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("decode %s: %w", s.Path, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Document{}, fmt.Errorf("decode %s: expected a single JSON document", s.Path)
	}
	doc, err = doc.Clean()
	if err != nil {
		return Document{}, fmt.Errorf("validate %s: %w", s.Path, err)
	}
	return doc, nil
}

func (s Store) Save(doc Document) (err error) {
	doc, err = doc.Clean()
	if err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	file, err := os.CreateTemp(dir, ".aks-cli-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	closed := false
	defer func() {
		if !closed {
			err = errors.Join(err, file.Close())
		}
		if cleanupErr := os.Remove(file.Name()); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("clean up temporary config: %w", cleanupErr))
		}
	}()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("flush config: %w", err)
	}
	closed = true
	if err := file.Close(); err != nil {
		return fmt.Errorf("close config: %w", err)
	}
	if err := os.Rename(file.Name(), s.Path); err != nil {
		return fmt.Errorf("replace %s: %w", s.Path, err)
	}
	return nil
}
