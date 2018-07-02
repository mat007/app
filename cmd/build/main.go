package build

import (
	"bytes"
	"flag"
	"io"
	"strings"
	"time"

	"github.com/mat007/brique"
)

const (
	binName = "docker-app"
	pkgName = "github.com/docker/app"
)

var (
	b = building.Builder()

	experimental = flag.String("experimental", "off", "enable experimental features, on or off")
	now          = time.Now().Format(time.RFC3339)
	commit       = b.GitShortCommit()
	tag          = b.GitTag()
)

// All builds and tests everything
func All(b *building.B) {
	Bin(b)
	Test(b)
}

// Test runs all tests
func Test(b *building.B) {
	TestUnit(b)
	TestE2e(b)
}

// Check runs linters and all tests
func Check(b *building.B) {
	Lint(b)
	Test(b)
}

// Clean cleans build artifacts
func Clean(b *building.B) {
	b.Remove("bin", "_build", binName+"-*.tar.gz")
}

// Lint runs linters
func Lint(b *building.B) {
	b.GoMetaLinter("--config=gometalinter.json", "./...")
}

// Vendor updates vendoring
func Vendor(b *building.B) {
	b.Remove("vendor")
	b.Dep("ensure", "-v")
}

// Bin builds application binaries
func Bin(b *building.B) {
	b.WithOS(func(goos string) {
		b.Go().WithEnv("GOOS="+goos, "CGO_ENABLED=0").
			Run("build", tags(), ldflags(), "-o", "bin/"+binName+"-"+goos+b.Exe(goos), "./cmd/"+binName)
	})
}

// E2e builds end to end test binaries
func E2e(b *building.B) {
	b.WithOS(func(goos string) {
		b.Go().WithEnv("GOOS="+goos, "CGO_ENABLED=0").
			Run("test", tags(), ldflags(), "-c", "-o", "bin/"+binName+"-e2e-"+goos+b.Exe(goos), "./e2e")
	})
}

func tags() string {
	tags := "-tags="
	if *experimental != "off" {
		tags += "experimental"
	}
	return tags
}

func ldflags() string {
	return "-ldflags=-s -w" +
		" -X " + pkgName + "/internal.GitCommit=" + commit +
		" -X " + pkgName + "/internal.Version=" + tag +
		" -X " + pkgName + "/internal.Experimental=" + *experimental +
		" -X " + pkgName + "/internal.BuildTime=" + now
}

// Tars creates tar archives with application and end to end test binaries
func Tars(b *building.B) {
	b.WithOS(func(goos string) {
		b.Tar().WithFiles("bin", binName+"-"+goos+b.Exe(goos)).Run(binName + "-" + goos + ".tar.gz")
		b.Tar().WithFiles("bin", binName+"-e2e-"+goos+b.Exe(goos)).Run(binName + "-e2e-" + goos + ".tar.gz")
	})
}

// TestUnit runs unit tests
func TestUnit(b *building.B) {
	// filter out e2e tests
	buf := &bytes.Buffer{}
	b.Go().WithOutput(buf).Run("list", "./...")
	packages := buf.String()
	cmd := []string{"test"}
	for _, p := range strings.Split(packages, "\n") {
		if !strings.HasSuffix(p, "/e2e") {
			cmd = append(cmd, p)
		}
	}
	b.Go(cmd...)
}

// TestE2e runs end to end tests
func TestE2e(b *building.B) {
	b.Go().WithEnv("CGO_ENABLED=0").Run("test", tags(), ldflags(), "-v", "./e2e")
}

// GradleTest runs end to end tests for the gradle plugin
func GradleTest(b *building.B) {
	r, w := io.Pipe()
	go func() {
		defer w.Close()
		b.Tar().WithOutput(w).Run("-", "Dockerfile.gradle", "bin/"+binName+"-linux", "integrations/gradle")
	}()
	image := binName + "-gradle:" + b.GitTag()
	b.Docker().WithInput(r).Run("build", "-t", image, "-f", "Dockerfile.gradle", "-")
	b.Docker().Run("run", "--rm", image, "bash", "-c", "ls -la && ./gradlew --stacktrace build && cd example && gradle renderIt")
}

// Schemas generates specification/bindata.go from json schemas
func Schemas(b *building.B) {
	b.Go().WithTool(esc(b)).Run("generate", pkgName+"/specification")
}

func esc(b *building.B, args ...string) building.Tool {
	return b.MakeTool(
		"esc",
		"--help",
		"https://github.com/mjibson/esc",
		"FROM golang:"+building.GoVersion+"-alpine"+building.AlpineVersion+`
RUN apk add --no-cache git && \
	go get gopkg.in/mjibson/esc.v0 && \
	mv /go/bin/esc.v0 /go/bin/esc`,
		args...)
}
