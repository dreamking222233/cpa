package api

import (
	"bytes"
	"testing"
)

func TestInjectManagementAuthFilesBulkHTML(t *testing.T) {
	input := []byte("<html><body><div id=\"root\"></div></body></html>")

	output := injectManagementAuthFilesBulkHTML(input)

	if !bytes.Contains(output, []byte(`id="cpa-auth-files-bulk-script"`)) {
		t.Fatalf("expected injected script, got %s", string(output))
	}
	if !bytes.Contains(output, []byte(`</body>`)) {
		t.Fatalf("expected body marker to remain, got %s", string(output))
	}
	if bytes.Index(output, []byte(`id="cpa-auth-files-bulk-script"`)) > bytes.Index(output, []byte(`</body>`)) {
		t.Fatalf("expected script before body close, got %s", string(output))
	}
}

func TestInjectManagementAuthFilesBulkHTMLIsIdempotent(t *testing.T) {
	input := injectManagementAuthFilesBulkHTML([]byte("<html><body></body></html>"))
	output := injectManagementAuthFilesBulkHTML(input)

	if bytes.Count(output, []byte(`id="cpa-auth-files-bulk-script"`)) != 1 {
		t.Fatalf("expected one injected script, got %d", bytes.Count(output, []byte(`id="cpa-auth-files-bulk-script"`)))
	}
}
