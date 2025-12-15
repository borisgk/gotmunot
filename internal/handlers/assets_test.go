package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadJSNoThumbnails(t *testing.T) {
	// Navigate up from internal/handlers to root
	jsPath := filepath.Join("..", "..", "static", "js", "upload.js")

	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("Failed to read upload.js: %v", err)
	}

	jsCode := string(content)

	forbiddenStrings := []string{
		"new FileReader()",
		"readAsDataURL",
		"createElement('img')",
		".upload-item-thumb",
	}

	for _, str := range forbiddenStrings {
		if strings.Contains(jsCode, str) {
			t.Errorf("upload.js should not contain thumbnail generation logic, but found '%s'", str)
		}
	}
}
