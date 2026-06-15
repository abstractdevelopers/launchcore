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

	fwConfig := getFrameworkConfig(detection.Framework)

	plan := &BuildPlan{
		Output:        fwConfig.Output,
		BuildPack:    "launchpack",
		StaticDeploy: fwConfig.IsStatic,
		Cache: &CacheConfig{
			Mounts: p.getNodeCacheMounts(pm),
		},
		Strategy: "buildpack_node",
		Environment: fwConfig.EnvVars,
		Ports:     fwConfig.Ports,
		Volumes:   p.detectVolumes(detection),
		Metadata: &PlanMetadata{
			EstimatedBuildTimeMs: fwConfig.BuildTimeMs,
			CacheHitRate:        0.85,
			ImageSizeEstimateMB: fwConfig.ImageSizeMB,
			OptimizationApplied: fwConfig.Optimizations,
		},
	}

	plan.InstallCmd = p.getNodeInstallCmd(pm)
	plan.BuildCmd = fwConfig.BuildCmd
	plan.StartCmd = fwConfig.StartCmd
	plan.Stages = p.generateNodeStages(detection, pm, fwConfig)

	return plan
}

// FrameworkConfig holds all configuration for a specific framework
type FrameworkConfig struct {
	Name            string
	Output         string
	BuildCmd       string
	StartCmd       string
	RuntimeImage   string
	Ports          []int
	IsStatic      bool
	IsSPA         bool
	ImageSizeMB   int
	BuildTimeMs   int64
	Optimizations []string
	EnvVars       map[string]string
}

// getFrameworkConfig returns configuration for a specific framework
func getFrameworkConfig(framework string) FrameworkConfig {
	configs := map[string]FrameworkConfig{
		// Meta-frameworks
		"nextjs": {
			Name: "Next.js", Output: ".next", BuildCmd: "next build",
			StartCmd: "next start", RuntimeImage: "node:22-alpine",
			Ports: []int{3000}, ImageSizeMB: 20, BuildTimeMs: 60000,
			Optimizations: []string{"multi-stage", "layer-caching", "standalone-output"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},
		"nuxt": {
			Name: "Nuxt.js", Output: ".output", BuildCmd: "nuxt build",
			StartCmd: "node .output/server/index.mjs", RuntimeImage: "node:22-alpine",
			Ports: []int{3000}, ImageSizeMB: 18, BuildTimeMs: 55000,
			Optimizations: []string{"multi-stage", "layer-caching", "server-output"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},
		"sveltekit": {
			Name: "SvelteKit", Output: "build", BuildCmd: "vite build",
			StartCmd: "node build/index.js", RuntimeImage: "node:22-alpine",
			Ports: []int{3000}, ImageSizeMB: 12, BuildTimeMs: 45000,
			Optimizations: []string{"multi-stage", "layer-caching", "prerender"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},
		"remix": {
			Name: "Remix", Output: "build", BuildCmd: "remix build",
			StartCmd: "node build/server/index.js", RuntimeImage: "node:22-alpine",
			Ports: []int{3000}, ImageSizeMB: 15, BuildTimeMs: 50000,
			Optimizations: []string{"multi-stage", "layer-caching"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},
		"redwoodjs": {
			Name: "RedwoodJS", Output: "api/dist", BuildCmd: "rw build",
			StartCmd: "node api/dist/index.js", RuntimeImage: "node:22-alpine",
			Ports: []int{8911}, ImageSizeMB: 25, BuildTimeMs: 90000,
			Optimizations: []string{"multi-stage", "api-web-split"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},
		"blitzjs": {
			Name: "Blitz.js", Output: ".next", BuildCmd: "blitz build",
			StartCmd: "next start", RuntimeImage: "node:22-alpine",
			Ports: []int{3000}, ImageSizeMB: 22, BuildTimeMs: 70000,
			Optimizations: []string{"multi-stage", "layer-caching"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},
		"keystonejs": {
			Name: "Keystone.js", Output: "dist", BuildCmd: "keystone build",
			StartCmd: "node dist/keystone.js", RuntimeImage: "node:22-alpine",
			Ports: []int{8000}, ImageSizeMB: 18, BuildTimeMs: 60000,
			Optimizations: []string{"multi-stage"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},
		"strapi": {
			Name: "Strapi", Output: "dist", BuildCmd: "strapi build",
			StartCmd: "node dist/bin/main.js", RuntimeImage: "node:22-alpine",
			Ports: []int{1337}, ImageSizeMB: 25, BuildTimeMs: 80000,
			Optimizations: []string{"multi-stage"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},

		// Static Site Generators
		"gatsby": {
			Name: "Gatsby", Output: "public", BuildCmd: "gatsby build",
			StartCmd: "", RuntimeImage: "nginx:alpine", IsStatic: true,
			Ports: []int{80, 443}, ImageSizeMB: 8, BuildTimeMs: 120000,
			Optimizations: []string{"static-output", "nginx-minimal"},
			EnvVars: map[string]string{},
		},
		"astro": {
			Name: "Astro", Output: "dist", BuildCmd: "astro build",
			StartCmd: "", RuntimeImage: "nginx:alpine", IsStatic: true,
			Ports: []int{80, 443}, ImageSizeMB: 5, BuildTimeMs: 30000,
			Optimizations: []string{"static-output", "nginx-minimal", "zero-js"},
			EnvVars: map[string]string{},
		},
		"eleventy": {
			Name: "Eleventy", Output: "_site", BuildCmd: "eleventy",
			StartCmd: "", RuntimeImage: "nginx:alpine", IsStatic: true,
			Ports: []int{80, 443}, ImageSizeMB: 4, BuildTimeMs: 15000,
			Optimizations: []string{"static-output", "minimal"},
			EnvVars: map[string]string{},
		},
		"hugo": {
			Name: "Hugo", Output: "public", BuildCmd: "hugo",
			StartCmd: "", RuntimeImage: "nginx:alpine", IsStatic: true,
			Ports: []int{80, 443}, ImageSizeMB: 3, BuildTimeMs: 5000,
			Optimizations: []string{"static-output", "golang-binary"},
			EnvVars: map[string]string{},
		},
		"docusaurus": {
			Name: "Docusaurus", Output: "build", BuildCmd: "docusaurus build",
			StartCmd: "", RuntimeImage: "nginx:alpine", IsStatic: true,
			Ports: []int{3000}, ImageSizeMB: 12, BuildTimeMs: 60000,
			Optimizations: []string{"static-output", "react-static"},
			EnvVars: map[string]string{},
		},
		"vitepress": {
			Name: "VitePress", Output: ".vitepress/dist", BuildCmd: "vitepress build",
			StartCmd: "", RuntimeImage: "nginx:alpine", IsStatic: true,
			Ports: []int{80, 443}, ImageSizeMB: 5, BuildTimeMs: 20000,
			Optimizations: []string{"static-output", "vue-static"},
			EnvVars: map[string]string{},
		},
		"docsify": {
			Name: "Docsify", Output: "docs", BuildCmd: "",
			StartCmd: "docsify serve docs", RuntimeImage: "node:22-alpine", IsStatic: true,
			Ports: []int{3000}, ImageSizeMB: 6, BuildTimeMs: 5000,
			Optimizations: []string{"static-output"},
			EnvVars: map[string]string{},
		},

		// Build Tools
		"vite": {
			Name: "Vite", Output: "dist", BuildCmd: "vite build",
			StartCmd: "vite preview", RuntimeImage: "node:22-alpine",
			Ports: []int{3000, 5173}, ImageSizeMB: 10, BuildTimeMs: 30000,
			Optimizations: []string{"multi-stage", "esbuild"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},
		"webpack": {
			Name: "Webpack", Output: "dist", BuildCmd: "webpack --mode production",
			StartCmd: "node dist/main.js", RuntimeImage: "node:22-alpine",
			Ports: []int{8080}, ImageSizeMB: 15, BuildTimeMs: 90000,
			Optimizations: []string{"multi-stage", "tree-shaking"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},
		"rollup": {
			Name: "Rollup", Output: "dist", BuildCmd: "rollup -c",
			StartCmd: "node dist/bundle.js", RuntimeImage: "node:22-alpine",
			Ports: []int{3000}, ImageSizeMB: 8, BuildTimeMs: 40000,
			Optimizations: []string{"multi-stage", "tree-shaking"},
			EnvVars: map[string]string{},
		},
		"parcel": {
			Name: "Parcel", Output: "dist", BuildCmd: "parcel build",
			StartCmd: "node dist/index.js", RuntimeImage: "node:22-alpine",
			Ports: []int{3000}, ImageSizeMB: 12, BuildTimeMs: 60000,
			Optimizations: []string{"multi-stage"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},
		"typescript": {
			Name: "TypeScript", Output: "dist", BuildCmd: "tsc",
			StartCmd: "node dist/index.js", RuntimeImage: "node:22-alpine",
			Ports: []int{3000}, ImageSizeMB: 8, BuildTimeMs: 30000,
			Optimizations: []string{"multi-stage", "type-check"},
			EnvVars: map[string]string{},
		},

		// Full-stack Frameworks
		"nestjs": {
			Name: "NestJS", Output: "dist", BuildCmd: "nest build",
			StartCmd: "node dist/main.js", RuntimeImage: "node:22-alpine",
			Ports: []int{3000}, ImageSizeMB: 15, BuildTimeMs: 50000,
			Optimizations: []string{"multi-stage", "reflect-metadata"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},
		"angular": {
			Name: "Angular", Output: "dist", BuildCmd: "ng build",
			StartCmd: "node dist/server/server.mjs", RuntimeImage: "node:22-alpine",
			Ports: []int{4000}, ImageSizeMB: 20, BuildTimeMs: 120000,
			Optimizations: []string{"multi-stage", "ivy-compiler"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},
		"nx": {
			Name: "NX", Output: "dist/apps", BuildCmd: "nx build",
			StartCmd: "node dist/apps/*/server/main.js", RuntimeImage: "node:22-alpine",
			Ports: []int{3000}, ImageSizeMB: 30, BuildTimeMs: 180000,
			Optimizations: []string{"multi-stage", "affected-builds"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},
		"turborepo": {
			Name: "Turborepo", Output: "dist", BuildCmd: "turbo build",
			StartCmd: "node dist/index.js", RuntimeImage: "node:22-alpine",
			Ports: []int{3000}, ImageSizeMB: 25, BuildTimeMs: 150000,
			Optimizations: []string{"multi-stage", "remote-cache"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},
		"lerna": {
			Name: "Lerna", Output: "packages/*/dist", BuildCmd: "lerna run build",
			StartCmd: "node packages/*/dist/index.js", RuntimeImage: "node:22-alpine",
			Ports: []int{3000}, ImageSizeMB: 30, BuildTimeMs: 200000,
			Optimizations: []string{"multi-stage", "monorepo"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},

		// SPAs
		"react": {
			Name: "React", Output: "build", BuildCmd: "react-scripts build",
			StartCmd: "serve -s build -l 3000", RuntimeImage: "node:22-alpine", IsSPA: true,
			Ports: []int{3000}, ImageSizeMB: 12, BuildTimeMs: 120000,
			Optimizations: []string{"static-output", "code-splitting"},
			EnvVars: map[string]string{},
		},
		"vue": {
			Name: "Vue.js", Output: "dist", BuildCmd: "vue-cli-service build",
			StartCmd: "serve -s dist -l 80", RuntimeImage: "nginx:alpine", IsSPA: true,
			Ports: []int{80}, ImageSizeMB: 8, BuildTimeMs: 60000,
			Optimizations: []string{"static-output", "code-splitting"},
			EnvVars: map[string]string{},
		},
		"svelte": {
			Name: "Svelte", Output: "public", BuildCmd: "rollup -c",
			StartCmd: "sirv public --no-clear --port 5000", RuntimeImage: "node:22-alpine", IsSPA: true,
			Ports: []int{5000}, ImageSizeMB: 5, BuildTimeMs: 30000,
			Optimizations: []string{"static-output"},
			EnvVars: map[string]string{},
		},
		"solidjs": {
			Name: "SolidJS", Output: "dist", BuildCmd: "vite build",
			StartCmd: "vite preview", RuntimeImage: "node:22-alpine", IsSPA: true,
			Ports: []int{3000}, ImageSizeMB: 6, BuildTimeMs: 25000,
			Optimizations: []string{"static-output", "fine-grained"},
			EnvVars: map[string]string{},
		},
		"preact": {
			Name: "Preact", Output: "build", BuildCmd: "preact build",
			StartCmd: "serve -s build -l 3000", RuntimeImage: "node:22-alpine", IsSPA: true,
			Ports: []int{3000}, ImageSizeMB: 5, BuildTimeMs: 40000,
			Optimizations: []string{"static-output", "tiny-bundle"},
			EnvVars: map[string]string{},
		},
		"lit": {
			Name: "Lit", Output: "dist", BuildCmd: "npm run build",
			StartCmd: "node dist/index.js", RuntimeImage: "node:22-alpine",
			Ports: []int{8000}, ImageSizeMB: 4, BuildTimeMs: 30000,
			Optimizations: []string{"web-components"},
			EnvVars: map[string]string{},
		},
		"qwik": {
			Name: "Qwik", Output: "dist", BuildCmd: "qwik build",
			StartCmd: "node server/entry.express", RuntimeImage: "node:22-alpine",
			Ports: []int{3000}, ImageSizeMB: 10, BuildTimeMs: 40000,
			Optimizations: []string{"resumability", "lazy-loading"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},
		"shopify-hydrogen": {
			Name: "Shopify Hydrogen", Output: "dist", BuildCmd: "hydrogen build",
			StartCmd: "npm run start", RuntimeImage: "node:22-alpine",
			Ports: []int{3000}, ImageSizeMB: 20, BuildTimeMs: 90000,
			Optimizations: []string{"oxygen", "streaming"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},

		// Mobile / Desktop
		"react-native": {
			Name: "React Native", Output: "android/app/build", BuildCmd: "react-native build-android",
			StartCmd: "", RuntimeImage: "ubuntu:22.04",
			Ports: []int{8081}, ImageSizeMB: 50, BuildTimeMs: 300000,
			Optimizations: []string{"android-build"},
			EnvVars: map[string]string{"ANDROID_HOME": "/opt/android-sdk"},
		},
		"expo": {
			Name: "Expo", Output: "dist", BuildCmd: "expo export",
			StartCmd: "npx serve dist", RuntimeImage: "node:22-alpine",
			Ports: []int{3000}, ImageSizeMB: 15, BuildTimeMs: 120000,
			Optimizations: []string{"expo-export"},
			EnvVars: map[string]string{},
		},
		"ionic": {
			Name: "Ionic", Output: "www", BuildCmd: "ionic build",
			StartCmd: "ionic serve", RuntimeImage: "node:22-alpine", IsSPA: true,
			Ports: []int{8100}, ImageSizeMB: 10, BuildTimeMs: 90000,
			Optimizations: []string{"capacitor", "pwa"},
			EnvVars: map[string]string{},
		},
		"quasar": {
			Name: "Quasar", Output: "dist/spa", BuildCmd: "quasar build",
			StartCmd: "quasar serve dist/spa", RuntimeImage: "node:22-alpine", IsSPA: true,
			Ports: []int{80}, ImageSizeMB: 12, BuildTimeMs: 120000,
			Optimizations: []string{"spa-mode"},
			EnvVars: map[string]string{},
		},
		"stencil": {
			Name: "Stencil", Output: "dist", BuildCmd: "stencil build",
			StartCmd: "http-server dist -p 3333", RuntimeImage: "node:22-alpine",
			Ports: []int{3333}, ImageSizeMB: 8, BuildTimeMs: 60000,
			Optimizations: []string{"web-components", "lazy-loading"},
			EnvVars: map[string]string{},
		},

		// Other
		"prisma": {
			Name: "Prisma", Output: "dist", BuildCmd: "prisma generate",
			StartCmd: "node dist/index.js", RuntimeImage: "node:22-alpine",
			Ports: []int{3300}, ImageSizeMB: 8, BuildTimeMs: 30000,
			Optimizations: []string{"code-generation"},
			EnvVars: map[string]string{},
		},
		"trpc": {
			Name: "tRPC", Output: "dist", BuildCmd: "tsc",
			StartCmd: "node dist/index.js", RuntimeImage: "node:22-alpine",
			Ports: []int{3000}, ImageSizeMB: 10, BuildTimeMs: 45000,
			Optimizations: []string{"type-safe"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},
		"gridsome": {
			Name: "Gridsome", Output: "dist", BuildCmd: "gridsome build",
			StartCmd: "", RuntimeImage: "nginx:alpine", IsStatic: true,
			Ports: []int{80}, ImageSizeMB: 10, BuildTimeMs: 90000,
			Optimizations: []string{"graphql-static"},
			EnvVars: map[string]string{},
		},
		"frontity": {
			Name: "Frontity", Output: "build", BuildCmd: "frontity build",
			StartCmd: "frontity serve", RuntimeImage: "node:22-alpine",
			Ports: []int{3000}, ImageSizeMB: 12, BuildTimeMs: 80000,
			Optimizations: []string{"serverless"},
			EnvVars: map[string]string{},
		},
		"sapper": {
			Name: "Sapper", Output: "__sapper__/export", BuildCmd: "npm run build",
			StartCmd: "node __sapper__/build/index.js", RuntimeImage: "node:22-alpine",
			Ports: []int{3000}, ImageSizeMB: 15, BuildTimeMs: 70000,
			Optimizations: []string{"svelte-fullstack"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},
		"umi": {
			Name: "UmiJS", Output: "dist", BuildCmd: "umi build",
			StartCmd: "node dist/index.js", RuntimeImage: "node:22-alpine",
			Ports: []int{8000}, ImageSizeMB: 18, BuildTimeMs: 90000,
			Optimizations: []string{"plugin-system"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},
		"genesis": {
			Name: "Genesis", Output: "dist", BuildCmd: "genesis build",
			StartCmd: "genesis start", RuntimeImage: "node:22-alpine",
			Ports: []int{3000}, ImageSizeMB: 12, BuildTimeMs: 60000,
			Optimizations: []string{"ultra-fast"},
			EnvVars: map[string]string{"NODE_ENV": "production"},
		},
		"modernizr": {
			Name: "Modernizr", Output: "dist", BuildCmd: "modernizr",
			StartCmd: "serve dist -p 3000", RuntimeImage: "node:22-alpine", IsStatic: true,
			Ports: []int{3000}, ImageSizeMB: 3, BuildTimeMs: 15000,
			Optimizations: []string{"feature-detection"},
			EnvVars: map[string]string{},
		},

		// Generic Node
		"node": {
			Name: "Node.js", Output: ".", BuildCmd: "",
			StartCmd: "node index.js", RuntimeImage: "node:22-alpine",
			Ports: []int{3000}, ImageSizeMB: 8, BuildTimeMs: 10000,
			Optimizations: []string{},
			EnvVars: map[string]string{},
		},
	}

	if cfg, ok := configs[framework]; ok {
		return cfg
	}

	// Default fallback
	return FrameworkConfig{
		Name: framework, Output: "dist", BuildCmd: "npm run build",
		StartCmd: "node dist/index.js", RuntimeImage: "node:22-alpine",
		Ports: []int{3000}, ImageSizeMB: 10, BuildTimeMs: 45000,
		Optimizations: []string{"generic"},
		EnvVars: map[string]string{"NODE_ENV": "production"},
	}
}

// detectVolumes detects common persistent volume requirements
func (p *Planner) detectVolumes(detection *analyzer.DetectionResult) []Volume {
	var volumes []Volume

	// Common storage directories that should be persistent
	if p.hasFile("storage/") {
		volumes = append(volumes, Volume{
			Type:   "persistent",
			Name:   "storage",
			Target: "/app/storage",
		})
	}

	if p.hasFile("data/") || p.hasFile("db/") {
		volumes = append(volumes, Volume{
			Type:   "persistent",
			Name:   "data",
			Target: "/app/data",
		})
	}

	if p.hasFile("uploads/") {
		volumes = append(volumes, Volume{
			Type:   "persistent",
			Name:   "uploads",
			Target: "/app/uploads",
		})
	}

	// Framework-specific volumes
	switch detection.Framework {
	case "nextjs":
		volumes = append(volumes, Volume{Type: "cache", Name: "next-cache", Target: "/app/.next/cache"})
	case "strapi", "keystonejs", "blitzjs":
		volumes = append(volumes, Volume{Type: "persistent", Name: "database", Target: "/app/data"})
		volumes = append(volumes, Volume{Type: "persistent", Name: "uploads", Target: "/app/public/uploads"})
	case "redwoodjs":
		volumes = append(volumes, Volume{Type: "persistent", Name: "db", Target: "/app/api/db"})
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
