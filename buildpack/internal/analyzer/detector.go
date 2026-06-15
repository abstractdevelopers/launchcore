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
	// Framework detection files (order matters - more specific first)
	frameworks := []struct {
		file      string
		framework string
	}{
		// Meta-frameworks
		{"next.config.js", "nextjs"},
		{"next.config.ts", "nextjs"},
		{"nuxt.config.ts", "nuxt"},
		{"nuxt.config.js", "nuxt"},
		{"svelte.config.js", "sveltekit"},
		{"svelte.config.ts", "sveltekit"},
		{"remix.config.js", "remix"},
		{"redwood.toml", "redwoodjs"},
		{"blitz.config.js", "blitzjs"},
		{"keystone.js", "keystonejs"},
		{"strapi.config.js", "strapi"},
		
		// Static Site Generators
		{"gatsby-config.js", "gatsby"},
		{"astro.config.mjs", "astro"},
		{"astro.config.ts", "astro"},
		{"11ty.config.js", "eleventy"},
		{"eleventy.config.js", "eleventy"},
		{"hugo.toml", "hugo"},
		{"hugo.yaml", "hugo"},
		{"docusaurus.config.js", "docusaurus"},
		{"docusaurus.config.ts", "docusaurus"},
		{"vitepress.config.js", "vitepress"},
		{"docsify/_sidebar.md", "docsify"},
		
		// Build Tools
		{"vite.config.ts", "vite"},
		{"vite.config.js", "vite"},
		{"vite.config.mjs", "vite"},
		{"webpack.config.js", "webpack"},
		{"webpack.config.ts", "webpack"},
		{"rollup.config.js", "rollup"},
		{"parcel.config.js", "parcel"},
		{"tsconfig.json", "typescript"},
		
		// Full-stack Frameworks
		{"nest-cli.json", "nestjs"},
		{"angular.json", "angular"},
		{"nx.json", "nx"},
		{"turbo.json", "turborepo"},
		{"lerna.json", "lerna"},
		
		// SPAs / Libraries
		{"react-native.config.js", "react-native"},
		{"expo-sdk-version", "expo"},
		{"ionic.config.json", "ionic"},
		{"quasar.conf.js", "quasar"},
		
		// Other
		{"qwik-city-plan", "qwik"},
		{"shopify/hydrogen.config.js", "shopify-hydrogen"},
		{"solid.config.js", "solidjs"},
		{"preact.config.js", "preact"},
		{"lit.config.js", "lit"},
		{"sapper.cnfg", "sapper"},
		{"umirc.ts", "umi"},
		{"umirc.js", "umi"},
		{"genesisis.config.js", "genesis"},
		{"modernizr-config.json", "modernizr"},
		{"gridsome.config.js", "gridsome"},
		{"front-end.config.js", "frontity"},
		{"stencil.config.ts", "stencil"},
		{"sapper.config.js", "sapper"},
		{"prisma/schema.prisma", "prisma"},
		{"trpc", "trpc"},
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
			Scripts    map[string]string `json:"scripts"`
			Dependencies map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			// Detect by dependencies
			if deps := mergeMaps(pkg.Dependencies, pkg.DevDependencies); len(deps) > 0 {
				if _, ok := deps["next"]; ok {
					return "nextjs"
				}
				if _, ok := deps["@nuxt/core"]; ok || _, ok := deps["nuxt"]; ok {
					return "nuxt"
				}
				if _, ok := deps["@sveltejs/kit"]; ok {
					return "sveltekit"
				}
				if _, ok := deps["remix"]; ok {
					return "remix"
				}
				if _, ok := deps["@nestjs/core"]; ok {
					return "nestjs"
				}
				if _, ok := deps["angular"]; ok || _, ok := deps["@angular/core"]; ok {
					return "angular"
				}
				if _, ok := deps["react"]; ok {
					return "react"
				}
				if _, ok := deps["vue"]; ok {
					return "vue"
				}
				if _, ok := deps["@redwoodjs/core"]; ok {
					return "redwoodjs"
				}
				if _, ok := deps["@shopify/hydrogen"]; ok {
					return "shopify-hydrogen"
				}
				if _, ok := deps["@builder.io/qwik"]; ok || _, ok := deps["@builder.io/qwik-city"]; ok {
					return "qwik"
				}
				if _, ok := deps["solid-js"]; ok {
					return "solidjs"
				}
				if _, ok := deps["preact"]; ok {
					return "preact"
				}
				if _, ok := deps["svelte"]; ok {
					return "svelte"
				}
				if _, ok := deps["expo"]; ok {
					return "expo"
				}
				if _, ok := deps["@ionic/react"]; ok {
					return "ionic"
				}
				if _, ok := deps["@quasar/extras"]; ok {
					return "quasar"
				}
			}
			
			// Detect by scripts
			scripts := pkg.Scripts
			if scripts == nil {
				scripts = make(map[string]string)
			}
			
			scriptFramework := detectByScript(scripts)
			if scriptFramework != "" {
				return scriptFramework
			}
		}
	}

	return "node" // generic node
}

func mergeMaps(a, b map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}

func detectByScript(scripts map[string]string) string {
	scriptToFramework := map[string]string{
		"next":          "nextjs",
		"nuxt:dev":      "nuxt",
		"nuxt:build":     "nuxt",
		"gatsby develop": "gatsby",
		"gatsby build":   "gatsby",
		"astro dev":     "astro",
		"astro build":   "astro",
		"svelte-kit":     "sveltekit",
		"remix":         "remix",
		"nest":          "nestjs",
		"build:watch":    "angular",
		"nx":            "nx",
		"build:ssr":     "angular-universal",
		"prisma:generate": "prisma",
		"trpc":          "trpc",
	}
	
	for script := range scripts {
		if fw, ok := scriptToFramework[script]; ok {
			return fw
		}
	}
	
	// Check for common patterns
	for script := range scripts {
		if strings.Contains(script, "next") {
			return "nextjs"
		}
		if strings.Contains(script, "nuxt") {
			return "nuxt"
		}
		if strings.Contains(script, "vite") {
			return "vite"
		}
		if strings.Contains(script, "gatsby") {
			return "gatsby"
		}
		if strings.Contains(script, "astro") {
			return "astro"
		}
		if strings.Contains(script, "nest") {
			return "nestjs"
		}
		if strings.Contains(script, "angular") {
			return "angular"
		}
	}
	
	return ""
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
