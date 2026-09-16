param(
    [Parameter(Mandatory=$true)][string]$Binary,
    [string[]]$Modes = @('AAAA', 'AAAB', 'AABA', 'AABB', 'ABAA', 'ABAB', 'ABBA', 'ABBB'),
    [ValidateSet('device', 'instance', 'enumerate')][string]$Level = 'device',
    [ValidateRange(1, 100)][int]$Runs = 2,
    [ValidateRange(1, 300)][int]$TimeoutSeconds = 15,
    [string]$OutputDirectory = (Join-Path $env:TEMP ('vulkan-thread-close-' + (Get-Date -Format 'yyyyMMdd-HHmmss-fff')))
)
$ErrorActionPreference = 'Stop'
$binaryPath = (Resolve-Path -LiteralPath $Binary).Path
foreach ($mode in $Modes) {
    if ($mode -cnotmatch '^[AB]{4}$') { throw "Invalid mode: $mode" }
}
New-Item -ItemType Directory -Path $OutputDirectory -Force | Out-Null
$outputPath = (Resolve-Path -LiteralPath $OutputDirectory).Path
$results = @()
foreach ($mode in $Modes) {
    for ($run = 1; $run -le $Runs; $run++) {
        $name = "$Level-$mode-$run"
        $stdout = Join-Path $outputPath "$name.stdout.txt"
        $stderr = Join-Path $outputPath "$name.stderr.txt"
        $process = Start-Process -FilePath $binaryPath -ArgumentList @($mode, $Level) `
            -WindowStyle Hidden -PassThru -RedirectStandardOutput $stdout -RedirectStandardError $stderr
        $timedOut = -not $process.WaitForExit($TimeoutSeconds * 1000)
        if ($timedOut) {
            # Only this diagnostic process; it creates threads, no child processes.
            $process.Kill()
            $process.WaitForExit()
        }
        $process.Refresh()
        $lines = @(Get-Content -LiteralPath $stdout)
        $result = [pscustomobject]@{
            Name = $name
            ExitCode = $process.ExitCode
            TimedOut = $timedOut
            Success = (!$timedOut -and $process.ExitCode -eq 0 -and $lines -contains 'SUCCESS')
            Last = ($lines | Select-Object -Last 1)
        }
        $results += $result
        $result | ConvertTo-Json -Compress
        $process.Dispose()
    }
}
ConvertTo-Json -InputObject $results -Depth 3 | Set-Content -LiteralPath (Join-Path $outputPath 'results.json')
Write-Host "Logs: $outputPath"
