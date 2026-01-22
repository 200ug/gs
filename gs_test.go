package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRemote(t *testing.T) {
	tests := []struct {
		input       string
		wantHost    string
		wantPort    string
		wantPath    string
		shouldError bool
	}{
		{"user@host:22:/path/to/sync", "user@host", "22", "/path/to/sync", false},
		{"root@192.168.1.1:2222:/data", "root@192.168.1.1", "2222", "/data", false},
		{"sync@s.example.org:45454:/gs/", "sync@s.example.org", "45454", "/gs", false},
		{"user@host:/path", "", "", "", true},
		{"user@host", "", "", "", true},
		{"", "", "", "", true},
	}

	for _, tt := range tests {
		host, port, path, err := parseRemote(tt.input)
		if tt.shouldError {
			if err == nil {
				t.Errorf("parseRemote(%q) expected error, got none", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseRemote(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if host != tt.wantHost || port != tt.wantPort || path != tt.wantPath {
			t.Errorf("parseRemote(%q) = (%q, %q, %q), want (%q, %q, %q)",
				tt.input, host, port, path, tt.wantHost, tt.wantPort, tt.wantPath)
		}
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	tests := []struct {
		input string
		want  string
	}{
		{"~/documents", filepath.Join(home, "documents")},
		{"~/", filepath.Join(home, "")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~notexpanded", "~notexpanded"},
	}

	for _, tt := range tests {
		got := expandPath(tt.input)
		if got != tt.want {
			t.Errorf("expandPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRemoteForLocal(t *testing.T) {
	cfg := &Config{
		Server:     "user@host",
		Port:       "22",
		RemotePath: "/srv/sync",
	}

	tests := []struct {
		local *Local
		want  string
	}{
		{&Local{Name: "notes", Path: "/home/user/notes"}, "user@host:/srv/sync/notes"},
		{&Local{Name: "projects", Path: "/home/user/projects"}, "user@host:/srv/sync/projects"},
	}

	for _, tt := range tests {
		got := cfg.RemoteForLocal(tt.local)
		if got != tt.want {
			t.Errorf("RemoteForLocal(%v) = %q, want %q", tt.local, got, tt.want)
		}
	}
}

func TestFindLocalForPath(t *testing.T) {
	cfg := &Config{
		Locals: []Local{
			{Name: "notes", Path: "/home/user/notes"},
			{Name: "projects", Path: "/home/user/projects"},
		},
	}

	tests := []struct {
		path     string
		wantName string
	}{
		{"/home/user/notes", "notes"},
		{"/home/user/notes/subdir", "notes"},
		{"/home/user/notes/deep/nested/path", "notes"},
		{"/home/user/projects", "projects"},
		{"/home/user/other", ""},
		{"/home/user/notesbutnotreally", ""},
		{"/other/path", ""},
	}

	for _, tt := range tests {
		local := cfg.FindLocalForPath(tt.path)
		if tt.wantName == "" {
			if local != nil {
				t.Errorf("FindLocalForPath(%q) = %v, want nil", tt.path, local)
			}
			continue
		}
		if local == nil {
			t.Errorf("FindLocalForPath(%q) = nil, want %q", tt.path, tt.wantName)
			continue
		}
		if local.Name != tt.wantName {
			t.Errorf("FindLocalForPath(%q).Name = %q, want %q", tt.path, local.Name, tt.wantName)
		}
	}
}

func TestFindLocalByName(t *testing.T) {
	cfg := &Config{
		Locals: []Local{
			{Name: "notes", Path: "/home/user/notes"},
			{Name: "projects", Path: "/home/user/projects"},
		},
	}

	tests := []struct {
		name     string
		wantPath string
	}{
		{"notes", "/home/user/notes"},
		{"projects", "/home/user/projects"},
		{"nonexistent", ""},
	}

	for _, tt := range tests {
		local := cfg.FindLocalByName(tt.name)
		if tt.wantPath == "" {
			if local != nil {
				t.Errorf("FindLocalByName(%q) = %v, want nil", tt.name, local)
			}
			continue
		}
		if local == nil {
			t.Errorf("FindLocalByName(%q) = nil, want path %q", tt.name, tt.wantPath)
			continue
		}
		if local.Path != tt.wantPath {
			t.Errorf("FindLocalByName(%q).Path = %q, want %q", tt.name, local.Path, tt.wantPath)
		}
	}
}

func TestAddLocal(t *testing.T) {
	t.Run("add new local", func(t *testing.T) {
		cfg := &Config{}
		err := cfg.AddLocal("notes", "/home/user/notes")
		if err != nil {
			t.Fatalf("AddLocal() unexpected error: %v", err)
		}
		if len(cfg.Locals) != 1 {
			t.Fatalf("expected 1 local, got %d", len(cfg.Locals))
		}
		if cfg.Locals[0].Name != "notes" || cfg.Locals[0].Path != "/home/user/notes" {
			t.Errorf("local = %v, want {notes, /home/user/notes}", cfg.Locals[0])
		}
	})

	t.Run("duplicate name rejected", func(t *testing.T) {
		cfg := &Config{
			Locals: []Local{{Name: "notes", Path: "/home/user/notes"}},
		}
		err := cfg.AddLocal("notes", "/home/user/other")
		if err == nil {
			t.Error("AddLocal() expected error for duplicate name, got none")
		}
	})

	t.Run("duplicate path rejected", func(t *testing.T) {
		cfg := &Config{
			Locals: []Local{{Name: "notes", Path: "/home/user/notes"}},
		}
		err := cfg.AddLocal("other", "/home/user/notes")
		if err == nil {
			t.Error("AddLocal() expected error for duplicate path, got none")
		}
	})

	t.Run("subpath rejected", func(t *testing.T) {
		cfg := &Config{
			Locals: []Local{{Name: "notes", Path: "/home/user/notes"}},
		}
		err := cfg.AddLocal("subdir", "/home/user/notes/subdir")
		if err == nil {
			t.Error("AddLocal() expected error for subpath, got none")
		}
	})
}

func TestRemoveLocal(t *testing.T) {
	t.Run("remove existing", func(t *testing.T) {
		cfg := &Config{
			Locals: []Local{
				{Name: "notes", Path: "/home/user/notes"},
				{Name: "projects", Path: "/home/user/projects"},
			},
		}
		cfg.RemoveLocal("notes")
		if len(cfg.Locals) != 1 {
			t.Fatalf("expected 1 local after remove, got %d", len(cfg.Locals))
		}
		if cfg.Locals[0].Name != "projects" {
			t.Errorf("remaining local = %v, want projects", cfg.Locals[0])
		}
	})

	t.Run("remove nonexistent is no-op", func(t *testing.T) {
		cfg := &Config{
			Locals: []Local{{Name: "notes", Path: "/home/user/notes"}},
		}
		cfg.RemoveLocal("nonexistent")
		if len(cfg.Locals) != 1 {
			t.Fatalf("expected 1 local after no-op remove, got %d", len(cfg.Locals))
		}
	})

	t.Run("remove last local", func(t *testing.T) {
		cfg := &Config{
			Locals: []Local{{Name: "notes", Path: "/home/user/notes"}},
		}
		cfg.RemoveLocal("notes")
		if len(cfg.Locals) != 0 {
			t.Fatalf("expected 0 locals after remove, got %d", len(cfg.Locals))
		}
	})
}
