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
    & $signTool.FullName sign `
        /fd SHA256 `
        /td SHA256 `
        /tr 'https://timestamp.digicert.com' `
        /f $env:PAIEMAROC_CERTIFICATE_PATH `
        /p $env:WINDOWS_CERTIFICATE_PASSWORD `
        $Path

    if ($LASTEXITCODE -ne 0) {
        throw "Échec de la signature Authenticode de $Path"
    }
}

& $signTool.FullName verify /pa /all /v $Path
if ($LASTEXITCODE -ne 0) {
    throw "Échec de la vérification Authenticode de $Path"
}
