package analyzer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DetectionResult represents the outcome of repository analysis
type DetectionResult struct {
	Language       string                 `json:"language"`
	Framework      string                 `json:"framework"`
	PackageManager string                 `json:"packageManager"`
	Confidence     float64                `json:"confidence"`
	EntryPoint     string                 `json:"entry"`
	Monorepo       bool                   `json:"monorepo"`
	BuildTool      string                 `json:"buildTool"`
	Runtime        string                 `json:"runtime"`
	Port           int                    `json:"port"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	DetectedAt     time.Time              `json:"detected_at"`
}

// Detector is the high-performance repository analyzer
type Detector struct {
	parallel bool
	timeout  time.Duration
}

// NewDetector creates a new detector instance
func NewDetector() *Detector {
	return &Detector{
		parallel: true,
		timeout:  50 * time.Millisecond,
	}
}

// Detect performs O(n) repository type detection
func (d *Detector) Detect(rootPath string) (*DetectionResult, error) {
	start := time.Now()
	defer func() {
		_ = time.Since(start) // Logging happens elsewhere
	}()

	result := &DetectionResult{
		Confidence: 0.5,
		Metadata:   make(map[string]interface{}),
		DetectedAt: time.Now(),
	}

	// Fast path: check for Dockerfile first
	if d.hasDockerfile(rootPath) {
		result.Language = "dockerfile"
		result.Framework = "dockerfile_user_supplied"
		result.Confidence = 1.0
		return result, nil
	}

	// Run detectors in parallel for speed
	var wg sync.WaitGroup
	var mu sync.Mutex

	detectors := []struct {
		name   string
		detect func(string) (string, string, float64, bool)
	}{
		{"node", d.detectNode},
		{"python", d.detectPython},
		{"go", d.detectGo},
		{"rust", d.detectRust},
		{"php", d.detectPHP},
		{"ruby", d.detectRuby},
		{"java", d.detectJava},
		{"dotnet", d.detectDotnet},
	}

	results := make(chan struct {
		language  string
		framework string
		confidence float64
		found     bool
	}, len(detectors))

	for _, detector := range detectors {
		wg.Add(1)
		go func(detectFunc func(string) (string, string, float64, bool)) {
			defer wg.Done()
			lang, framework, conf, found := detectFunc(rootPath)
			if found {
				results <- struct {
					language   string
					framework  string
					confidence float64
					found      bool
				}{lang, framework, conf, found}
			}
		}(detector.detect)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect best result
	var best struct {
		language   string
		framework  string
		confidence float64
	}

	for r := range results {
		if r.confidence > best.confidence {
			best = r
		}
	}

	if best.confidence > 0 {
		result.Language = best.language
		result.Framework = best.framework
		result.Confidence = best.confidence
		d.inferDefaults(result)
	}

	// Check for monorepo
	result.Monorepo = d.detectMonorepo(rootPath, result.Language)

	return result, nil
}

func (d *Detector) hasDockerfile(path string) bool {
	dockerfiles := []string{"Dockerfile", "dockerfile", "Containerfile"}
	for _, df := range dockerfiles {
		if _, err := os.Stat(filepath.Join(path, df)); err == nil {
			return true
		}
	}
	return false
}

func (d *Detector) detectNode(path string) (string, string, float64, bool) {
	markers := map[string]string{
		"package.json":           "node",
		"pnpm-workspace.yaml":    "node",
		"lerna.json":             "node",
		"nx.json":                "node",
		"turbo.json":             "node",
	}

	for marker, lang := range markers {
		if _, err := os.Stat(filepath.Join(path, marker)); err == nil {
			// Detect specific framework
			framework := d.detectNodeFramework(path)
			pm := d.detectPackageManager(path)
			return lang, framework, 0.98, true
		}
	}
	return "", "", 0, false
}

func (d *Detector) detectNodeFramework(path string) string {
	frameworks := []struct {
		file     string
		framework string
	}{
		{"next.config.js", "nextjs"},
		{"nuxt.config.ts", "nuxt"},
		{"nuxt.config.js", "nuxt"},
		{"gatsby-config.js", "gatsby"},
		{"astro.config.mjs", "astro"},
		{"vite.config.ts", "vite"},
		{"vite.config.js", "vite"},
		{"webpack.config.js", "webpack"},
		{"nest-cli.json", "nestjs"},
		{"remix.config.js", "remix"},
		{"svelte.config.js", "sveltekit"},
	}

	for _, f := range frameworks {
		if _, err := os.Stat(filepath.Join(path, f.file)); err == nil {
			return f.framework
		}
	}

	// Check package.json for scripts
	pkgJSON := filepath.Join(path, "package.json")
	if data, err := os.ReadFile(pkgJSON); err == nil {
		var pkg struct {
			Scripts map[string]string `json:"scripts"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			if _, ok := pkg.Scripts["next"]; ok {
				return "nextjs"
			}
			if _, ok := pkg.Scripts["dev"]; ok {
				if _, ok := pkg.Scripts["build"]; ok {
					return "express" // default for custom setups
				}
				return "vite" // likely vite
			}
		}
	}

	return "node" // generic node
}

func (d *Detector) detectPackageManager(path string) string {
	if _, err := os.Stat(filepath.Join(path, "pnpm-lock.yaml")); err == nil {
		return "pnpm"
	}
	if _, err := os.Stat(filepath.Join(path, "yarn.lock")); err == nil {
		return "yarn"
	}
	if _, err := os.Stat(filepath.Join(path, "bun.lockb")); err == nil {
		return "bun"
	}
	return "npm"
}

func (d *Detector) detectPython(path string) (string, string, float64, bool) {
	markers := []struct {
		file     string
		framework string
		conf     float64
	}{
		{"requirements.txt", "python", 0.7},
		{"pyproject.toml", "python", 0.85},
		{"Pipfile", "python", 0.85},
		{"setup.py", "python", 0.9},
		{"manage.py", "django", 0.98},
	}

	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(path, m.file)); err == nil {
			framework := m.framework
			if m.framework == "python" {
				framework = d.detectPythonFramework(path)
			}
			return "python", framework, m.conf, true
		}
	}
	return "", "", 0, false
}

func (d *Detector) detectPythonFramework(path string) string {
	frameworks := []struct {
		file     string
		framework string
	}{
		{"asgi.py", "fastapi"},
		{"main.py", "fastapi"},
		{"wsgi.py", "flask"},
		{"app.py", "flask"},
		{"Django", "django"},
	}

	for _, f := range frameworks {
		if _, err := os.Stat(filepath.Join(path, f.file)); err == nil {
			return f.framework
		}
	}
	return "python"
}

func (d *Detector) detectGo(path string) (string, string, float64, bool) {
	if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
		return "go", "go", 0.98, true
	}
	return "", "", 0, false
}

func (d *Detector) detectRust(path string) (string, string, float64, bool) {
	if _, err := os.Stat(filepath.Join(path, "Cargo.toml")); err == nil {
		return "rust", "rust", 0.98, true
	}
	return "", "", 0, false
}

func (d *Detector) detectPHP(path string) (string, string, float64, bool) {
	markers := []struct {
		file     string
		framework string
	}{
		{"artisan", "laravel"},
		{"composer.json", "php"},
	}

	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(path, m.file)); err == nil {
			return "php", m.framework, 0.95, true
		}
	}
	return "", "", 0, false
}

func (d *Detector) detectRuby(path string) (string, string, float64, bool) {
	if _, err := os.Stat(filepath.Join(path, "Gemfile")); err == nil {
		framework := "rails"
		if _, err := os.Stat(filepath.Join(path, "config.ru")); err != nil {
			framework = "ruby"
		}
		return "ruby", framework, 0.95, true
	}
	return "", "", 0, false
}

func (d *Detector) detectJava(path string) (string, string, float64, bool) {
	markers := []string{"pom.xml", "build.gradle", "build.gradle.kts"}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(path, m)); err == nil {
			framework := "java"
			if strings.Contains(m, "gradle") {
				framework = "spring"
			}
			return "java", framework, 0.95, true
		}
	}
	return "", "", 0, false
}

func (d *Detector) detectDotnet(path string) (string, string, float64, bool) {
	if _, err := os.Stat(filepath.Join(path, "*.csproj")); err == nil {
		return "dotnet", "aspnet", 0.95, true
	}
	return "", "", 0, false
}

func (d *Detector) detectMonorepo(path, language string) bool {
	switch language {
	case "node":
		monorepoMarkers := []string{"pnpm-workspace.yaml", "lerna.json", "nx.json", "turbo.json", "package.json"}
		for _, m := range monorepoMarkers {
			if _, err := os.Stat(filepath.Join(path, m)); err == nil {
				return true
			}
		}
	case "go":
		if _, err := os.Stat(filepath.Join(path, "go.work")); err == nil {
			return true
		}
	}
	return false
}

func (d *Detector) inferDefaults(result *DetectionResult) {
	portDefaults := map[string]int{
		"nextjs":  3000,
		"nuxt":    3000,
		"vite":    3000,
		"gatsby":  8000,
		"astro":   3000,
		"nestjs":  3000,
		"express": 3000,
		"fastapi": 8000,
		"django":  8000,
		"flask":   5000,
		"rails":   3000,
		"go":      8080,
		"rust":    8080,
	}

	if port, ok := portDefaults[result.Framework]; ok {
		result.Port = port
	} else if result.Port == 0 {
		result.Port = 3000 // generic default
	}

	// Infer runtime
	runtimes := map[string]string{
		"node":    "node:22-alpine",
		"python":  "python:3.12-alpine",
		"go":      "golang:1.22-alpine",
		"rust":    "rust:1.77-alpine",
		"php":     "php:8.3-fpm-alpine",
		"ruby":    "ruby:3.3-alpine",
		"java":    "eclipse-temurin:21-jdk-alpine",
		"dotnet":  "mcr.microsoft.com/dotnet/aspnet:8.0",
	}

	if runtime, ok := runtimes[result.Language]; ok {
		result.Runtime = runtime
	}
}
