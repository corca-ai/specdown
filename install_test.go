package specdown

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const installerTestVersion = "v1.2.3"

func TestInstallerVerifiesAndInstallsReleaseArchive(t *testing.T) {
	archiveName := installerArchiveName(t)
	archive := installerArchive(t, []byte("#!/bin/sh\necho specdown\n"))
	checksum := fmt.Sprintf("%x  %s\n", sha256.Sum256(archive), archiveName)
	server := installerServer(t, archiveName, archive, checksum, "")

	installDir := t.TempDir()
	output, err := runInstaller(t, installDir, server.URL, nil)
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, output)
	}

	body, err := os.ReadFile(filepath.Join(installDir, "specdown"))
	if err != nil {
		t.Fatalf("read installed binary: %v", err)
	}
	if string(body) != "#!/bin/sh\necho specdown\n" {
		t.Fatalf("installed binary = %q", body)
	}
}

func TestInstallerRejectsChecksumMismatch(t *testing.T) {
	archiveName := installerArchiveName(t)
	archive := installerArchive(t, []byte("specdown"))
	checksum := fmt.Sprintf("%064d  %s\n", 0, archiveName)
	server := installerServer(t, archiveName, archive, checksum, "")

	installDir := t.TempDir()
	output, err := runInstaller(t, installDir, server.URL, nil)
	if err == nil {
		t.Fatalf("install.sh succeeded with a mismatched checksum:\n%s", output)
	}
	if !strings.Contains(output, "Checksum mismatch") {
		t.Fatalf("output does not explain checksum mismatch:\n%s", output)
	}
	if _, statErr := os.Stat(filepath.Join(installDir, "specdown")); !os.IsNotExist(statErr) {
		t.Fatalf("binary installed after checksum mismatch: %v", statErr)
	}
}

func TestInstallerRejectsUnsupportedOperatingSystem(t *testing.T) {
	fakeBin := t.TempDir()
	unamePath := filepath.Join(fakeBin, "uname")
	if err := os.WriteFile(unamePath, []byte("#!/bin/sh\ncase \"$1\" in\n  -s) echo Plan9 ;;\n  -m) echo x86_64 ;;\nesac\n"), 0o755); err != nil {
		t.Fatalf("write fake uname: %v", err)
	}

	installDir := t.TempDir()
	output, err := runInstaller(t, installDir, "http://127.0.0.1:1", []string{
		"PATH=" + fakeBin + string(os.PathListSeparator) + os.Getenv("PATH"),
	})
	if err == nil {
		t.Fatalf("install.sh succeeded on an unsupported OS:\n%s", output)
	}
	if !strings.Contains(output, "Unsupported operating system: plan9") {
		t.Fatalf("output does not identify unsupported OS:\n%s", output)
	}
}

func TestInstallerRejectsFailedAndPartialDownloads(t *testing.T) {
	archiveName := installerArchiveName(t)
	archive := installerArchive(t, []byte("specdown"))
	checksum := fmt.Sprintf("%x  %s\n", sha256.Sum256(archive), archiveName)

	tests := []struct {
		name string
		mode string
	}{
		{name: "failed", mode: "failed"},
		{name: "partial", mode: "partial"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := installerServer(t, archiveName, archive, checksum, tt.mode)
			installDir := t.TempDir()
			output, err := runInstaller(t, installDir, server.URL, nil)
			if err == nil {
				t.Fatalf("install.sh succeeded after %s download:\n%s", tt.name, output)
			}
			if _, statErr := os.Stat(filepath.Join(installDir, "specdown")); !os.IsNotExist(statErr) {
				t.Fatalf("binary installed after %s download: %v", tt.name, statErr)
			}
		})
	}
}

func installerArchiveName(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("install.sh supports macOS and Linux")
	}

	var arch string
	switch runtime.GOARCH {
	case "amd64":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	default:
		t.Skipf("install.sh does not support %s", runtime.GOARCH)
	}
	return "specdown_1.2.3_" + runtime.GOOS + "_" + arch + ".tar.gz"
}

func installerArchive(t *testing.T, binary []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: "specdown",
		Mode: 0o755,
		Size: int64(len(binary)),
	}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tarWriter.Write(binary); err != nil {
		t.Fatalf("write tar body: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return buffer.Bytes()
}

func installerServer(t *testing.T, archiveName string, archive []byte, checksum, mode string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch filepath.Base(request.URL.Path) {
		case "checksums.txt":
			_, _ = fmt.Fprint(w, checksum)
		case archiveName:
			switch mode {
			case "failed":
				http.Error(w, "download failed", http.StatusBadGateway)
			case "partial":
				w.Header().Set("Content-Length", fmt.Sprint(len(archive)+10))
				_, _ = w.Write(archive[:len(archive)/2])
			default:
				_, _ = w.Write(archive)
			}
		default:
			http.NotFound(w, request)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func runInstaller(t *testing.T, installDir, releaseBaseURL string, extraEnv []string) (string, error) {
	t.Helper()
	scriptPath, err := filepath.Abs("install.sh")
	if err != nil {
		t.Fatalf("resolve install.sh: %v", err)
	}
	command := exec.Command("/bin/sh", scriptPath)
	command.Env = append(os.Environ(),
		"VERSION="+installerTestVersion,
		"INSTALL_DIR="+installDir,
		"SPECDOWN_RELEASE_BASE_URL="+releaseBaseURL,
	)
	command.Env = append(command.Env, extraEnv...)
	output, err := command.CombinedOutput()
	return string(output), err
}
