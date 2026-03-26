package main

import (
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/yinxulai/ait/internal/prompt"
)

func TestValidateLength(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		wantErr bool
	}{
		{name: "valid", length: 100, wantErr: false},
		{name: "zero", length: 0, wantErr: true},
		{name: "negative", length: -1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLength(tt.length)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
		})
	}
}

func TestGeneratePromptMatchesInternalLogic(t *testing.T) {
	length := 500

	got, err := generatePrompt(length)
	if err != nil {
		t.Fatalf("generatePrompt() error = %v", err)
	}

	want := prompt.GeneratePromptByLength(length)
	if got != want {
		t.Fatal("generated prompt does not match internal/prompt.GeneratePromptByLength")
	}

	if utf8.RuneCountInString(got) != length {
		t.Fatalf("generated prompt rune length = %d, want %d", utf8.RuneCountInString(got), length)
	}
}

func TestGeneratePromptIsDeterministic(t *testing.T) {
	length := 300

	first, err := generatePrompt(length)
	if err != nil {
		t.Fatalf("first generatePrompt() error = %v", err)
	}

	second, err := generatePrompt(length)
	if err != nil {
		t.Fatalf("second generatePrompt() error = %v", err)
	}

	if first != second {
		t.Fatal("expected deterministic prompt generation for the same length")
	}
}

func TestResolveOutputPath(t *testing.T) {
	if got := resolveOutputPath(128, ""); got != defaultOutputPath(128) {
		t.Fatalf("resolveOutputPath() = %s, want %s", got, defaultOutputPath(128))
	}

	custom := "./tmp/prompt_128.txt"
	if got := resolveOutputPath(128, custom); got != custom {
		t.Fatalf("resolveOutputPath() = %s, want %s", got, custom)
	}
}

func TestWritePromptFile(t *testing.T) {
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "nested", "prompt.txt")
	content := "测试 prompt 内容"

	if err := writePromptFile(outputPath, content); err != nil {
		t.Fatalf("writePromptFile() error = %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if string(data) != content {
		t.Fatalf("file content = %q, want %q", string(data), content)
	}
}

func TestRunCreatesDefaultOutputFile(t *testing.T) {
	tempDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWD)
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	length := 120
	if err := run(length, ""); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	outputPath := filepath.Join(tempDir, defaultOutputPath(length))
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if utf8.RuneCountInString(string(data)) != length {
		t.Fatalf("file rune length = %d, want %d", utf8.RuneCountInString(string(data)), length)
	}
}
