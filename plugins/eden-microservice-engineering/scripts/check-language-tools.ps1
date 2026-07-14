param([switch]$Install)

$ErrorActionPreference = "Stop"
$checks = @(
  @{ Name = "gopls"; Command = "gopls"; Install = { & go install golang.org/x/tools/gopls@latest } },
  @{ Name = "typescript-language-server"; Command = "typescript-language-server"; Install = { & npm install -g typescript-language-server typescript } },
  @{ Name = "vue-language-server"; Command = "vue-language-server"; Install = { & npm install -g @vue/language-server } }
)

foreach ($check in $checks) {
  if (-not (Get-Command $check.Command -ErrorAction SilentlyContinue) -and $Install) {
    & $check.Install
  }
  if (Get-Command $check.Command -ErrorAction SilentlyContinue) {
    Write-Host "available: $($check.Name)"
  } else {
    Write-Warning "missing: $($check.Name). Run .\scripts\check-language-tools.ps1 -Install"
  }
}
