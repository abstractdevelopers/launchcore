package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/abstractdevelopers/launchcore/buildpack/internal/analyzer"
	"github.com/abstractdevelopers/launchcore/buildpack/internal/logger"
	"github.com/abstractdevelopers/launchcore/buildpack/internal/planner"
	"github.com/abstractdevelopers/launchcore/buildpack/internal/synthesizer"
)

var (
	flagRepoPath   string
	flagOutputPath string
	flagJSON       bool
	flagVerbose    bool
	flagDryRun     bool
	flagPlanOnly   bool
)

func main() {
	flag.StringVar(&flagRepoPath, "repo", ".", "Path to repository")
	flag.StringVar(&flagRepoPath, "r", ".", "Path to repository (shorthand)")
	flag.StringVar(&flagOutputPath, "output", "", "Output Dockerfile path")
	flag.StringVar(&flagOutputPath, "o", "", "Output Dockerfile path (shorthand)")
	flag.BoolVar(&flagJSON, "json", false, "JSON output mode")
	flag.BoolVar(&flagVerbose, "verbose", false, "Verbose logging")
	flag.BoolVar(&flagDryRun, "dry-run", false, "Generate plan without Dockerfile")
	flag.BoolVar(&flagPlanOnly, "plan", false, "Output build plan JSON only")

	flag.Parse()

	// Create logger
	log := logger.NewLogger(!flagJSON)
	startTime := time.Now()

	// Header
	if !flagJSON {
		fmt.Println("🚀 LaunchPack - Ultra-Fast Build System v1.0")
		fmt.Println("============================================")
		fmt.Println()
	}

	// Step 1: Detection
	fmt.Println("📍 Step 1/3: Detecting repository type...")
	detector := analyzer.NewDetector()
	detection, err := detector.Detect(flagRepoPath)
	if err != nil {
		log.ErrorLog(err, "Detection failed")
		os.Exit(1)
	}

	log.DetectionComplete(detection.Language, detection.Framework, detection.Confidence, time.Since(startTime))

	// Step 2: Planning
	fmt.Println("📋 Step 2/3: Generating build plan...")
	planStart := time.Now()

	planGenerator := planner.NewPlanner()
	plan := planGenerator.GeneratePlan(detection)

	log.PlanGenerated(plan.Strategy, len(plan.Stages), plan.Metadata.OptimizationApplied, time.Since(planStart))

	// Output plan only if requested
	if flagPlanOnly {
		planJSON, _ := json.MarshalIndent(plan, "", "  ")
		fmt.Println(string(planJSON))
		return
	}

	// Step 3: Synthesis
	if !flagDryRun && plan.Strategy != "dockerfile_user_supplied" {
		fmt.Println("🔧 Step 3/3: Synthesizing Dockerfile...")

		synth := synthesizer.NewSynthesizer()
		dockerfile, err := synth.Synthesize(plan)
		if err != nil {
			if err.Error() == "user-supplied Dockerfile" {
				fmt.Println("⚠️  User-supplied Dockerfile detected. Skipping synthesis.")
			} else {
				log.ErrorLog(err, "Synthesis failed")
				os.Exit(1)
			}
		} else {
			// Output Dockerfile
			if flagOutputPath != "" {
				outputPath := flagOutputPath
				if !filepath.IsAbs(outputPath) {
					outputPath = filepath.Join(flagRepoPath, outputPath)
				}

				// Ensure directory exists
				os.MkdirAll(filepath.Dir(outputPath), 0755)

				if err := os.WriteFile(outputPath, []byte(dockerfile), 0644); err != nil {
					log.ErrorLog(err, "Failed to write Dockerfile")
					os.Exit(1)
				}
				fmt.Printf("✅ Dockerfile written to: %s\n", outputPath)
			} else {
				fmt.Println("\n" + dockerfile)
			}
		}
	}

	// Summary
	totalDuration := time.Since(startTime)
	if !flagJSON {
		fmt.Println()
		fmt.Println("============================================")
		fmt.Println("✅ Build Complete!")
		fmt.Println("============================================")
		fmt.Printf("  Total Time:   %.2fs\n", totalDuration.Seconds())
		fmt.Printf("  Language:     %s\n", detection.Language)
		fmt.Printf("  Framework:    %s\n", detection.Framework)
		fmt.Printf("  Strategy:     %s\n", plan.Strategy)
		fmt.Printf("  Stages:       %d\n", len(plan.Stages))
		fmt.Printf("  Cache Hits:   %.0f%%\n", plan.Metadata.CacheHitRate*100)
		fmt.Printf("  Image Size:   ~%dMB\n", plan.Metadata.ImageSizeEstimateMB)
		fmt.Println()
		fmt.Println("Commands for Coolify integration:")
		fmt.Printf("  Install: %s\n", plan.InstallCmd)
		fmt.Printf("  Build:   %s\n", plan.BuildCmd)
		fmt.Printf("  Start:   %s\n", plan.StartCmd)
		fmt.Println("============================================")
	}

	// Output structured data
	if flagJSON {
		result := map[string]interface{}{
			"success": true,
			"detection": detection,
			"plan": plan,
			"duration_ms": totalDuration.Milliseconds(),
		}
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(out))
	}
}
