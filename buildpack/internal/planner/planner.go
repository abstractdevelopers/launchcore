package planner

import (
	"os"

	"github.com/abstractdevelopers/launchcore/buildpack/internal/analyzer"
)

// BuildPlan represents the complete build execution plan
type BuildPlan struct {
	Stages      []Stage        `json:"stages"`
	Output      string         `json:"output"`
	Runtime     string         `json:"runtime"`
	Entrypoint  []string       `json:"entrypoint"`
	EnvVars     []string       `json:"envVars,omitempty"`
	CacheConfig CacheConfig    `json:"cacheConfig"`
	Strategy    string         `json:"strategy"`
	Metadata    PlanMetadata   `json:"metadata"`
}

// Stage represents a single build stage
type Stage struct {
	Name       string   `json:"name"`
	Image      string   `json:"image"`
	Steps      []Step   `json:"steps"`
	Cache      []string `json:"cache,omitempty"`
	Needs      []string `json:"needs,omitempty"`
}

// Step represents a single build step
type Step struct {
	Type    string   `json:"type"` // copy, run, workdir, env, etc.
	Src     string   `json:"src,omitempty"`
	Dest    string   `json:"dest,omitempty"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

// CacheConfig defines caching strategy
type CacheConfig struct {
	Keys   map[string]string `json:"keys"`   // cache key -> mount path
	Mounts []CacheMount      `json:"mounts"`
}

// CacheMount represents a persistent cache mount
type CacheMount struct {
	Type   string   `json:"type"` // volume, bind
	Source string   `json:"source"`
	Target string   `json:"target"`
}

// PlanMetadata contains additional plan information
type PlanMetadata struct {
	EstimatedBuildTimeMs int64   `json:"estimatedBuildTimeMs"`
	CacheHitRate         float64 `json:"cacheHitRate"`
	ImageSizeEstimateMB  int     `json:"imageSizeEstimateMB"`
	OptimizationApplied  []string `json:"optimizationApplied"`
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
		Strategy: p.selectStrategy(detection),
		Metadata: PlanMetadata{
			OptimizationApplied: []string{},
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

	return plan
}

func (p *Planner) selectStrategy(detection *analyzer.DetectionResult) string {
	if detection.Framework == "dockerfile_user_supplied" {
		return "dockerfile_user_supplied"
	}
	return "buildpack_" + detection.Language
}

func (p *Planner) planNode(detection *analyzer.DetectionResult) *BuildPlan {
	pm := detection.PackageManager
	if pm == "" {
		pm = "npm"
	}

	plan := &BuildPlan{
		Output:  "./dist",
		Runtime: detection.Runtime,
		CacheConfig: CacheConfig{
			Keys: map[string]string{
				pm: "/root/.npm",
			},
			Mounts: []CacheMount{
				{Type: "cache", Target: "/root/.npm"},
			},
		},
		Strategy: "buildpack_node",
		Metadata: PlanMetadata{
			EstimatedBuildTimeMs: 30000,
			OptimizationApplied: []string{"multi-stage", "layer-caching", "dependency-install-cache"},
		},
	}

	// Multi-stage plan: deps -> build -> runtime
	plan.Stages = []Stage{
		{
			Name:  "deps",
			Image: detection.Runtime,
			Steps: p.getNodeDepsSteps(pm),
			Cache: []string{"/root/.npm"},
		},
	}

	// Add build stage if needed
	if detection.Framework != "" && !p.isStaticFramework(detection.Framework) {
		buildStep := p.getNodeBuildStep(detection)
		if buildStep != nil {
			plan.Stages = append(plan.Stages, Stage{
				Name:  "build",
				Image: detection.Runtime,
				Steps: []Step{*buildStep},
				Needs: []string{"deps"},
			})
		}
	}

	// Runtime stage (final)
	plan.Stages = append(plan.Stages, Stage{
		Name:       "runtime",
		Image:      p.getNodeRuntimeImage(detection),
		Steps:      p.getNodeRuntimeSteps(detection),
		Entrypoint: p.getNodeEntrypoint(detection),
	})

	return plan
}

func (p *Planner) getNodeDepsSteps(pm string) []Step {
	lockFile := p.getLockFile(pm)
	
	steps := []Step{
		{Type: "copy", Src: lockFile, Dest: "."},
		{Type: "copy", Src: "package.json", Dest: "."},
	}

	switch pm {
	case "pnpm":
		steps = append(steps, Step{Type: "run", Command: "corepack enable pnpm && pnpm install --frozen-lockfile"})
	case "yarn":
		steps = append(steps, Step{Type: "run", Command: "yarn install --frozen-lockfile"})
	case "bun":
		steps = append(steps, Step{Type: "run", Command: "bun install --frozen-lockfile"})
	default:
		steps = append(steps, Step{Type: "run", Command: "npm ci"})
	}

	return steps
}

func (p *Planner) getNodeBuildStep(detection *analyzer.DetectionResult) *Step {
	buildScripts := map[string]string{
		"nextjs":  "next build",
		"nuxt":    "nuxt build",
		"gatsby":  "gatsby build",
		"astro":   "astro build",
		"vite":    "vite build",
		"nestjs":  "nest build",
		"remix":   "remix build",
		"sveltekit": "vite build",
	}

	if script, ok := buildScripts[detection.Framework]; ok {
		return &Step{
			Type:    "run",
			Command: script,
		}
	}

	// Generic: check package.json for build script
	return &Step{
		Type:    "run",
		Command: "npm run build",
	}
}

func (p *Planner) getNodeRuntimeSteps(detection *analyzer.DetectionResult) []Step {
	steps := []Step{
		{Type: "copy", Src: ".", Dest: "/app"},
	}

	if detection.Framework == "nextjs" {
		steps = append(steps, Step{
			Type:    "env",
			Command: "NODE_ENV=production",
		})
	}

	return steps
}

func (p *Planner) getNodeEntrypoint(detection *analyzer.DetectionResult) []string {
	entrypoints := map[string][]string{
		"nextjs":    {"node", "server.js"},
		"nuxt":      {"node", ".output/server/index.mjs"},
		"gatsby":    {"node", "public/index.js"},
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

func (p *Planner) getNodeRuntimeImage(detection *analyzer.DetectionResult) string {
	runtimeImages := map[string]string{
		"nextjs":  "node:22-alpine",
		"nuxt":    "node:22-alpine",
		"vite":    "node:22-alpine",
		"nestjs":  "node:22-alpine",
		"express": "node:22-alpine",
		"gatsby":  "nginx:alpine", // static
		"astro":   "nginx:alpine", // static
	}

	if img, ok := runtimeImages[detection.Framework]; ok {
		return img
	}
	return "node:22-alpine"
}

func (p *Planner) getLockFile(pm string) string {
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

func (p *Planner) isStaticFramework(framework string) bool {
	staticFrameworks := map[string]bool{
		"gatsby": true,
		"astro":  true,
	}
	return staticFrameworks[framework]
}

func (p *Planner) planPython(detection *analyzer.DetectionResult) *BuildPlan {
	pythonVersion := "3.12"

	plan := &BuildPlan{
		Output:  "./app",
		Runtime: "python:" + pythonVersion + "-alpine",
		CacheConfig: CacheConfig{
			Keys: map[string]string{
				"pip": "/root/.cache/pip",
			},
			Mounts: []CacheMount{
				{Type: "cache", Target: "/root/.cache/pip"},
				{Type: "cache", Target: "/root/.local"},
			},
		},
		Strategy: "buildpack_python",
		Metadata: PlanMetadata{
			EstimatedBuildTimeMs: 45000,
			OptimizationApplied: []string{"multi-stage", "layer-caching", "pip-cache"},
		},
	}

	plan.Stages = []Stage{
		{
			Name:  "deps",
			Image: "python:" + pythonVersion + "-slim",
			Steps: p.getPythonDepsSteps(detection),
			Cache: []string{"/root/.cache/pip", "/root/.local"},
		},
		{
			Name:  "build",
			Image: "python:" + pythonVersion + "-slim",
			Steps: p.getPythonBuildSteps(detection),
			Needs: []string{"deps"},
		},
		{
			Name:       "runtime",
			Image:      "python:" + pythonVersion + "-alpine",
			Steps:      []Step{{Type: "copy", Src: ".", Dest: "/app"}},
			Entrypoint: p.getPythonEntrypoint(detection),
		},
	}

	return plan
}

func (p *Planner) getPythonDepsSteps(detection *analyzer.DetectionResult) []Step {
	steps := []Step{
		{Type: "workdir", Dest: "/app"},
	}

	if _, err := os.Stat("pyproject.toml"); err == nil {
		steps = append(steps, Step{Type: "copy", Src: "pyproject.toml", Dest: "."})
		steps = append(steps, Step{Type: "run", Command: "pip install uv && uv pip install --system -r requirements.txt"})
	} else if _, err := os.Stat("requirements.txt"); err == nil {
		steps = append(steps, Step{Type: "copy", Src: "requirements.txt", Dest: "."})
		steps = append(steps, Step{Type: "run", Command: "pip install --no-cache-dir -r requirements.txt"})
	} else if _, err := os.Stat("Pipfile"); err == nil {
		steps = append(steps, Step{Type: "copy", Src: "Pipfile*", Dest: "."})
		steps = append(steps, Step{Type: "run", Command: "pip install pipenv && pipenv install --system --deploy"})
	}

	return steps
}

func (p *Planner) getPythonBuildSteps(detection *analyzer.DetectionResult) []Step {
	return []Step{
		{Type: "workdir", Dest: "/app"},
		{Type: "copy", Src: ".", Dest: "/app"},
	}
}

func (p *Planner) getPythonEntrypoint(detection *analyzer.DetectionResult) []string {
	entrypoints := map[string][]string{
		"fastapi": {"uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"},
		"django":  {"gunicorn", "--bind", ":8080", "config.wsgi:application"},
		"flask":   {"flask", "run", "--host=0.0.0.0"},
	}

	if ep, ok := entrypoints[detection.Framework]; ok {
		return ep
	}

	return []string{"python", "main.py"}
}

func (p *Planner) planGo(detection *analyzer.DetectionResult) *BuildPlan {
	plan := &BuildPlan{
		Output:  "./bin",
		Runtime: "alpine",
		CacheConfig: CacheConfig{
			Keys: map[string]string{
				"go": "/go/pkg/mod",
			},
			Mounts: []CacheMount{
				{Type: "cache", Target: "/go/pkg/mod"},
				{Type: "cache", Target: "/root/.cache/go-build"},
			},
		},
		Strategy: "buildpack_go",
		Metadata: PlanMetadata{
			EstimatedBuildTimeMs: 60000,
			OptimizationApplied: []string{"multi-stage", "go-build-cache"},
		},
	}

	plan.Stages = []Stage{
		{
			Name:  "builder",
			Image: "golang:1.22-alpine",
			Steps: []Step{
				{Type: "workdir", Dest: "/build"},
				{Type: "copy", Src: "go.mod", Dest: "."},
				{Type: "copy", Src: "go.sum", Dest: "."},
				{Type: "run", Command: "go mod download"},
				{Type: "copy", Src: ".", Dest: "."},
				{Type: "run", Command: "CGO_ENABLED=0 go build -ldflags='-s -w' -o main ."},
			},
			Cache: []string{"/go/pkg/mod", "/root/.cache/go-build"},
		},
		{
			Name:       "runtime",
			Image:      "alpine:3.19",
			Steps:      []Step{{Type: "copy", Src: "bin/main", Dest: "/app/main"}},
			Entrypoint: []string{"/app/main"},
		},
	}

	return plan
}

func (p *Planner) planRust(detection *analyzer.DetectionResult) *BuildPlan {
	plan := &BuildPlan{
		Output:  "./target/release",
		Runtime: "scratch",
		CacheConfig: CacheConfig{
			Keys: map[string]string{
				"cargo": "/usr/local/cargo/registry",
			},
			Mounts: []CacheMount{
				{Type: "cache", Target: "/usr/local/cargo/registry"},
				{Type: "cache", Target: "/app/target"},
			},
		},
		Strategy: "buildpack_rust",
		Metadata: PlanMetadata{
			EstimatedBuildTimeMs: 120000,
			OptimizationApplied: []string{"multi-stage", "cargo-cache"},
		},
	}

	plan.Stages = []Stage{
		{
			Name:  "builder",
			Image: "rust:1.77-alpine",
			Steps: []Step{
				{Type: "workdir", Dest: "/app"},
				{Type: "copy", Src: "Cargo.toml", Dest: "."},
				{Type: "copy", Src: "Cargo.lock", Dest: "."},
				{Type: "run", Command: "cargo build --release"},
			},
			Cache: []string{"/usr/local/cargo/registry", "/app/target"},
		},
		{
			Name:       "runtime",
			Image:      "scratch",
			Steps:      []Step{{Type: "copy", Src: "target/release/app", Dest: "/app"}},
			Entrypoint: []string{"/app"},
		},
	}

	return plan
}

func (p *Planner) planPHP(detection *analyzer.DetectionResult) *BuildPlan {
	runtime := "php:8.3-fpm-alpine"
	if detection.Framework == "laravel" {
		runtime = "php:8.3-fpm-alpine"
	}

	plan := &BuildPlan{
		Output:  "./public",
		Runtime: runtime,
		CacheConfig: CacheConfig{
			Keys: map[string]string{
				"composer": "/root/.composer",
			},
			Mounts: []CacheMount{
				{Type: "cache", Target: "/root/.composer"},
			},
		},
		Strategy: "buildpack_php",
		Metadata: PlanMetadata{
			EstimatedBuildTimeMs: 30000,
			OptimizationApplied: []string{"multi-stage", "composer-cache"},
		},
	}

	plan.Stages = []Stage{
		{
			Name:  "deps",
			Image: "composer:2",
			Steps: []Step{
				{Type: "copy", Src: "composer.json", Dest: "/app"},
				{Type: "copy", Src: "composer.lock", Dest: "/app"},
				{Type: "run", Command: "composer install --no-dev --optimize-autoloader"},
			},
			Cache: []string{"/root/.composer"},
		},
		{
			Name:  "build",
			Image: runtime,
			Steps: []Step{
				{Type: "copy", Src: ".", Dest: "/var/www/html"},
			},
			Needs: []string{"deps"},
		},
	}

	return plan
}

func (p *Planner) planDockerfile(detection *analyzer.DetectionResult) *BuildPlan {
	return &BuildPlan{
		Strategy: "dockerfile_user_supplied",
		Metadata: PlanMetadata{
			EstimatedBuildTimeMs: 0,
			OptimizationApplied: []string{"user-supplied"},
		},
	}
}

func (p *Planner) planGeneric(detection *analyzer.DetectionResult) *BuildPlan {
	return &BuildPlan{
		Output:  ".",
		Runtime: "alpine:latest",
		Strategy: "buildpack_generic",
		Metadata: PlanMetadata{
			EstimatedBuildTimeMs: 30000,
			OptimizationApplied: []string{"generic-build"},
		},
		Stages: []Stage{
			{
				Name:  "build",
				Image: "alpine:latest",
				Steps: []Step{
					{Type: "copy", Src: ".", Dest: "/app"},
					{Type: "run", Command: "echo 'No build steps detected'"},
				},
			},
		},
	}
}
