package jsonrpc_test

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/zstd"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/web/router"
)

// Check the actual frontend embedded in the release, rather than only checking
// the backend source. A newer frontend can require RPCs absent in this backend.
func TestEmbeddedFrontendRPCMethodsRegistered(t *testing.T) {
	archivePath := filepath.Join("..", "..", "public", "defaultTheme", "dist.tar.zst")
	compressed, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read embedded frontend archive: %v", err)
	}
	decoder, err := zstd.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("open embedded frontend archive: %v", err)
	}
	defer decoder.Close()

	registered := make(map[string]bool)
	for _, method := range rpc.ListMethods() {
		registered[method] = true
	}
	methodPattern := regexp.MustCompile(`["']((?:admin|common|public):[A-Za-z0-9_]+)["']`)
	checked := make(map[string]bool)
	archive := tar.NewReader(decoder)
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read frontend archive entry: %v", err)
		}
		if header.Typeflag != tar.TypeReg || !strings.HasSuffix(header.Name, ".js") {
			continue
		}
		source, err := io.ReadAll(archive)
		if err != nil {
			t.Fatalf("read frontend asset %s: %v", header.Name, err)
		}
		for _, match := range methodPattern.FindAllSubmatch(source, -1) {
			method := string(match[1])
			if checked[method] {
				continue
			}
			checked[method] = true
			if !registered[method] {
				t.Errorf("frontend asset %s requires unregistered RPC method %q", header.Name, method)
			}
		}
	}
	if len(checked) == 0 {
		t.Fatal("no frontend RPC methods found; compatibility check did not inspect the bundle")
	}
	t.Logf("checked %d frontend RPC methods against the backend registry", len(checked))
}

func TestNotificationFrontendRoutesAvailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	router.Register(r)
	routes := make(map[string]bool)
	for _, route := range r.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, route := range []string{
		"GET /api/admin/settings/message-sender",
		"POST /api/admin/settings/message-sender",
		"POST /api/admin/test/sendMessage",
	} {
		if !routes[route] {
			t.Errorf("notification frontend requires missing route %s", route)
		}
	}
}
