<?php

namespace App\Console\Commands;

use App\Jobs\CheckTraefikVersionJob;
use Illuminate\Console\Command;

class CheckTraefikVersionCommand extends Command
{
    protected $signature = 'traefik:check-version';

    protected $description = 'Check Traefik proxy versions on all servers and send notifications for outdated versions';

    public function handle(): int
    {
        // Traefik version check is disabled for standalone mode
        $this->warn('Traefik version checking is disabled in standalone mode.');

        return Command::SUCCESS;
    }
}
