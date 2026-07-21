package ci

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func workflowSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve workflow test path")
	}
	content, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", ".github", "workflows", "geili-ghcr.yml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func TestGHCRPublicationUsesVerifiedImmutableCommit(t *testing.T) {
	workflow := workflowSource(t)
	required := []string{
		"  verify:\n",
		"go test ./... -count=1",
		"build-and-push:\n    name:",
		"needs: verify",
		"org.opencontainers.image.revision=${{ github.sha }}",
		"load: true",
		`docker tag "new-api:ci-$GITHUB_SHA" "$image:sha-$short_sha"`,
		"docker push",
	}
	for _, marker := range required {
		if !strings.Contains(workflow, marker) {
			t.Errorf("workflow missing %q", marker)
		}
	}
	if strings.Contains(workflow, "github.event.inputs.ref") {
		t.Error("workflow allows a mutable arbitrary ref")
	}
	if got := strings.Count(workflow, "ref: ${{ github.sha }}"); got != 2 {
		t.Errorf("immutable checkout count = %d, want 2", got)
	}
	if got := strings.Count(workflow, `test "$(git rev-parse HEAD)" = "$GITHUB_SHA"`); got != 2 {
		t.Errorf("checkout assertion count = %d, want 2", got)
	}
	if strings.Contains(workflow, "push: true") {
		t.Error("workflow rebuilds during publication instead of pushing the verified candidate")
	}
	build := strings.Index(workflow, "load: true")
	verify := strings.Index(workflow, "Verify candidate image revision")
	push := strings.LastIndex(workflow, "docker push")
	if build < 0 || verify < build || push < verify {
		t.Errorf("candidate order invalid: build=%d verify=%d push=%d", build, verify, push)
	}
}
