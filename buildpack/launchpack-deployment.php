<?php

/**
 * ============================================================================
 * LAUNCHPACK INTEGRATION FOR COOLIFY
 * ============================================================================
 * 
 * FILE 1: app/Enums/BuildPackTypes.php
 * Add: case LAUNCHPACK = 'launchpack';
 * 
 * FILE 2: app/Jobs/ApplicationDeploymentJob.php
 * Add the methods below (deploy_launchpack_buildpack and build_launchpack_image)
 * 
 * FILE 3: resources/views/livewire/project/application/general.blade.php
 * Add option: <option value="launchpack">LaunchPack (Ultra-Fast)</option>
 * 
 * ============================================================================
 */

// ============================================================================
// STEP 1: Add to BuildPackTypes enum (app/Enums/BuildPackTypes.php)
// ============================================================================

// Add this line to the enum:
// case LAUNCHPACK = 'launchpack';


// ============================================================================
// STEP 2: Add to decide_what_to_do() switch (around line 504)
// ============================================================================

/*
In app/Jobs/ApplicationDeploymentJob.php, find the decide_what_to_do() method
and add this case:

} elseif ($this->application->build_pack === 'railpack') {
    $this->deploy_railpack_buildpack();
} elseif ($this->application->build_pack === 'launchpack') {
    $this->deploy_launchpack_buildpack();  // ADD THIS
} else {
    throw new DeploymentException("Unsupported build pack: {$this->application->build_pack}");
}


// ============================================================================
// STEP 3: Add deployment methods (add these at the end of the class)
// ============================================================================

/**
 * Deploy using LaunchPack - Ultra-fast buildpack
 */
private function deploy_launchpack_buildpack(): void
{
    if ($this->use_build_server) {
        $this->server = $this->build_server;
    }

    $this->application_deployment_queue->addLogEntry("Starting deployment with LaunchPack...");
    $this->application_deployment_queue->addLogEntry("LaunchPack: Ultra-fast build system (< 60s cold builds)");

    $this->prepare_builder_image();
    $this->check_git_if_build_needed();
    $this->generate_image_names();

    if (! $this->force_rebuild) {
        $this->check_image_locally_or_remotely();
        if ($this->should_skip_build()) {
            return;
        }
    }

    $this->clone_repository();
    $this->cleanup_git();
    $this->generate_compose_file();

    // Build with LaunchPack
    $this->build_launchpack_image();

    $this->save_buildtime_environment_variables();
    $this->save_runtime_environment_variables();
    $this->push_to_docker_registry();
    $this->rolling_update();
}

/**
 * Build image using LaunchPack
 */
private function build_launchpack_image(): void
{
    $this->application_deployment_queue->addLogEntry('----------------------------------------');
    $this->application_deployment_queue->addLogEntry('Building with LaunchPack...');

    // Step 1: Ensure buildpack binary is available
    // Downloads if not present (no image rebuild needed!)
    $downloadScript = <<<'BASH'
if [ ! -f /usr/local/bin/buildpack ]; then
    echo "Downloading LaunchPack binary..."
    curl -fsSL https://raw.githubusercontent.com/abstractdevelopers/launchcore/layer/buildpack/bin/buildpack -o /usr/local/bin/buildpack 2>/dev/null || \
    curl -fsSL https://cdn.example.com/buildpack -o /usr/local/bin/buildpack 2>/dev/null || \
    echo "Warning: buildpack binary not found. Please ensure it's installed."
    chmod +x /usr/local/bin/buildpack 2>/dev/null || true
fi
BASH;

    $this->execute_remote_command([
        executeInDocker($this->deployment_uuid, $downloadScript),
    ]);

    // Step 2: Check if buildpack is available
    $checkCmd = 'test -f /usr/local/bin/buildpack && /usr/local/bin/buildpack --version || echo "buildpack_not_found"';
    $result = $this->execute_remote_command([
        executeInDocker($this->deployment_uuid, $checkCmd),
        'hidden' => true,
    ]);

    $output = $this->saved_outputs->get('buildpack_version_check') ?? '';

    if (str_contains($output, 'buildpack_not_found') || ! str_contains($output, 'LaunchPack')) {
        // Fallback: Use nixpacks if buildpack not available
        $this->application_deployment_queue->addLogEntry('⚠️ LaunchPack binary not found, falling back to Nixpacks...');
        $this->deploy_nixpacks_buildpack();

        return;
    }

    // Step 3: Generate Dockerfile with LaunchPack
    $this->application_deployment_queue->addLogEntry('Generating build plan with LaunchPack...');

    $generateCmd = '/usr/local/bin/buildpack -repo '.$this->workdir.' -o '.$this->workdir.'/.coolify/Dockerfile 2>&1';

    $this->execute_remote_command(
        [
            executeInDocker($this->deployment_uuid, $generateCmd),
            'hidden' => true,
            'save' => 'launchpack_output',
        ],
        [
            executeInDocker($this->deployment_uuid, 'cat '.$this->workdir.'/.coolify/Dockerfile 2>/dev/null || echo "Dockerfile generation failed"'),
            'hidden' => true,
        ]
    );

    // Log LaunchPack output
    $launchpackOutput = $this->saved_outputs->get('launchpack_output') ?? '';
    if ($launchpackOutput) {
        $lines = explode("\n", $launchpackOutput);
        foreach ($lines as $line) {
            if (trim($line)) {
                $this->application_deployment_queue->addLogEntry('LaunchPack: '.$line);
            }
        }
    }

    // Step 4: Build the Docker image
    $this->application_deployment_queue->addLogEntry('Building Docker image...');

    if ($this->dockerBuildkitSupported) {
        $build_command = "DOCKER_BUILDKIT=1 docker build {$this->addHosts} --network host -f {$this->workdir}/.coolify/Dockerfile --progress plain -t {$this->production_image_name} {$this->workdir}";
    } else {
        $build_command = "docker build {$this->addHosts} --network host -f {$this->workdir}/.coolify/Dockerfile -t {$this->production_image_name} {$this->workdir}";
    }

    $base64_build_command = base64_encode($build_command);

    $this->execute_remote_command(
        [
            executeInDocker($this->deployment_uuid, "echo '{$base64_build_command}' | base64 -d | tee ".self::BUILD_SCRIPT_PATH.' > /dev/null'),
            'hidden' => true,
        ],
        [
            executeInDocker($this->deployment_uuid, 'cat '.self::BUILD_SCRIPT_PATH),
            'hidden' => true,
        ],
        [
            executeInDocker($this->deployment_uuid, 'bash '.self::BUILD_SCRIPT_PATH),
            'hidden' => false, // Show build progress
        ]
    );

    $this->application_deployment_queue->addLogEntry('LaunchPack build completed!');
    $this->application_deployment_queue->addLogEntry('----------------------------------------');
}


// ============================================================================
// STEP 4: Add UI option (resources/views/livewire/project/application/general.blade.php)
// ============================================================================

/*
Add this option to the build pack selector (around line 35):

<option value="launchpack">LaunchPack (Ultra-Fast)</option>

Example:

<x-forms.select wire:model.live="buildPack" label="Build Pack" required>
    <option value="nixpacks">Nixpacks</option>
    <option value="launchpack">LaunchPack (Ultra-Fast)</option>  <!-- ADD THIS -->
    <option value="railpack">Railpack (Beta)</option>
    <option value="static">Static</option>
    <option value="dockerfile">Dockerfile</option>
    <option value="dockercompose">Docker Compose</option>
</x-forms.select>
*/
