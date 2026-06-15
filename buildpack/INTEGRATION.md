# Integration Guide: LaunchCore Buildpack → Coolify

## Overview

This guide explains how to integrate the LaunchCore Buildpack into your Coolify installation (either on your server or as a custom Docker image).

## Two Integration Methods

### Method 1: Direct File Override (Quick Test)

```bash
# SSH into your server
ssh user@your-server

# Stop coolify container
docker stop coolify

# Copy buildpack files
docker cp buildpack/bin/buildpack coolify:/usr/local/bin/

# Make executable
docker exec coolify chmod +x /usr/local/bin/buildpack

# Add enum case
docker exec coolify bash -c 'echo "case LAUNCHPACK = '\''launchpack'\'';" >> /var/www/html/app/Enums/BuildPackTypes.php'

# Copy the PHP integration code
# (See integration.php below)

# Restart
docker start coolify
```

### Method 2: Custom Docker Image (Production)

```bash
# In your fork directory
cd buildpack

# Build the binary
go build -o buildpack ./cmd/buildpack

# Create Dockerfile.buildpack
cat > Dockerfile.buildpack << 'EOF'
FROM golang:1.22-alpine AS builder
WORKDIR /build
COPY . .
RUN go build -o buildpack ./cmd/buildpack

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /build/buildpack /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/buildpack"]
EOF

docker build -f Dockerfile.buildpack -t launchpack:latest .

# Tag for registry
docker tag launchpack:latest ghcr.io/yourusername/launchpack:latest

# Push
docker push ghcr.io/yourusername/launchpack:latest
```

---

## Step 1: Add to BuildPackTypes Enum

Add to `app/Enums/BuildPackTypes.php`:

```php
case LAUNCHPACK = 'launchpack';
```

---

## Step 2: Add Deployment Method

Add to `app/Jobs/ApplicationDeploymentJob.php`:

```php
// In decide_what_to_do() switch:
} elseif ($this->application->build_pack === 'launchpack') {
    $this->deploy_launchpack_buildpack();

// Add new method (~100 lines):
private function deploy_launchpack_buildpack(): void
{
    if ($this->use_build_server) {
        $this->server = $this->build_server;
    }

    $this->application_deployment_queue->addLogEntry("Starting deployment with LaunchPack...");
    $this->prepare_builder_image();
    $this->check_git_if_build_needed();
    $this->generate_image_names();

    if (!$this->force_rebuild) {
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

private function build_launchpack_image(): void
{
    // Step 1: Generate Dockerfile using buildpack
    $this->application_deployment_queue->addLogEntry('Generating build plan with LaunchPack...');

    $generateCmd = '/usr/local/bin/buildpack -repo ' . $this->workdir . ' -o ' . $this->workdir . '/.coolify/Dockerfile';

    $this->execute_remote_command([
        executeInDocker($this->deployment_uuid, $generateCmd),
        'hidden' => true,
    ]);

    // Step 2: Build using the generated Dockerfile
    $this->application_deployment_queue->addLogEntry('Building image with LaunchPack...');

    $build_command = "docker build -f {$this->workdir}/.coolify/Dockerfile -t {$this->production_image_name} {$this->workdir}";

    if ($this->dockerBuildkitSupported) {
        $build_command = "DOCKER_BUILDKIT=1 " . $build_command;
    }

    $base64_build_command = base64_encode($build_command);
    $this->execute_remote_command([
        executeInDocker($this->deployment_uuid, "echo '{$base64_build_command}' | base64 -d | tee /tmp/build.sh > /dev/null"),
        'hidden' => true,
    ],[
        executeInDocker($this->deployment_uuid, 'bash /tmp/build.sh'),
        'hidden' => true,
    ]);

    $this->application_deployment_queue->addLogEntry('Build completed.');
}
```

---

## Step 3: Add UI Option

In `resources/views/livewire/project/application/general.blade.php`:

```blade
<option value="launchpack">LaunchPack (Ultra-Fast)</option>
```

Add it to the build pack selector:

```blade
<x-forms.select wire:model.live="buildPack" label="Build Pack" required>
    <option value="nixpacks">Nixpacks</option>
    <option value="launchpack">LaunchPack (Ultra-Fast)</option>  <!-- ADD THIS -->
    <option value="railpack">Railpack (Beta)</option>
    <option value="static">Static</option>
    <option value="dockerfile">Dockerfile</option>
    <option value="dockercompose">Docker Compose</option>
</x-forms.select>
```

---

## Step 4: Build & Deploy

### For Testing on Server:

```bash
# Build the binary
cd /path/to/launchcore/buildpack
go build -o buildpack ./cmd/buildpack

# Copy to server
scp buildpack user@your-server:/tmp/

# SSH and copy into container
ssh user@your-server
docker cp /tmp/buildpack coolify:/usr/local/bin/
docker exec coolify chmod +x /usr/local/bin/buildpack

# Restart
docker restart coolify
```

### For Production (Custom Image):

```bash
# Build image with Coolify + buildpack
cd /path/to/launchcore

# Create custom Dockerfile
cat > Dockerfile.coolify << 'EOF'
FROM ghcr.io/coollabsio/coolify:v4.1.2

# Install buildpack binary
COPY buildpack/bin/buildpack /usr/local/bin/
RUN chmod +x /usr/local/bin/buildpack

# Your custom code changes
COPY app/Enums/BuildPackTypes.php /var/www/html/app/Enums/
COPY app/Jobs/ApplicationDeploymentJob.php /var/www/html/app/Jobs/
COPY resources/views/... /var/www/html/resources/views/

# Clear cache
RUN cd /var/www/html && php artisan optimize:clear
EOF

# Build
docker build -f Dockerfile.coolify -t my-coolify:latest .

# Run
docker run -d \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -p 8000:8080 \
  my-coolify:latest
```

---

## Logging Integration

The buildpack outputs structured logs. To capture them in Coolify:

```php
// In build_launchpack_image():
$output = $this->saved_outputs->get('launchpack_output');

// Parse and log
if ($output) {
    $logs = json_decode($output, true);
    foreach ($logs['logs'] ?? [] as $log) {
        $this->application_deployment_queue->addLogEntry(
            "[LaunchPack] " . ($log['message'] ?? json_encode($log))
        );
    }
}
```

---

## Verification

After integration, you should see:

1. **Build Pack Dropdown** - "LaunchPack (Ultra-Fast)" option
2. **Build Logs** - Shows detection, planning, optimization phases
3. **Performance** - Faster builds than Nixpacks/Railpack

---

## Troubleshooting

### Buildpack Not Found
```bash
# Check if binary exists in container
docker exec coolify which buildpack
docker exec coolify buildpack --version
```

### Permission Denied
```bash
# Fix permissions
docker exec coolify chmod +x /usr/local/bin/buildpack
```

### Detection Fails
```bash
# Run with verbose mode
docker exec coolify /usr/local/bin/buildpack -repo /app -verbose

# Check JSON output
docker exec coolify /usr/local/bin/buildpack -repo /app -json
```

---

## Next Steps

1. ✅ Build the `buildpack` binary
2. ✅ Test on your server
3. ✅ Create custom Docker image
4. ✅ Deploy to production
5. ✅ Monitor build times and optimize

## Performance Monitoring

Track these metrics:

- **Cold Build Time**: Target < 60s
- **Warm Build Time**: Target < 10s
- **Cache Hit Rate**: Target > 80%
- **Image Size**: Compare with Nixpacks output

```bash
# Enable verbose logging to see metrics
./buildpack -repo . -json | jq '.logs[] | select(.event == "build_complete")'
```
