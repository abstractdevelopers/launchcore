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
	flagRepoPath    string
	flagOutputPath  string
	flagJSON        bool
	flagVerbose     bool
	flagDryRun      bool
)

func main() {
	flag.StringVar(&flagRepoPath, "repo", ".", "Path to repository")
	flag.StringVar(&flagRepoPath, "r", ".", "Path to repository (shorthand)")
	flag.StringVar(&flagOutputPath, "output", "", "Output Dockerfile path")
	flag.StringVar(&flagOutputPath, "o", "", "Output Dockerfile path (shorthand)")
	flag.BoolVar(&flagJSON, "json", false, "JSON output mode")
	flag.BoolVar(&flagVerbose, "verbose", false, "Verbose logging")
	flag.BoolVar(&flagDryRun, "dry-run", false, "Generate plan without Dockerfile")

	flag.Parse()

	// Create logger
	log := logger.NewLogger(!flagJSON)

	startTime := time.Now()

	// Header
	log.Header("🚀 LaunchCore Buildpack - Ultra-Fast Build System")

	// Step 1: Detection
	log.Info("Scanning repository: %s", flagRepoPath)

	detector := analyzer.NewDetector()
	detection, err := detector.Detect(flagRepoPath)
	if err != nil {
		log.ErrorLog(err, "Detection failed")
		os.Exit(1)
	}

	log.DetectionComplete(detection.Language, detection.Framework, detection.Confidence, time.Since(startTime))

	// Step 2: Planning
	log.Info("Generating optimized build plan...")
	planStart := time.Now()

	planGenerator := planner.NewPlanner()
	plan := planGenerator.GeneratePlan(detection)

	log.PlanGenerated(plan.Strategy, len(plan.Stages), plan.Metadata.OptimizationApplied, time.Since(planStart))

	// Step 3: Synthesis
	if !flagDryRun && plan.Strategy != "dockerfile_user_supplied" {
		log.Info("Synthesizing optimized Dockerfile...")
		synth := synthesizer.NewSynthesizer()

		dockerfile, err := synth.Synthesize(plan)
		if err != nil {
			log.ErrorLog(err, "Synthesis failed")
			os.Exit(1)
		}

		// Output
		if flagOutputPath != "" {
			outputPath := flagOutputPath
			if filepath.IsAbs(outputPath) == false {
				outputPath = filepath.Join(flagRepoPath, outputPath)
			}

			if err := os.WriteFile(outputPath, []byte(dockerfile), 0644); err != nil {
				log.ErrorLog(err, "Failed to write Dockerfile")
				os.Exit(1)
			}

			log.Info("Dockerfile written to: %s", outputPath)
		} else {
			fmt.Println(dockerfile)
		}
	}

	// Step 4: Plan JSON output
	if flagJSON || flagVerbose {
		planJSON, _ := json.MarshalIndent(plan, "", "  ")
		fmt.Println("\n--- Build Plan JSON ---")
		fmt.Println(string(planJSON))
	}

	// Summary
	log.BuildComplete(time.Since(startTime), 0.85, estimateImageSize(plan))

	if flagJSON {
		logsJSON, _ := log.OutputMachineLogs()
		fmt.Println("\n--- Machine Logs ---")
		fmt.Println(string(logsJSON))
	}
}

func estimateImageSize(plan *planner.BuildPlan) int {
	// Rough estimate based on runtime image
	baseSizes := map[string]int{
		"alpine":              5,
		"node:22-alpine":      15,
		"python:3.12-alpine":  10,
		"golang:1.22-alpine":  8,
		"rust:1.77-alpine":    12,
		"php:8.3-fpm-alpine":  8,
	}

	base := 5 // default small
	for pattern, size := range baseSizes {
		if plan.Runtime != "" && len(pattern) <= len(plan.Runtime) {
			if plan.Runtime[:len(pattern)] == pattern {
				base = size
			}
		}
	}

	return base
}
