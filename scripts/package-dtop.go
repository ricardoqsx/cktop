package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type target struct {
	os   string
	arch string
}

type archiveFile struct {
	name string
	mode int64
	data []byte
}

var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

func main() {
	if len(os.Args) != 5 {
		fatalf("usage: go run ./scripts/package-dtop.go VERSION COMMIT BUILD_DATE OUTPUT_DIR")
	}
	version, commit, dateValue, outputDir := os.Args[1], os.Args[2], os.Args[3], os.Args[4]
	if !versionPattern.MatchString(version) {
		fatalf("invalid version %q", version)
	}
	if strings.TrimSpace(commit) == "" || strings.ContainsAny(commit, " \t\r\n") {
		fatalf("commit must be a nonempty value without whitespace")
	}
	buildDate, err := time.Parse(time.RFC3339, dateValue)
	if err != nil {
		fatalf("invalid RFC3339 build date: %v", err)
	}
	if err := packageAll(version, commit, buildDate.UTC(), outputDir); err != nil {
		fatalf("package dtop: %v", err)
	}
}

func packageAll(version, commit string, buildDate time.Time, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	license, err := os.ReadFile("LICENSE")
	if err != nil {
		return err
	}
	readme, err := os.ReadFile("docs/README-dtop.md")
	if err != nil {
		return err
	}
	config, err := os.ReadFile("configs/dtop.conf.example")
	if err != nil {
		return err
	}

	targets := []target{{"darwin", "amd64"}, {"darwin", "arm64"}, {"linux", "amd64"}, {"linux", "arm64"}}
	checksums := make(map[string]string, len(targets))
	for _, target := range targets {
		name := fmt.Sprintf("dtop_%s_%s_%s.tar.gz", version, target.os, target.arch)
		path := filepath.Join(outputDir, name)
		binary, err := build(version, commit, buildDate, target)
		if err != nil {
			return err
		}
		files := []archiveFile{
			{name: "LICENSE", mode: 0o644, data: license},
			{name: "README.md", mode: 0o644, data: readme},
			{name: "dtop", mode: 0o755, data: binary},
			{name: "dtop.conf.example", mode: 0o644, data: config},
		}
		if err := writeArchive(path, buildDate, files); err != nil {
			return err
		}
		digest, err := fileSHA256(path)
		if err != nil {
			return err
		}
		checksums[name] = digest
	}

	names := make([]string, 0, len(checksums))
	for name := range checksums {
		names = append(names, name)
	}
	sort.Strings(names)
	var content strings.Builder
	for _, name := range names {
		fmt.Fprintf(&content, "%s  %s\n", checksums[name], name)
	}
	return os.WriteFile(filepath.Join(outputDir, "SHA256SUMS"), []byte(content.String()), 0o644)
}

func build(version, commit string, buildDate time.Time, target target) ([]byte, error) {
	temporary, err := os.MkdirTemp("", "dtop-build-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(temporary)
	binaryPath := filepath.Join(temporary, "dtop")
	ldflags := fmt.Sprintf("-s -w -X main.version=%s -X main.commit=%s -X main.buildDate=%s", version, commit, buildDate.Format(time.RFC3339))
	command := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags", ldflags, "-o", binaryPath, "./apps/dtop/cmd/dtop")
	command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+target.os, "GOARCH="+target.arch)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("build %s/%s: %w: %s", target.os, target.arch, err, strings.TrimSpace(string(output)))
	}
	return os.ReadFile(binaryPath)
}

func writeArchive(path string, timestamp time.Time, files []archiveFile) (resultErr error) {
	temporary := path + ".tmp"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
		if resultErr != nil {
			_ = os.Remove(temporary)
		}
	}()

	gzipWriter, err := gzip.NewWriterLevel(file, gzip.BestCompression)
	if err != nil {
		return err
	}
	gzipWriter.Header.ModTime = timestamp
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range files {
		header := &tar.Header{Name: entry.name, Mode: entry.mode, Size: int64(len(entry.data)), ModTime: timestamp, Typeflag: tar.TypeReg, Format: tar.FormatUSTAR}
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if _, err := tarWriter.Write(entry.data); err != nil {
			return err
		}
	}
	if err := tarWriter.Close(); err != nil {
		return err
	}
	if err := gzipWriter.Close(); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
