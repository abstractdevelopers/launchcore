<?php

namespace App\Jobs;

use App\Actions\Server\UpdateCoolify;
use App\Models\Server;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldBeEncrypted;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;
use Illuminate\Support\Facades\Log;

class UpdateCoolifyJob implements ShouldBeEncrypted, ShouldQueue
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    public $timeout = 600;

    public function __construct()
    {
        $this->onQueue('high');
    }

    public function handle(): void
    {
        // Auto-update is disabled - platform is running in standalone mode
        // No external HTTP calls or automatic updates are performed
        return;
    }
}
