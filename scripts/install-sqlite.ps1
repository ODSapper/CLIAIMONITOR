$ErrorActionPreference = "Stop"

$sqliteDir = "C:\sqlite"
$zipUrl = "https://www.sqlite.org/2024/sqlite-tools-win-x64-3470200.zip"
$tempZip = Join-Path $env:TEMP "sqlite-tools.zip"

Write-Host "Installing SQLite CLI tools..."
Write-Host "Download URL: $zipUrl"
Write-Host "Temp file: $tempZip"
Write-Host "Install dir: $sqliteDir"

# Download
Write-Host "`nDownloading..."
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
(New-Object Net.WebClient).DownloadFile($zipUrl, $tempZip)

# Extract
Write-Host "Extracting..."
if (Test-Path $sqliteDir) {
    Remove-Item $sqliteDir -Recurse -Force
}
Expand-Archive -Path $tempZip -DestinationPath $sqliteDir -Force

# The zip contains a subdirectory, move contents up
$subDir = Get-ChildItem $sqliteDir -Directory | Select-Object -First 1
if ($subDir) {
    Get-ChildItem $subDir.FullName | Move-Item -Destination $sqliteDir -Force
    Remove-Item $subDir.FullName -Force
}

# Verify
Write-Host "`nInstalled files:"
Get-ChildItem $sqliteDir

# Add to PATH for this session
$env:PATH = "$sqliteDir;$env:PATH"

# Test
Write-Host "`nTesting sqlite3..."
& "$sqliteDir\sqlite3.exe" --version

Write-Host "`nSQLite installed to $sqliteDir"
Write-Host "Add to PATH: $sqliteDir"
