[CmdletBinding()]
param(
  [Parameter(Mandatory)]
  [ValidateSet('Check', 'Download')]
  [string]$Mode,
  [Parameter(Mandatory)]
  [string]$Repository,
  [string]$Tag,
  [string]$EventReleaseId,
  [string]$AssetId,
  [string]$AssetSize
)

$ErrorActionPreference = 'Stop'

if ($Repository -cnotmatch '^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$') {
  throw "Invalid repository: $Repository"
}
if ([string]::IsNullOrWhiteSpace($env:GH_TOKEN)) {
  throw 'GH_TOKEN is required'
}

function ConvertTo-PositiveInt64 {
  param([object]$Value, [string]$Name)

  [long]$number = 0
  if ($null -eq $Value -or [string]$Value -cnotmatch '^[1-9][0-9]*$' -or
      -not [long]::TryParse([string]$Value, [ref]$number)) {
    throw "Invalid $Name"
  }
  return $number
}

function Get-GhApiObject {
  param([string]$Endpoint)

  $json = gh api $Endpoint
  if ($LASTEXITCODE -ne 0) {
    throw "GitHub API request failed: $Endpoint"
  }
  return ConvertFrom-Json -InputObject ($json -join "`n") -AsHashtable
}

if ($Mode -eq 'Check') {
  if ([string]::IsNullOrWhiteSpace($Tag)) {
    throw 'A release tag is required'
  }
  if ([string]::IsNullOrWhiteSpace($env:GITHUB_OUTPUT)) {
    throw 'GITHUB_OUTPUT is required'
  }

  if ([string]::IsNullOrWhiteSpace($EventReleaseId)) {
    $encodedTag = [uri]::EscapeDataString($Tag)
    $byTag = Get-GhApiObject "repos/$Repository/releases/tags/$encodedTag"
    $releaseId = ConvertTo-PositiveInt64 $byTag.id 'release ID'
  } else {
    $releaseId = ConvertTo-PositiveInt64 $EventReleaseId 'release ID'
  }

  $release = Get-GhApiObject "repos/$Repository/releases/$releaseId"
  if ((ConvertTo-PositiveInt64 $release.id 'release ID') -ne $releaseId -or
      $release.tag_name -cne $Tag -or $release.draft -ne $false) {
    throw "Release $releaseId does not match published tag $Tag"
  }

  $json = gh api "repos/$Repository/releases/$releaseId/assets?per_page=100" --paginate --slurp
  if ($LASTEXITCODE -ne 0) {
    throw "Failed to list assets for release $releaseId"
  }
  $pages = ConvertFrom-Json -InputObject ($json -join "`n") -AsHashtable -NoEnumerate
  if ($pages -isnot [array]) {
    throw "Invalid asset list for release $releaseId"
  }

  $checksums = @(
    foreach ($page in $pages) {
      if ($page -isnot [array]) {
        throw "Invalid asset page for release $releaseId"
      }
      foreach ($asset in $page) {
        if ($asset -isnot [System.Collections.IDictionary] -or
            [string]::IsNullOrWhiteSpace($asset.name)) {
          throw "Invalid asset metadata for release $releaseId"
        }
        if ($asset.name -ceq 'checksums.txt') {
          $asset
        }
      }
    }
  )

  if ($checksums.Count -eq 0) {
    'found=false' | Add-Content -LiteralPath $env:GITHUB_OUTPUT
    Write-Host "::warning::No checksums.txt attached to release $Tag, nothing to sign."
    return
  }
  if ($checksums.Count -ne 1 -or $checksums[0].state -cne 'uploaded') {
    throw "checksums.txt is not a single completed asset on release $releaseId"
  }

  $assetId = ConvertTo-PositiveInt64 $checksums[0].id 'checksum asset ID'
  $assetSize = ConvertTo-PositiveInt64 $checksums[0].size 'checksum asset size'
  @('found=true', "asset_id=$assetId", "asset_size=$assetSize") |
    Add-Content -LiteralPath $env:GITHUB_OUTPUT
  return
}

$assetId = ConvertTo-PositiveInt64 $AssetId 'checksum asset ID'
$assetSize = ConvertTo-PositiveInt64 $AssetSize 'checksum asset size'
$checksumPath = Join-Path (Get-Location).Path 'checksums.txt'
$tempPath = Join-Path (Get-Location).Path 'checksums.txt.download'
if ((Test-Path -LiteralPath $checksumPath) -or (Test-Path -LiteralPath $tempPath)) {
  throw 'Refusing to overwrite an existing checksum download'
}

try {
  $response = Invoke-WebRequest `
    -Uri "https://api.github.com/repos/$Repository/releases/assets/$assetId" `
    -Headers @{
      Authorization = "Bearer $env:GH_TOKEN"
      Accept = 'application/octet-stream'
      'X-GitHub-Api-Version' = '2022-11-28'
    } `
    -OutFile $tempPath -PassThru

  if ($response.Headers['Content-Type'] -match '(?i)(json|text/html)') {
    throw 'GitHub returned asset metadata instead of checksums.txt'
  }
  $actualSize = (Get-Item -LiteralPath $tempPath).Length
  if ($actualSize -ne $assetSize) {
    throw "Downloaded checksums.txt size mismatch: expected $assetSize bytes, got $actualSize"
  }
  Move-Item -LiteralPath $tempPath -Destination $checksumPath
} finally {
  if (Test-Path -LiteralPath $tempPath) {
    Remove-Item -LiteralPath $tempPath
  }
}
