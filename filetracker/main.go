package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// FileInfo holds information about a file or directory.
type FileInfo struct {
	Path    string    `json:"path"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

// Snapshot stores a mapping from path to file information.
type Snapshot struct {
	Files map[string]FileInfo `json:"files"`
}

// ScanDir walks the given root directory and returns a snapshot.
func ScanDir(root string) (*Snapshot, error) {
	snap := &Snapshot{Files: make(map[string]FileInfo)}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		snap.Files[rel] = FileInfo{
			Path:    rel,
			IsDir:   d.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return snap, nil
}

// SaveSnapshot saves the snapshot to a JSON file.
func SaveSnapshot(s *Snapshot, file string) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}

// LoadSnapshot loads a snapshot from a JSON file.
func LoadSnapshot(file string) (*Snapshot, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var snap Snapshot
	dec := json.NewDecoder(f)
	if err := dec.Decode(&snap); err != nil {
		return nil, err
	}
	if snap.Files == nil {
		snap.Files = make(map[string]FileInfo)
	}
	return &snap, nil
}

// DiffSnapshots compares two snapshots and returns added, modified and deleted files.
func DiffSnapshots(oldSnap, newSnap *Snapshot) (added, modified, deleted []FileInfo) {
	for path, newInfo := range newSnap.Files {
		if oldInfo, ok := oldSnap.Files[path]; !ok {
			added = append(added, newInfo)
		} else {
			if newInfo.IsDir != oldInfo.IsDir || newInfo.Size != oldInfo.Size || !newInfo.ModTime.Equal(oldInfo.ModTime) {
				modified = append(modified, newInfo)
			}
		}
	}
	for path, oldInfo := range oldSnap.Files {
		if _, ok := newSnap.Files[path]; !ok {
			deleted = append(deleted, oldInfo)
		}
	}
	return
}

func main() {
	a := app.New()
	w := a.NewWindow("Folder Snapshot Compare")

	var selected string
	output := widget.NewMultiLineEntry()
	output.SetMinRowsVisible(20)

	selectBtn := widget.NewButton("Select Folder", func() {
		dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if uri == nil || err != nil {
				return
			}
			selected = uri.Path()
			output.SetText(fmt.Sprintf("Selected: %s\n", selected))
		}, w).Show()
	})

	saveBtn := widget.NewButton("Save Snapshot", func() {
		if selected == "" {
			dialog.ShowInformation("Error", "Please select a folder first", w)
			return
		}
		snap, err := ScanDir(selected)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		file := filepath.Join(selected, "snapshot.json")
		if err := SaveSnapshot(snap, file); err != nil {
			dialog.ShowError(err, w)
			return
		}
		output.SetText(output.Text + fmt.Sprintf("Snapshot saved to %s\n", file))
	})

	compareBtn := widget.NewButton("Compare", func() {
		if selected == "" {
			dialog.ShowInformation("Error", "Please select a folder first", w)
			return
		}
		file := filepath.Join(selected, "snapshot.json")
		oldSnap, err := LoadSnapshot(file)
		if err != nil {
			dialog.ShowError(fmt.Errorf("load snapshot: %w", err), w)
			return
		}
		newSnap, err := ScanDir(selected)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		added, modified, deleted := DiffSnapshots(oldSnap, newSnap)
		out := fmt.Sprintf("Added: %d\n", len(added))
		for _, f := range added {
			out += " + " + f.Path + "\n"
		}
		out += fmt.Sprintf("Modified: %d\n", len(modified))
		for _, f := range modified {
			out += " * " + f.Path + "\n"
		}
		out += fmt.Sprintf("Deleted: %d\n", len(deleted))
		for _, f := range deleted {
			out += " - " + f.Path + "\n"
		}
		output.SetText(out)
		if err := SaveSnapshot(newSnap, file); err != nil {
			dialog.ShowError(err, w)
		}
	})

	content := container.NewVBox(selectBtn, saveBtn, compareBtn, output)
	w.SetContent(content)
	w.Resize(fyne.NewSize(600, 400))
	w.ShowAndRun()
}
