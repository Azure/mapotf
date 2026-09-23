Describe 'Get-ReleaseChecksum.ps1' {
  BeforeAll {
    $helper = Join-Path -Path $PSScriptRoot -ChildPath '..' -AdditionalChildPath 'scripts', 'Get-ReleaseChecksum.ps1'

    function gh {
      $global:MapotfSigningTest.ApiCalls += ,@($args)
      if ($args[1] -match '/assets\?per_page=100$') {
        $global:LASTEXITCODE = $global:MapotfSigningTest.AssetsExitCode
        return $global:MapotfSigningTest.AssetsResponse
      }
      $global:LASTEXITCODE = $global:MapotfSigningTest.ApiExitCode
      if ($args[1] -match '/releases/tags/') {
        return $global:MapotfSigningTest.TagResponse
      }
      return $global:MapotfSigningTest.ReleaseResponse
    }

    function Invoke-WebRequest {
      param(
        [string]$Uri,
        [hashtable]$Headers,
        [string]$OutFile,
        [switch]$PassThru
      )
      $global:MapotfSigningTest.DownloadRequest = @{ Uri = $Uri; Headers = $Headers; OutFile = $OutFile; PassThru = $PassThru }
      [System.IO.File]::WriteAllBytes($OutFile, $global:MapotfSigningTest.DownloadBytes)
      if ($global:MapotfSigningTest.DownloadError) {
        throw 'Simulated download failure'
      }
      return @{ Headers = @{ 'Content-Type' = $global:MapotfSigningTest.ContentType } }
    }

    $originalToken = $env:GH_TOKEN
    $originalOutput = $env:GITHUB_OUTPUT
    $originalLastExitCode = $global:LASTEXITCODE
  }

  BeforeEach {
    $global:MapotfSigningTest = @{
      ApiCalls = @()
      ApiExitCode = 0
      AssetsExitCode = 0
      TagResponse = '{"id":123}'
      ReleaseResponse = '{"id":123,"tag_name":"v1.0.0","draft":false}'
      AssetsResponse = '[[{"id":456,"name":"checksums.txt","state":"uploaded","size":7}]]'
      DownloadBytes = [System.Text.Encoding]::UTF8.GetBytes('1234567')
      ContentType = 'application/octet-stream'
      DownloadRequest = $null
      DownloadError = $false
    }
    $env:GH_TOKEN = 'test-token'
    $testPath = Join-Path $TestDrive ([guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $testPath | Out-Null
    $env:GITHUB_OUTPUT = Join-Path $testPath 'output'
    Push-Location $testPath
  }

  AfterEach {
    Pop-Location
  }

  AfterAll {
    $env:GH_TOKEN = $originalToken
    $env:GITHUB_OUTPUT = $originalOutput
    $global:LASTEXITCODE = $originalLastExitCode
    Remove-Variable MapotfSigningTest -Scope Global
  }

  Describe 'Checking the release checksum asset' {
    It 'uses the release event ID and finds the completed checksum on a later asset page' {
      $global:MapotfSigningTest.AssetsResponse = '[[{"id":12,"name":"archive.zip","state":"uploaded","size":4}],[{"id":456,"name":"checksums.txt","state":"uploaded","size":7}]]'

      & $helper -Mode Check -Repository Azure/mapotf -Tag v1.0.0 -EventReleaseId 123

      (Get-Content $env:GITHUB_OUTPUT) -join ',' | Should -Be 'found=true,asset_id=456,asset_size=7'
      $global:MapotfSigningTest.ApiCalls.Count | Should -Be 2
      $global:MapotfSigningTest.ApiCalls[0][1] | Should -Be 'repos/Azure/mapotf/releases/123'
      $global:MapotfSigningTest.ApiCalls[1] -join ' ' | Should -Be 'api repos/Azure/mapotf/releases/123/assets?per_page=100 --paginate --slurp'
    }

    It 'resolves a manual tag to an ID and validates the ID against the release' {
      $global:MapotfSigningTest.ReleaseResponse = '{"id":123,"tag_name":"v1/test","draft":false}'

      & $helper -Mode Check -Repository Azure/mapotf -Tag 'v1/test'

      $global:MapotfSigningTest.ApiCalls[0][1] | Should -Be 'repos/Azure/mapotf/releases/tags/v1%2Ftest'
      $global:MapotfSigningTest.ApiCalls[1][1] | Should -Be 'repos/Azure/mapotf/releases/123'
      (Get-Content $env:GITHUB_OUTPUT) -join ',' | Should -Be 'found=true,asset_id=456,asset_size=7'
    }

    It 'skips signing only when the checksum is absent from every page' {
      $global:MapotfSigningTest.AssetsResponse = '[[{"id":12,"name":"archive.zip","state":"uploaded","size":4}],[]]'

      & $helper -Mode Check -Repository Azure/mapotf -Tag v1.0.0 -EventReleaseId 123

      (Get-Content $env:GITHUB_OUTPUT) -join ',' | Should -Be 'found=false'
    }

    It 'rejects a release with a different tag' {
      $global:MapotfSigningTest.ReleaseResponse = '{"id":123,"tag_name":"v2.0.0","draft":false}'

      { & $helper -Mode Check -Repository Azure/mapotf -Tag v1.0.0 -EventReleaseId 123 } |
        Should -Throw '*does not match published tag*'
      Test-Path $env:GITHUB_OUTPUT | Should -BeFalse
    }

    It 'rejects a release ID that does not match the event' {
      $global:MapotfSigningTest.ReleaseResponse = '{"id":124,"tag_name":"v1.0.0","draft":false}'

      { & $helper -Mode Check -Repository Azure/mapotf -Tag v1.0.0 -EventReleaseId 123 } |
        Should -Throw '*does not match published tag*'
      Test-Path $env:GITHUB_OUTPUT | Should -BeFalse
    }

    It 'rejects a draft release' {
      $global:MapotfSigningTest.ReleaseResponse = '{"id":123,"tag_name":"v1.0.0","draft":true}'

      { & $helper -Mode Check -Repository Azure/mapotf -Tag v1.0.0 -EventReleaseId 123 } |
        Should -Throw '*does not match published tag*'
    }

    It 'rejects an invalid event release ID' {
      { & $helper -Mode Check -Repository Azure/mapotf -Tag v1.0.0 -EventReleaseId '../latest' } |
        Should -Throw '*Invalid release ID*'
      $global:MapotfSigningTest.ApiCalls.Count | Should -Be 0
    }

    It 'fails instead of skipping when resolving a manual tag fails' {
      $global:MapotfSigningTest.ApiExitCode = 1

      { & $helper -Mode Check -Repository Azure/mapotf -Tag v1.0.0 } |
        Should -Throw '*GitHub API request failed*'
      Test-Path $env:GITHUB_OUTPUT | Should -BeFalse
    }

    It 'fails instead of skipping when listing assets fails' {
      $global:MapotfSigningTest.AssetsExitCode = 1

      { & $helper -Mode Check -Repository Azure/mapotf -Tag v1.0.0 -EventReleaseId 123 } |
        Should -Throw '*Failed to list assets*'
      Test-Path $env:GITHUB_OUTPUT | Should -BeFalse
    }

    It 'fails instead of skipping when the asset list is malformed' {
      $global:MapotfSigningTest.AssetsResponse = '{"assets":[]}'

      { & $helper -Mode Check -Repository Azure/mapotf -Tag v1.0.0 -EventReleaseId 123 } |
        Should -Throw '*Invalid asset list*'
      Test-Path $env:GITHUB_OUTPUT | Should -BeFalse
    }

    It 'rejects an incomplete checksum asset' {
      $global:MapotfSigningTest.AssetsResponse = '[[{"id":456,"name":"checksums.txt","state":"starter","size":7}]]'

      { & $helper -Mode Check -Repository Azure/mapotf -Tag v1.0.0 -EventReleaseId 123 } |
        Should -Throw '*not a single completed asset*'
    }

    It 'rejects an empty checksum asset' {
      $global:MapotfSigningTest.AssetsResponse = '[[{"id":456,"name":"checksums.txt","state":"uploaded","size":0}]]'

      { & $helper -Mode Check -Repository Azure/mapotf -Tag v1.0.0 -EventReleaseId 123 } |
        Should -Throw '*Invalid checksum asset size*'
    }

    It 'rejects duplicate checksum assets' {
      $global:MapotfSigningTest.AssetsResponse = '[[{"id":456,"name":"checksums.txt","state":"uploaded","size":7}],[{"id":457,"name":"checksums.txt","state":"uploaded","size":7}]]'

      { & $helper -Mode Check -Repository Azure/mapotf -Tag v1.0.0 -EventReleaseId 123 } |
        Should -Throw '*not a single completed asset*'
    }
  }

  Describe 'Downloading the release checksum asset' {
    It 'downloads authenticated binary content by asset ID and verifies its size' {
      & $helper -Mode Download -Repository Azure/mapotf -AssetId 456 -AssetSize 7

      (Get-Content -Raw checksums.txt) | Should -Be '1234567'
      $global:MapotfSigningTest.DownloadRequest.Uri | Should -Be 'https://api.github.com/repos/Azure/mapotf/releases/assets/456'
      $global:MapotfSigningTest.DownloadRequest.Headers.Authorization | Should -Be 'Bearer test-token'
      $global:MapotfSigningTest.DownloadRequest.Headers.Accept | Should -Be 'application/octet-stream'
      $global:MapotfSigningTest.DownloadRequest.PassThru | Should -BeTrue
      Test-Path checksums.txt.download | Should -BeFalse
    }

    It 'rejects a size mismatch without keeping a partial file' {
      { & $helper -Mode Download -Repository Azure/mapotf -AssetId 456 -AssetSize 8 } |
        Should -Throw '*size mismatch*'

      Test-Path checksums.txt | Should -BeFalse
      Test-Path checksums.txt.download | Should -BeFalse
    }

    It 'cleans up a partial file if the authenticated request fails' {
      $global:MapotfSigningTest.DownloadError = $true

      { & $helper -Mode Download -Repository Azure/mapotf -AssetId 456 -AssetSize 7 } |
        Should -Throw '*Simulated download failure*'
      Test-Path checksums.txt | Should -BeFalse
      Test-Path checksums.txt.download | Should -BeFalse
    }

    It 'rejects JSON metadata even if it has the expected size' {
      $global:MapotfSigningTest.ContentType = 'application/json; charset=utf-8'

      { & $helper -Mode Download -Repository Azure/mapotf -AssetId 456 -AssetSize 7 } |
        Should -Throw '*metadata instead of checksums.txt*'
      Test-Path checksums.txt | Should -BeFalse
    }

    It 'rejects an existing checksum file rather than overwriting it' {
      Set-Content -Path checksums.txt -Value 'already here'

      { & $helper -Mode Download -Repository Azure/mapotf -AssetId 456 -AssetSize 7 } |
        Should -Throw '*Refusing to overwrite*'
      $global:MapotfSigningTest.DownloadRequest | Should -BeNullOrEmpty
    }
  }
}
