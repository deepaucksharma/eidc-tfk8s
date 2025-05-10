# Script to update all import paths in Go files
# This will replace github.com/newrelic/nrdot-internal-devlab with eidc-tfk8s

$oldImportPath = "github.com/newrelic/nrdot-internal-devlab"
$newImportPath = "eidc-tfk8s"

Write-Host "Updating import paths from $oldImportPath to $newImportPath..." -ForegroundColor Cyan

# Find all .go files recursively
$goFiles = Get-ChildItem -Path . -Filter "*.go" -Recurse

$count = 0

foreach ($file in $goFiles) {
    $content = Get-Content -Path $file.FullName -Raw
    
    if ($content -match $oldImportPath) {
        $updatedContent = $content -replace $oldImportPath, $newImportPath
        Set-Content -Path $file.FullName -Value $updatedContent
        $count++
        Write-Host "Updated: $($file.FullName)" -ForegroundColor Green
    }
}

Write-Host "Done! Updated $count files." -ForegroundColor Cyan
