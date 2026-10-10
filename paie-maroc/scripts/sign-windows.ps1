param(
    [Parameter(Mandatory = $true)]
    [string]$Path,
    [switch]$VerifyOnly
)

$ErrorActionPreference = 'Stop'

if (-not $env:PAIEMAROC_CERTIFICATE_PATH -or -not $env:WINDOWS_CERTIFICATE_PASSWORD) {
    throw 'Les variables de signature Authenticode sont absentes.'
}

$signTool = Get-ChildItem 'C:\Program Files (x86)\Windows Kits\10\bin' `
    -Filter signtool.exe -Recurse |
    Where-Object FullName -Match '\\x64\\' |
    Sort-Object FullName |
    Select-Object -Last 1

if (-not $signTool) {
    throw 'signtool.exe est introuvable sur le runner Windows.'
}

if (-not $VerifyOnly) {
    $arguments = @(
        'sign',
        '/fd', 'SHA256',
        '/f', $env:PAIEMAROC_CERTIFICATE_PATH,
        '/p', $env:WINDOWS_CERTIFICATE_PASSWORD
    )
    if ($env:PAIEMAROC_SELF_SIGNED -ne 'true') {
        $arguments += @('/td', 'SHA256', '/tr', 'https://timestamp.digicert.com')
    }
    $arguments += $Path
    & $signTool.FullName @arguments

    if ($LASTEXITCODE -ne 0) {
        throw "Échec de la signature Authenticode de $Path"
    }
}

if ($env:PAIEMAROC_SELF_SIGNED -eq 'true') {
    if (-not $VerifyOnly) {
        Write-Host "Signature auto-signée créée : $Path"
        return
    }

    $verification = @(& $signTool.FullName verify /pa /all /v $Path 2>&1)
    $verifyExitCode = $LASTEXITCODE
    $verifyText = $verification -join "`n"
    $verification | ForEach-Object { Write-Host $_ }
    $normalizedText = $verifyText -replace '\s', ''
    if ($normalizedText -notmatch [regex]::Escape($env:PAIEMAROC_SELF_SIGNED_THUMBPRINT)) {
        throw "L'empreinte Authenticode de $Path ne correspond pas au certificat provisoire attendu."
    }
    if ($verifyExitCode -ne 0 -and
        $normalizedText -notmatch 'rootcertificate.*nottrusted') {
        throw "Échec de la vérification Authenticode de $Path"
    }
    Write-Host "Signature auto-signée et empreinte vérifiées : $Path"
    return
}

& $signTool.FullName verify /pa /all /v $Path
if ($LASTEXITCODE -ne 0) {
    throw "Échec de la vérification Authenticode de $Path"
}
