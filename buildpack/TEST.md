# LaunchPack Test Guide for Kamatera

## Prerequisites

1. Coolify installed on Kamatera server
2. API enabled in Coolify settings
3. API token from Coolify

## Step 1: Get Your API Token

1. Log into Coolify dashboard
2. Go to Settings > API
3. Create a new API token
4. Save it securely

## Step 2: Test API Request

Replace the values and run:

```bash
# Set variables
COOLIFY_URL="https://your-coolify-domain.com"
API_TOKEN="your-api-token-here"
TEAM_ID="your-team-id"  # Usually 1 for first team

# Create application with LaunchPack build pack
curl -X POST "${COOLIFY_URL}/api/v1/applications" \
  -H "Authorization: Bearer ${API_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-launchpack",
    "description": "Test app with LaunchPack",
    "git_repository": "https://github.com/abstractdevelopers/launchcore",
    "git_branch": "layer",
    "build_pack": "launchpack",
    "port": 3000,
    "health_check_enabled": true,
    "health_check_path": "/"
  }'
```

## Step 3: Deploy the Application

After creating, trigger deployment:

```bash
# Get the application UUID from the response
APP_UUID="your-app-uuid"

# Deploy
curl -X POST "${COOLIFY_URL}/api/v1/deploy" \
  -H "Authorization: Bearer ${API_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{
    \"deployment_uuid\": \"${APP_UUID}\"
  }"
```

## Step 4: Monitor Deployment

In Coolify dashboard:
1. Go to your test application
2. Click "Deployments"
3. Watch the logs for "LaunchPack" messages

Expected logs:
```
Starting deployment with LaunchPack...
LaunchPack: Ultra-fast build system (< 60s cold builds)
Building with LaunchPack...
Generating build plan with LaunchPack...
Building Docker image...
LaunchPack build completed!
```

## Manual Test (without API)

1. Create new application in Coolify UI
2. Select Build Pack: **LaunchPack (Ultra-Fast)**
3. Enter Git repository: `https://github.com/abstractdevelopers/launchcore`
4. Branch: `layer`
5. Deploy

## Troubleshooting

### Buildpack binary not found

```bash
# SSH into server
ssh root@your-kamatera-server

# Check if binary is in container
docker exec coolify /usr/local/bin/buildpack --version

# If not, install it
docker cp buildpack coolify:/usr/local/bin/
docker exec coolify chmod +x /usr/local/bin/buildpack
```

### PHP errors after code changes

```bash
# Clear Laravel cache
docker exec coolify php artisan optimize:clear

# Or full cache clear
docker exec coolify php artisan cache:clear
docker exec coolify php artisan config:clear
docker exec coolify php artisan view:clear
```

### Check PHP syntax

```bash
docker exec coolify php -l /var/www/html/app/Jobs/ApplicationDeploymentJob.php
```

## Expected Performance

| Metric | With LaunchPack | With Nixpacks |
|--------|-----------------|---------------|
| Cold Build | ~45-60s | ~90-120s |
| Warm Build | ~5-10s | ~20-30s |
| Detection | < 50ms | ~200-500ms |

## Verification Checklist

- [ ] Binary installed in container
- [ ] Enum case added
- [ ] Deployment method added
- [ ] UI option visible
- [ ] Cache cleared
- [ ] Container restarted
- [ ] Test deployment successful
- [ ] Build time measured

## Build Time Test

```bash
# Time your deployment
time curl -X POST "${COOLIFY_URL}/api/v1/deploy" \
  -H "Authorization: Bearer ${API_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"application_uuid": "your-uuid"}'
```

Compare the time with Nixpacks deployments.
