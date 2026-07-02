param(
    [string]$InstallDir = "$env:LOCALAPPDATA\AnyRouter",
    [switch]$AddToPath = $true,
    [switch]$CreateShortcuts = $true
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$Cyan = "`e[36m"
$Green = "`e[32m"
$Yellow = "`e[33m"
$Red = "`e[31m"
$NC = "`e[0m"

Write-Host "$Cyan"
Write-Host @"
  █████  ███    ██ ██    ██ ██████   ██████  ██    ██ ████████ ███████ ██████  
 ██   ██ ████   ██  ██  ██  ██   ██ ██    ██ ██    ██    ██    ██      ██   ██ 
 ███████ ██ ██  ██   ████   ██████  ██    ██ ██    ██    ██    █████   ██████  
 ██   ██ ██  ██ ██    ██    ██   ██ ██    ██ ██    ██    ██    ██      ██   ██ 
 ██   ██ ██   ████    ██    ██   ██  ██████   ██████     ██    ███████ ██   ██
 =============================================================================
"@
Write-Host "$NC"

Write-Host "${Green}AnyRouter Installer for Windows${NC}"
Write-Host ""

# Check Admin
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "${Yellow}Note: Not running as Administrator.${NC}"
    Write-Host "${Yellow}Some features (system-wide PATH, all-users shortcuts) may not work.${NC}"
    Write-Host ""
}

# Detect architecture
$arch = $env:PROCESSOR_ARCHITECTURE
switch ($arch) {
    "AMD64" { $goarch = "amd64" }
    "ARM64" { $goarch = "arm64" }
    "X86"   { $goarch = "386" }
    default {
        Write-Host "${Red}Unsupported architecture: $arch${NC}"
        exit 1
    }
}

$binaryName = "anyrouter-windows-$goarch.exe"

Write-Host "${Yellow}Detected:${NC} Windows/$goarch"
Write-Host ""

# Check for local build (development mode)
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$localBuild = Join-Path $scriptDir "..\build\$binaryName"
if (-not (Test-Path $localBuild)) {
    $localBuild = Join-Path $scriptDir "..\anyrouter.exe"
}

$tmpDir = Join-Path $env:TEMP "anyrouter-install"
if (Test-Path $tmpDir) { Remove-Item -Recurse -Force $tmpDir }
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

$destPath = Join-Path $tmpDir "anyrouter.exe"

if (Test-Path $localBuild) {
    Write-Host "${Green}Using local build...${NC}"
    Copy-Item -Path $localBuild -Destination $destPath
}
else {
    Write-Host "${Yellow}Downloading latest release...${NC}"
    $downloadUrl = "https://github.com/anyrouter/cli/releases/latest/download/$binaryName"
    try {
        Invoke-WebRequest -Uri $downloadUrl -OutFile $destPath -UseBasicParsing
    }
    catch {
        Write-Host "${Red}Download failed: $_${NC}"
        Write-Host "${Yellow}Try building from source: go install github.com/anyrouter/cli@latest${NC}"
        exit 1
    }
}

# Create installation directory
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$finalPath = Join-Path $InstallDir "anyrouter.exe"
Copy-Item -Path $destPath -Destination $finalPath -Force

# Create a .cmd wrapper in a well-known PATH location as fallback
$system32Path = [Environment]::GetFolderPath("System")
$wrapperPath = Join-Path $system32Path "anyrouter.cmd"
$wrapperContent = "@echo off`r`n\"$finalPath\" %*"
Set-Content -Path $wrapperPath -Value $wrapperContent -Force
Write-Host "${Green}Created launcher: $wrapperPath${NC}"

Write-Host "${Green}Installed to: $finalPath${NC}"
Write-Host ""

# Add to PATH
if ($AddToPath) {
    if ($isAdmin) {
        # Add to System PATH for all users
        $machinePath = [Environment]::GetEnvironmentVariable("PATH", "Machine")
        if ($machinePath -notlike "*$InstallDir*") {
            [Environment]::SetEnvironmentVariable("PATH", "$machinePath;$InstallDir", "Machine")
            Write-Host "${Green}Added to System PATH (all users).${NC}"
        }
        else {
            Write-Host "${Green}Already in System PATH.${NC}"
        }
    }
    else {
        $userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
        if ($userPath -notlike "*$InstallDir*") {
            $newPath = "$userPath;$InstallDir"
            [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
            Write-Host "${Green}Added to User PATH.${NC}"
        }
        else {
            Write-Host "${Green}Already in PATH.${NC}"
        }
    }
}

# Create Start Menu shortcut
if ($CreateShortcuts) {
    $startMenuDir = Join-Path $env:ProgramData "Microsoft\Windows\Start Menu\Programs\AnyRouter"
    if (-not (Test-Path $startMenuDir)) {
        New-Item -ItemType Directory -Path $startMenuDir -Force | Out-Null
    }

    # Create shortcut via WScript.Shell
    $wshell = New-Object -ComObject WScript.Shell
    $shortcut = $wshell.CreateShortcut("$startMenuDir\AnyRouter.lnk")
    $shortcut.TargetPath = $finalPath
    $shortcut.Description = "AnyRouter - Universal LLM API Router"
    $shortcut.WorkingDirectory = $InstallDir
    $shortcut.Save()

    # Server shortcut
    $svcShortcut = $wshell.CreateShortcut("$startMenuDir\AnyRouter Server.lnk")
    $svcShortcut.TargetPath = $finalPath
    $svcShortcut.Arguments = "--serve"
    $svcShortcut.Description = "AnyRouter - Start Server Mode"
    $svcShortcut.WorkingDirectory = $InstallDir
    $svcShortcut.Save()

    Write-Host "${Green}Created Start Menu shortcuts.${NC}"
}

# Create uninstaller
$uninstallPs1 = Join-Path $InstallDir "uninstall.ps1"
$uninstallContent = @'
$InstallDir = "$env:LOCALAPPDATA\AnyRouter"

# Remove from PATH
$userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
$newPath = ($userPath -split ';' | Where-Object { $_ -ne $InstallDir }) -join ';'
[Environment]::SetEnvironmentVariable("PATH", $newPath, "User")

# Remove from system PATH
$machinePath = [Environment]::GetEnvironmentVariable("PATH", "Machine")
$newMachinePath = ($machinePath -split ';' | Where-Object { $_ -ne $InstallDir }) -join ';'
[Environment]::SetEnvironmentVariable("PATH", $newMachinePath, "Machine")

# Remove launcher
$wrapperPath = Join-Path ([Environment]::GetFolderPath("System")) "anyrouter.cmd"
if (Test-Path $wrapperPath) { Remove-Item -Path $wrapperPath -Force }

# Remove shortcuts
$startMenuDir = Join-Path $env:ProgramData "Microsoft\Windows\Start Menu\Programs\AnyRouter"
if (Test-Path $startMenuDir) { Remove-Item -Recurse -Force $startMenuDir }

# Remove installation directory
if (Test-Path $InstallDir) { Remove-Item -Recurse -Force $InstallDir }

# Remove uninstall registry key
$uninstallKey = "HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\AnyRouter"
if (Test-Path $uninstallKey) { Remove-Item -Path $uninstallKey -Force }

Write-Host "AnyRouter has been uninstalled."
'@
Set-Content -Path $uninstallPs1 -Value $uninstallContent -Force

# Register in Add/Remove Programs
if ($isAdmin) {
    $uninstallKey = "HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\AnyRouter"
    if (-not (Test-Path $uninstallKey)) {
        New-Item -Path $uninstallKey -Force | Out-Null
    }
    Set-ItemProperty -Path $uninstallKey -Name "DisplayName" -Value "AnyRouter"
    Set-ItemProperty -Path $uninstallKey -Name "DisplayVersion" -Value "1.1.0"
    Set-ItemProperty -Path $uninstallKey -Name "Publisher" -Value "AnyRouter"
    Set-ItemProperty -Path $uninstallKey -Name "InstallLocation" -Value $InstallDir
    Set-ItemProperty -Path $uninstallKey -Name "UninstallString" -Value "powershell.exe -NoProfile -ExecutionPolicy Bypass -File `"$uninstallPs1`""
    Set-ItemProperty -Path $uninstallKey -Name "DisplayIcon" -Value $finalPath
    Write-Host "${Green}Registered in Add/Remove Programs.${NC}"
}

Write-Host ""
Write-Host "${Green}Installation complete!${NC}"
Write-Host ""
Write-Host "Run ${Cyan}anyrouter${NC} to start the interactive TUI."
Write-Host "Run ${Cyan}anyrouter --serve${NC} for direct server mode."
Write-Host "To uninstall, run: ${Cyan}powershell -File `"$uninstallPs1`"${NC}"
Write-Host ""
Write-Host "${Cyan}  Website: https://anyrouter.planixx.com${NC}"
Write-Host "${Cyan}  GitHub:  https://github.com/anyrouter/cli${NC}"
Write-Host ""

# Refresh PATH in current session
$env:PATH = [Environment]::GetEnvironmentVariable("PATH", "User") + ";" + [Environment]::GetEnvironmentVariable("PATH", "Machine") + ";$InstallDir"

# Verify
try {
    $ver = & anyrouter --version 2>&1
    Write-Host "${Green}Verified: $ver${NC}"
}
catch {
    try {
        $ver = & $finalPath --version 2>&1
        Write-Host "${Green}Verified: $ver${NC}"
    }
    catch {
        Write-Host "${Yellow}Verification skipped. Run 'anyrouter --version' to verify.${NC}"
    }
}

Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue
