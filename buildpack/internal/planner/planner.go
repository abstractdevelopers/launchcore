package planner

import (
	"os"
	"strings"

	"github.com/abstractdevelopers/launchcore/buildpack/internal/analyzer"
)

// BuildPlan represents the complete build execution plan
type BuildPlan struct {
	Name          string                 `json:"name"`
	Version       string                 `json:"version"`
	Stages        []Stage               `json:"stages"`
	Output        string                 `json:"output"`
	Runtime       *RuntimeConfig         `json:"runtime"`
	BuildArgs     []BuildArg             `json:"buildArgs,omitempty"`
	Environment   map[string]string      `json:"environment,omitempty"`
	Ports         []int                 `json:"ports,omitempty"`
	Volumes       []Volume              `json:"volumes,omitempty"`
	Cache         *CacheConfig          `json:"cache,omitempty"`
	Strategy      string                 `json:"strategy"`
	Metadata      *PlanMetadata          `json:"metadata,omitempty"`
	BuildPack     string                 `json:"buildpack"`
	InstallCmd    string                 `json:"installCommand,omitempty"`
	BuildCmd      string                 `json:"buildCommand,omitempty"`
	StartCmd      string                 `json:"startCommand,omitempty"`
	StaticDeploy  bool                   `json:"static,omitempty"`
}

// Volume represents a persistent volume mount
type Volume struct {
	Type        string `json:"type"`        // persistent, cache, tmp, bind
	Name        string `json:"name"`        // volume name
	Source      string `json:"source"`      // host path (for bind)
	Target      string `json:"target"`      // container path
	ReadOnly   bool   `json:"readOnly"`   // mount as read-only
	Share       string `json:"share"`      // sharing mode: shared, private
}

// DockerVolume represents volume configuration for docker-compose output
type DockerVolume struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver,omitempty"`
	External   bool              `json:"external,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

// RuntimeConfig defines runtime settings
type RuntimeConfig struct {
	Image    string            `json:"image"`
	User    string            `json:"user,omitempty"`
	Workdir string            `json:"workdir,omitempty"`
	EnvVars map[string]string `json:"env,omitempty"`
}

// BuildArg represents a Docker build argument
type BuildArg struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// CacheConfig defines caching strategy
type CacheConfig struct {
	Mounts []CacheMount `json:"mounts"`
	Keys   []string     `json:"keys,omitempty"`
}

// CacheMount represents a persistent cache mount
type CacheMount struct {
	Type    string `json:"type"`   // cache, bind
	Source  string `json:"source,omitempty"`
	Target  string `json:"target"`
	Sharing string `json:"sharing,omitempty"`
	ID      string `json:"id,omitempty"`
}

// Stage represents a single build stage
type Stage struct {
	Name       string   `json:"name"`
	Image      string   `json:"image,omitempty"`
	Steps      []Step   `json:"steps"`
	Cache      []string `json:"cache,omitempty"`
	Needs      []string `json:"needs,omitempty"`
	Entrypoint []string `json:"entrypoint,omitempty"`
	Command    string   `json:"command,omitempty"`
}

// Step represents a single build step
type Step struct {
	Type    string       `json:"type"`
	Src     string       `json:"src,omitempty"`
	Dest    string       `json:"dest,omitempty"`
	Command string       `json:"command,omitempty"`
	Args    []string     `json:"args,omitempty"`
	Mount   *CacheMount  `json:"mount,omitempty"`
}

// PlanMetadata contains additional plan information
type PlanMetadata struct {
	EstimatedBuildTimeMs int64    `json:"estimatedBuildTimeMs"`
	CacheHitRate        float64  `json:"cacheHitRate"`
	ImageSizeEstimateMB int      `json:"imageSizeEstimateMB"`
	OptimizationApplied []string `json:"optimizationApplied"`
	DetectedAt          int64    `json:"detectedAt"`
}

// Planner generates optimal build plans
type Planner struct{}

// NewPlanner creates a new planner instance
func NewPlanner() *Planner {
	return &Planner{}
}

// GeneratePlan creates an optimized build plan from detection results
func (p *Planner) GeneratePlan(detection *analyzer.DetectionResult) *BuildPlan {
	plan := &BuildPlan{
		Version:  "1.0",
		Strategy: p.selectStrategy(detection),
		Metadata: &PlanMetadata{
			OptimizationApplied: []string{},
			DetectedAt:         detection.DetectedAt.Unix(),
		},
	}

	switch detection.Language {
	case "node":
		plan = p.planNode(detection)
	case "python":
		plan = p.planPython(detection)
	case "go":
		plan = p.planGo(detection)
	case "rust":
		plan = p.planRust(detection)
	case "php":
		plan = p.planPHP(detection)
	case "dockerfile":
		plan = p.planDockerfile(detection)
	default:
		plan = p.planGeneric(detection)
	}

	plan.Runtime = &RuntimeConfig{
		Image:    plan.Runtime.Image,
		Workdir:  "/app",
		EnvVars:  plan.Environment,
	}

	if detection.Port > 0 {
		plan.Ports = []int{detection.Port}
	}

	return plan
}

func (p *Planner) selectStrategy(detection *analyzer.DetectionResult) string {
	if detection.Framework == "dockerfile_user_supplied" {
		return "dockerfile_user_supplied"
	}
	return "buildpack_" + detection.Language
}

// ============ NODE.JS ============

func (p *Planner) planNode(detection *analyzer.DetectionResult) *BuildPlan {
	pm := detection.PackageManager
	if pm == "" {
		pm = "npm"
	}

	plan := &BuildPlan{
		Output:        p.getNodeOutput(detection),
		BuildPack:    "launchpack",
		StaticDeploy: p.isStaticFramework(detection.Framework),
		Cache: &CacheConfig{
			Mounts: p.getNodeCacheMounts(pm),
		},
		Strategy: "buildpack_node",
		Environment: map[string]string{
			"NODE_ENV": "production",
		},
		Volumes: p.detectVolumes(detection),
		Metadata: &PlanMetadata{
			EstimatedBuildTimeMs: 30000,
			CacheHitRate:        0.85,
			ImageSizeEstimateMB: 15,
			OptimizationApplied: []string{"multi-stage", "layer-caching", "dependency-install-cache"},
		},
	}

	plan.InstallCmd = p.getNodeInstallCmd(pm)
	plan.BuildCmd = p.getNodeBuildCmd(detection)
	plan.StartCmd = p.getNodeStartCmd(detection)
	plan.Stages = p.generateNodeStages(detection, pm)

	return plan
}

// detectVolumes detects common persistent volume requirements
func (p *Planner) detectVolumes(detection *analyzer.DetectionResult) []Volume {
	var volumes []Volume

	// Common storage directories that should be persistent
	persistentDirs := []string{
		"data",
		"storage",
		"uploads",
		"files",
		"cache",
		"logs",
		"tmp",
		"db",
		"database",
	}

	// Check for Laravel-style storage
	if p.hasFile("storage/") {
		volumes = append(volumes, Volume{
			Type:   "persistent",
			Name:   "storage",
			Target: "/app/storage",
		})
	}

	// Check for database directories
	if p.hasFile("data/") || p.hasFile("db/") {
		volumes = append(volumes, Volume{
			Type:   "persistent",
			Name:   "data",
			Target: "/app/data",
		})
	}

	// Check for upload directories
	if p.hasFile("uploads/") {
		volumes = append(volumes, Volume{
			Type:   "persistent",
			Name:   "uploads",
			Target: "/app/uploads",
		})
	}

	// Node.js specific: check for volume requirements in metadata
	if detection.Framework == "nextjs" {
		// Next.js may need persistent storage for .next/cache
		volumes = append(volumes, Volume{
			Type:   "cache",
			Name:   "next-cache",
			Target: "/app/.next/cache",
		})
	}

	// Python specific volumes
	if detection.Language == "python" {
		// Check for SQLite or other file-based databases
		if p.hasFile(".env") {
			volumes = append(volumes, Volume{
				Type:   "persistent",
				Name:   "env",
				Target: "/app/.env",
				ReadOnly: true,
			})
		}
	}

	return volumes
}

func (p *Planner) getNodeOutput(detection *analyzer.DetectionResult) string {
	outputs := map[string]string{
		"nextjs":    ".next",
		"nuxt":      ".output",
		"gatsby":    "public",
		"astro":     "dist",
		"vite":      "dist",
		"nestjs":    "dist",
		"remix":     "build",
		"sveltekit": "build",
	}
	if out, ok := outputs[detection.Framework]; ok {
		return out
	}
	return "dist"
}

func (p *Planner) getNodeCacheMounts(pm string) []CacheMount {
	mounts := []CacheMount{
		{Type: "cache", Target: "/root/.npm"},
	}
	switch pm {
	case "pnpm":
		mounts = append(mounts, CacheMount{Type: "cache", Target: "/root/.pnpm-store"})
	case "yarn":
		mounts = append(mounts, CacheMount{Type: "cache", Target: "/root/.yarn"})
	case "bun":
		mounts = append(mounts, CacheMount{Type: "cache", Target: "/root/.bun"})
	}
	return mounts
}

func (p *Planner) getNodeInstallCmd(pm string) string {
	switch pm {
	case "pnpm":
		return "corepack enable pnpm && pnpm install --frozen-lockfile"
	case "yarn":
		return "yarn install --frozen-lockfile"
	case "bun":
		return "bun install --frozen-lockfile"
	default:
		return "npm ci"
	}
}

func (p *Planner) getNodeBuildCmd(detection *analyzer.DetectionResult) string {
	buildScripts := map[string]string{
		"nextjs":    "next build",
		"nuxt":      "nuxt build",
		"gatsby":    "gatsby build",
		"astro":     "astro build",
		"vite":      "vite build",
		"nestjs":    "nest build",
		"remix":     "remix build",
		"sveltekit": "vite build",
	}
	if script, ok := buildScripts[detection.Framework]; ok {
		return script
	}
	return "npm run build"
}

func (p *Planner) getNodeStartCmd(detection *analyzer.DetectionResult) string {
	startCmds := map[string]string{
		"nextjs":    "next start",
		"nuxt":      "node .output/server/index.mjs",
		"vite":      "node dist/index.js",
		"nestjs":    "node dist/main.js",
		"express":   "node dist/index.js",
		"remix":     "node build/server/index.js",
	}
	if cmd, ok := startCmds[detection.Framework]; ok {
		return cmd
	}
	return "node index.js"
}

func (p *Planner) generateNodeStages(detection *analyzer.DetectionResult, pm string) []Stage {
	baseImage := detection.Runtime
	if baseImage == "" {
		baseImage = "node:22-alpine"
	}

	stages := []Stage{
		{
			Name:  "deps",
			Image: baseImage,
			Steps: []Step{
				{Type: "workdir", Dest: "/app"},
				{Type: "copy", Src: "package.json", Dest: "."},
				{Type: "copy", Src: p.getLockFileName(pm), Dest: "."},
				{Type: "mount", Mount: &CacheMount{Type: "cache", Target: p.getCacheDir(pm)}},
				{Type: "run", Command: p.getNodeInstallCmd(pm)},
			},
		},
	}

	if !p.isStaticFramework(detection.Framework) {
		stages = append(stages, Stage{
			Name:  "build",
			Image: baseImage,
			Steps: []Step{
				{Type: "workdir", Dest: "/app"},
				{Type: "copy", Src: ".", Dest: "."},
				{Type: "run", Command: p.getNodeBuildCmd(detection)},
			},
			Needs: []string{"deps"},
		})
	}

	runtimeImage := p.getNodeRuntimeImage(detection)
	stages = append(stages, Stage{
		Name:       "runtime",
		Image:      runtimeImage,
		Entrypoint: p.getNodeEntrypoint(detection),
		Steps:      p.getNodeRuntimeSteps(detection),
	})

	return stages
}

func (p *Planner) getNodeRuntimeImage(detection *analyzer.DetectionResult) string {
	if p.isStaticFramework(detection.Framework) {
		return "nginx:alpine"
	}
	return "node:22-alpine"
}

func (p *Planner) getNodeRuntimeSteps(detection *analyzer.DetectionResult) []Step {
	steps := []Step{
		{Type: "workdir", Dest: "/app"},
	}

	if p.isStaticFramework(detection.Framework) {
		publishDir := p.getNodeOutput(detection)
		steps = append(steps, Step{
			Type: "copy",
			Src:  "build:" + publishDir,
			Dest: "/usr/share/nginx/html",
		})
		steps = append(steps, Step{
			Type: "copy",
			Src:  "build:nginx.conf",
			Dest: "/etc/nginx/conf.d/default.conf",
		})
	} else {
		steps = append(steps, Step{
			Type: "copy",
			Src:  "build:.",
			Dest: "/app",
		})
	}

	return steps
}

func (p *Planner) getNodeEntrypoint(detection *analyzer.DetectionResult) []string {
	entrypoints := map[string][]string{
		"nextjs":    {"node", "server.js"},
		"nuxt":      {"node", ".output/server/index.mjs"},
		"gatsby":    {"/docker-entrypoint.sh", "nginx", "-g", "daemon off;"},
		"astro":     {"/docker-entrypoint.sh", "nginx", "-g", "daemon off;"},
		"vite":      {"node", "dist/index.js"},
		"nestjs":    {"node", "dist/main.js"},
		"express":   {"node", "dist/index.js"},
		"sveltekit": {"node", "build/index.js"},
	}
	if ep, ok := entrypoints[detection.Framework]; ok {
		return ep
	}
	return []string{"node", "index.js"}
}

func (p *Planner) getLockFileName(pm string) string {
	lockFiles := map[string]string{
		"pnpm": "pnpm-lock.yaml",
		"yarn": "yarn.lock",
		"bun":  "bun.lockb",
		"npm":  "package-lock.json",
	}
	if lock, ok := lockFiles[pm]; ok {
		return lock
	}
	return "package-lock.json"
}

func (p *Planner) getCacheDir(pm string) string {
	cacheDirs := map[string]string{
		"pnpm": "/root/.pnpm-store",
		"yarn": "/root/.yarn",
		"bun":  "/root/.bun",
		"npm":  "/root/.npm",
	}
	if dir, ok := cacheDirs[pm]; ok {
		return dir
	}
	return "/root/.npm"
}

func (p *Planner) isStaticFramework(framework string) bool {
	staticFrameworks := map[string]bool{
		"gatsby": true,
		"astro":  true,
	}
	return staticFrameworks[framework]
}

// ============ PYTHON ============

func (p *Planner) planPython(detection *analyzer.DetectionResult) *BuildPlan {
	plan := &BuildPlan{
		Output:       "./app",
		BuildPack:    "launchpack",
		StaticDeploy: false,
		Cache: &CacheConfig{
			Mounts: []CacheMount{
				{Type: "cache", Target: "/root/.cache/pip"},
				{Type: "cache", Target: "/root/.local"},
			},
		},
		Strategy: "buildpack_python",
		Environment: map[string]string{
			"PYTHONUNBUFFERED": "1",
		},
		Metadata: &PlanMetadata{
			EstimatedBuildTimeMs: 45000,
			CacheHitRate:        0.80,
			ImageSizeEstimateMB: 10,
			OptimizationApplied: []string{"multi-stage", "layer-caching", "pip-cache"},
		},
	}

	plan.InstallCmd = p.getPythonInstallCmd()
	plan.BuildCmd = ""
	plan.StartCmd = p.getPythonStartCmd(detection)
	plan.Stages = p.generatePythonStages(detection)

	return plan
}

func (p *Planner) getPythonInstallCmd() string {
	if p.hasFile("pyproject.toml") {
		return "pip install uv && uv pip install --system -r requirements.txt"
	}
	if p.hasFile("requirements.txt") {
		return "pip install --no-cache-dir -r requirements.txt"
	}
	if p.hasFile("Pipfile") {
		return "pip install pipenv && pipenv install --system --deploy"
	}
	return "echo 'No dependency file found'"
}

func (p *Planner) getPythonStartCmd(detection *analyzer.DetectionResult) string {
	startCmds := map[string][]string{
		"fastapi": {"uvicorn", "main:app", "--host", "0.0.0.0", "--port", "{{PORT}}"},
		"django":  {"gunicorn", "--bind", ":{{PORT}}", "config.wsgi:application"},
		"flask":   {"flask", "run", "--host=0.0.0.0", "--port={{PORT}}"},
	}
	if cmd, ok := startCmds[detection.Framework]; ok {
		return strings.Join(cmd, " ")
	}
	return "python main.py"
}

func (p *Planner) generatePythonStages(detection *analyzer.DetectionResult) []Stage {
	baseImage := "python:3.12-slim"

	stages := []Stage{
		{
			Name:  "deps",
			Image: baseImage,
			Steps: []Step{
				{Type: "workdir", Dest: "/app"},
				{Type: "mount", Mount: &CacheMount{Type: "cache", Target: "/root/.cache/pip"}},
				{Type: "mount", Mount: &CacheMount{Type: "cache", Target: "/root/.local"}},
			},
		},
	}

	if p.hasFile("requirements.txt") {
		stages[0].Steps = append(stages[0].Steps,
			Step{Type: "copy", Src: "requirements.txt", Dest: "."},
		)
	}

	stages = append(stages, Stage{
		Name:  "build",
		Image: baseImage,
		Steps: []Step{
			{Type: "workdir", Dest: "/app"},
			{Type: "copy", Src: ".", Dest: "."},
		},
		Needs: []string{"deps"},
	})

	stages = append(stages, Stage{
		Name:    "runtime",
		Image:   "python:3.12-alpine",
		Command: p.getPythonStartCmd(detection),
		Steps: []Step{
			{Type: "workdir", Dest: "/app"},
			{Type: "copy", Src: "build:.", Dest: "/app"},
			{Type: "run", Command: "adduser -D -u 1001 -s /bin/sh appuser && chown -R appuser /app"},
		},
	})

	return stages
}

// ============ GO ============

func (p *Planner) planGo(detection *analyzer.DetectionResult) *BuildPlan {
	plan := &BuildPlan{
		Output:       "./bin",
		BuildPack:    "launchpack",
		StaticDeploy: false,
		Cache: &CacheConfig{
			Mounts: []CacheMount{
				{Type: "cache", Target: "/go/pkg/mod"},
				{Type: "cache", Target: "/root/.cache/go-build"},
			},
		},
		Strategy: "buildpack_go",
		Environment: map[string]string{
			"CGO_ENABLED": "0",
		},
		Metadata: &PlanMetadata{
			EstimatedBuildTimeMs: 60000,
			CacheHitRate:        0.90,
			ImageSizeEstimateMB: 8,
			OptimizationApplied: []string{"multi-stage", "go-build-cache", "static-binary"},
		},
	}

	plan.InstallCmd = "go mod download"
	plan.BuildCmd = "CGO_ENABLED=0 go build -ldflags='-s -w' -o main ."
	plan.StartCmd = "/app/main"
	plan.Stages = p.generateGoStages()

	return plan
}

func (p *Planner) generateGoStages() []Stage {
	return []Stage{
		{
			Name:  "builder",
			Image: "golang:1.22-alpine",
			Steps: []Step{
				{Type: "workdir", Dest: "/build"},
				{Type: "copy", Src: "go.mod", Dest: "."},
				{Type: "copy", Src: "go.sum", Dest: "."},
				{Type: "mount", Mount: &CacheMount{Type: "cache", Target: "/go/pkg/mod"}},
				{Type: "mount", Mount: &CacheMount{Type: "cache", Target: "/root/.cache/go-build"}},
				{Type: "run", Command: "go mod download"},
				{Type: "copy", Src: ".", Dest: "."},
				{Type: "run", Command: "CGO_ENABLED=0 go build -ldflags='-s -w' -o main ."},
			},
		},
		{
			Name:  "runtime",
			Image: "alpine:3.19",
			Steps: []Step{
				{Type: "workdir", Dest: "/app"},
				{Type: "copy", Src: "builder:build/main", Dest: "/app/main"},
				{Type: "run", Command: "adduser -D -u 1001 -s /bin/sh appuser && chown -R appuser /app"},
			},
			Command: "/app/main",
		},
	}
}

// ============ RUST ============

func (p *Planner) planRust(detection *analyzer.DetectionResult) *BuildPlan {
	plan := &BuildPlan{
		Output:       "./target/release",
		BuildPack:    "launchpack",
		StaticDeploy: false,
		Cache: &CacheConfig{
			Mounts: []CacheMount{
				{Type: "cache", Target: "/usr/local/cargo/registry"},
				{Type: "cache", Target: "/app/target"},
			},
		},
		Strategy: "buildpack_rust",
		Metadata: &PlanMetadata{
			EstimatedBuildTimeMs: 120000,
			CacheHitRate:        0.85,
			ImageSizeEstimateMB: 5,
			OptimizationApplied: []string{"multi-stage", "cargo-cache", "static-binary"},
		},
	}

	plan.BuildCmd = "cargo build --release"
	plan.StartCmd = "/app/app"
	plan.Stages = p.generateRustStages()

	return plan
}

func (p *Planner) generateRustStages() []Stage {
	return []Stage{
		{
			Name:  "builder",
			Image: "rust:1.77-alpine",
			Steps: []Step{
				{Type: "workdir", Dest: "/app"},
				{Type: "copy", Src: "Cargo.toml", Dest: "."},
				{Type: "copy", Src: "Cargo.lock", Dest: "."},
				{Type: "mount", Mount: &CacheMount{Type: "cache", Target: "/usr/local/cargo/registry"}},
				{Type: "mount", Mount: &CacheMount{Type: "cache", Target: "/app/target"}},
				{Type: "run", Command: "cargo build --release"},
			},
		},
		{
			Name:  "runtime",
			Image: "scratch",
			Steps: []Step{
				{Type: "copy", Src: "builder:app/target/release/app", Dest: "/app"},
			},
			Command: "/app",
		},
	}
}

// ============ PHP ============

func (p *Planner) planPHP(detection *analyzer.DetectionResult) *BuildPlan {
	plan := &BuildPlan{
		Output:       "./public",
		BuildPack:    "launchpack",
		StaticDeploy: false,
		Cache: &CacheConfig{
			Mounts: []CacheMount{
				{Type: "cache", Target: "/root/.composer"},
			},
		},
		Strategy: "buildpack_php",
		Metadata: &PlanMetadata{
			EstimatedBuildTimeMs: 30000,
			CacheHitRate:        0.80,
			ImageSizeEstimateMB: 8,
			OptimizationApplied: []string{"multi-stage", "composer-cache"},
		},
	}

	plan.InstallCmd = "composer install --no-dev --optimize-autoloader"
	plan.StartCmd = p.getPHPStartCmd(detection)
	plan.Stages = p.generatePHPStages(detection)

	return plan
}

func (p *Planner) getPHPStartCmd(detection *analyzer.DetectionResult) string {
	if detection.Framework == "laravel" {
		return "php artisan serve --host=0.0.0.0 --port={{PORT}}"
	}
	return "php -S 0.0.0.0:{{PORT}}"
}

func (p *Planner) generatePHPStages(detection *analyzer.DetectionResult) []Stage {
	return []Stage{
		{
			Name:  "deps",
			Image: "composer:2",
			Steps: []Step{
				{Type: "workdir", Dest: "/app"},
				{Type: "copy", Src: "composer.json", Dest: "."},
				{Type: "copy", Src: "composer.lock", Dest: "."},
				{Type: "mount", Mount: &CacheMount{Type: "cache", Target: "/root/.composer"}},
				{Type: "run", Command: "composer install --no-dev --optimize-autoloader"},
			},
		},
		{
			Name:  "runtime",
			Image: "php:8.3-fpm-alpine",
			Steps: []Step{
				{Type: "workdir", Dest: "/var/www/html"},
				{Type: "copy", Src: "build:.", Dest: "/var/www/html"},
				{Type: "run", Command: "adduser -D -u 1001 -s /bin/sh www-data && chown -R www-data /var/www/html"},
			},
			Command: p.getPHPStartCmd(detection),
		},
	}
}

// ============ GENERIC ============

func (p *Planner) planDockerfile(detection *analyzer.DetectionResult) *BuildPlan {
	return &BuildPlan{
		Strategy: "dockerfile_user_supplied",
		Metadata: &PlanMetadata{
			OptimizationApplied: []string{"user-supplied"},
		},
	}
}

func (p *Planner) planGeneric(detection *analyzer.DetectionResult) *BuildPlan {
	return &BuildPlan{
		Output:       ".",
		BuildPack:    "launchpack",
		StaticDeploy: false,
		Strategy:     "buildpack_generic",
		Metadata: &PlanMetadata{
			EstimatedBuildTimeMs: 30000,
			CacheHitRate:        0.50,
			ImageSizeEstimateMB: 20,
			OptimizationApplied: []string{"generic-build"},
		},
		Stages: []Stage{
			{
				Name:  "build",
				Image: "alpine:latest",
				Steps: []Step{
					{Type: "workdir", Dest: "/app"},
					{Type: "copy", Src: ".", Dest: "/app"},
				},
			},
		},
	}
}

// Helper functions

func (p *Planner) hasFile(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}
