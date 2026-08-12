package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

const (
	providerFile = "provider.json"
	modelsDir    = "models"
	ext          = ".json"
)

// providerMeta is the per-provider file. It deliberately records nothing
// derived, such as a model count or a timestamp, because a derived field
// churns the diff of every run that changes nothing else.
type providerMeta struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// WriteProvider writes one provider's models under root, one file each, and
// removes the files of models the provider no longer publishes so a withdrawn
// model appears as a deletion.
//
// The models are sorted in place first. Sorting here rather than expecting the
// caller to do it is what guarantees the committed files are stable: a parser
// that emits prices in map order would otherwise rewrite files whose content
// had not changed, and the diff is the reason these files are split at all.
func WriteProvider(root string, p catalog.Provider) error {
	catalog.SortModels(p.Models)
	dir := filepath.Join(root, fileName(p.ID))
	models := filepath.Join(dir, modelsDir)
	if err := os.MkdirAll(models, 0o755); err != nil {
		return err
	}
	if err := writeJSON(
		filepath.Join(dir, providerFile),
		providerMeta{ID: p.ID, Name: p.Name},
	); err != nil {
		return err
	}
	taken := map[string]string{}
	keep := make(map[string]bool, len(p.Models))
	for _, m := range p.Models {
		name := uniqueName(taken, m.ID)
		keep[name] = true
		if err := writeJSON(filepath.Join(models, name), m); err != nil {
			return err
		}
	}
	return prune(models, keep)
}

// Load reads the whole tree back. A missing root is an empty catalog, not an
// error, so the first run of a fresh checkout works.
func Load(root string) (*catalog.Catalog, error) {
	cat := &catalog.Catalog{}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return cat, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		var meta providerMeta
		if err := readJSON(
			filepath.Join(dir, providerFile),
			&meta,
		); err != nil {
			return nil, err
		}
		models, err := loadModels(filepath.Join(dir, modelsDir))
		if err != nil {
			return nil, err
		}
		cat.Add(meta.ID, meta.Name, models)
	}
	cat.Normalize()
	return cat, nil
}

// loadModels reads every model file in a directory.
func loadModels(dir string) ([]catalog.Model, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	models := make([]catalog.Model, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ext) {
			continue
		}
		var m catalog.Model
		if err := readJSON(filepath.Join(dir, entry.Name()), &m); err != nil {
			return nil, err
		}
		models = append(models, m)
	}
	return models, nil
}

// WriteAggregate writes the single-document form of the catalog.
func WriteAggregate(path string, cat *catalog.Catalog) error {
	return writeJSON(path, cat)
}

// uniqueName returns the filename for a model identifier, disambiguating
// identifiers that differ only in ways a case-insensitive filesystem cannot
// represent.
func uniqueName(taken map[string]string, id string) string {
	base := fileName(id)
	name := base + ext
	for i := 2; ; i++ {
		key := strings.ToLower(name)
		if _, clash := taken[key]; !clash {
			taken[key] = id
			return name
		}
		name = fmt.Sprintf("%s-%d%s", base, i, ext)
	}
}

// fileName makes an identifier safe to use as a path element. Providers name
// models with slashes and colons, which are not filenames anywhere.
func fileName(id string) string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '.', r == '-', r == '_':
			return r
		}
		return '_'
	}, id)
	if safe == "" || strings.Trim(safe, "._") == "" {
		return "_"
	}
	return safe
}

// prune removes files the current run did not write.
func prune(dir string, keep map[string]bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ext) ||
			keep[entry.Name()] {
			continue
		}
		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// writeJSON encodes a value as indented JSON with a trailing newline, leaving
// HTML unescaped so URLs in the data stay readable in a diff.
func writeJSON(path string, value any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// bom is the byte order mark Windows editors prepend to a UTF-8 file. Files
// written here never carry one, but a hand-edited file can.
var bom = []byte{0xEF, 0xBB, 0xBF}

// readJSON decodes a file.
func readJSON(path string, value any) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	body = bytes.TrimPrefix(body, bom)
	if err := json.Unmarshal(body, value); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}
