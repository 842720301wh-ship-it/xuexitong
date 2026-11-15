param([switch]$CheckTools, [switch]$Offline)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot

# Prefer explicit configuration, then existing tools on this computer.
function Find-Directory($Candidates, $RequiredFile, $Description) {
    foreach ($candidate in $Candidates) {
        if ($candidate -and (Test-Path -LiteralPath (Join-Path $candidate $RequiredFile))) { return [IO.Path]::GetFullPath($candidate) }
    }
    throw "Missing $Description. See README.md for TIANTIAN_* tool settings."
}
$reference = $env:TIANTIAN_ANDROID_REFERENCE_ROOT
if (-not $reference) {
    $parent = Split-Path -Parent (Split-Path -Parent $projectRoot)
    $reference = Get-ChildItem -LiteralPath $parent -Directory | Where-Object {
        Test-Path -LiteralPath (Join-Path $_.FullName '.workbench-local/android-sdk')
    } | Select-Object -First 1 -ExpandProperty FullName
}
$referenceTools = if ($reference) { Join-Path $reference '.workbench-local' } else { $null }
$javaHome = Find-Directory @($env:TIANTIAN_JAVA_HOME, $env:JAVA_HOME, 'D:/HBuilderX/plugins/amazon-corretto') 'bin/javac.exe' 'JDK 17+'
$sdkCandidates = @($env:TIANTIAN_ANDROID_SDK, $env:ANDROID_HOME, $env:ANDROID_SDK_ROOT)
if ($referenceTools) { $sdkCandidates += Join-Path $referenceTools 'android-sdk' }
if ($env:LOCALAPPDATA) { $sdkCandidates += Join-Path $env:LOCALAPPDATA 'Android/Sdk' }
$sdk = Find-Directory $sdkCandidates 'platforms/android-36/android.jar' 'Android SDK 36'
$ndkVersion = if ($env:TIANTIAN_NDK_VERSION) { $env:TIANTIAN_NDK_VERSION } else { '28.2.13676358' }
$ndk = if ($env:ANDROID_NDK_HOME) { $env:ANDROID_NDK_HOME } else { Join-Path $sdk "ndk/$ndkVersion" }
$compiler = Join-Path $ndk 'toolchains/llvm/prebuilt/windows-x86_64/bin/aarch64-linux-android26-clang.cmd'
if (-not (Test-Path -LiteralPath $compiler)) { throw "Missing Android NDK compiler: $compiler" }
$gradle = $env:TIANTIAN_GRADLE
if (-not $gradle) {
    $installed = Get-Command gradle.bat -ErrorAction SilentlyContinue
    if ($installed) { $gradle = $installed.Source }
    elseif ($referenceTools) { $gradle = Join-Path $referenceTools 'gradle-8.13/bin/gradle.bat' }
}
if (-not $gradle -or -not (Test-Path -LiteralPath $gradle)) { throw 'Missing Gradle 8.13. Set TIANTIAN_GRADLE to gradle.bat.' }
if (-not (Test-Path -LiteralPath (Join-Path $sdk 'build-tools/36.0.0/apksigner.bat'))) { throw 'Missing Android SDK Build Tools 36.0.0.' }
if ($CheckTools) { Write-Host 'Android toolchain ready.'; exit 0 }

$metadata = Get-Content -LiteralPath (Join-Path $projectRoot 'package.json') -Raw -Encoding UTF8 | ConvertFrom-Json
$names = @('JAVA_HOME','ANDROID_HOME','ANDROID_USER_HOME','GRADLE_USER_HOME','GRADLE_RO_DEP_CACHE','JAVA_TOOL_OPTIONS','PATH','GOCACHE','GOTMPDIR','TEMP','TMP','GOOS','GOARCH','CGO_ENABLED','CC','CGO_LDFLAGS')
$saved = @{}
foreach ($name in $names) { $saved[$name] = [Environment]::GetEnvironmentVariable($name, 'Process') }
try {
    $env:JAVA_HOME = $javaHome
    $env:ANDROID_HOME = $sdk
    $cache = Join-Path $projectRoot '.cache'
    $directories = @{
        ANDROID_USER_HOME = 'android-user'; GRADLE_USER_HOME = 'gradle'; GOCACHE = 'go-build'
        GOTMPDIR = 'go-tmp'; TEMP = 'android-tmp'; TMP = 'android-tmp'
    }
    foreach ($name in $directories.Keys) {
        $directory = Join-Path $cache $directories[$name]
        New-Item -ItemType Directory -Path $directory -Force | Out-Null
        [Environment]::SetEnvironmentVariable($name, $directory, 'Process')
    }
    # Read shared dependency artifacts without writing to another project.
    if (-not $env:GRADLE_RO_DEP_CACHE) {
        $shared = Join-Path (Split-Path -Parent $projectRoot) 'panguai/.cache/gradle/caches'
        if (Test-Path -LiteralPath (Join-Path $shared 'modules-2')) { $env:GRADLE_RO_DEP_CACHE = $shared }
    }
    $env:PATH = (Join-Path $javaHome 'bin') + [IO.Path]::PathSeparator + $env:PATH
    if ($env:HTTPS_PROXY) {
        $proxy = [Uri]$env:HTTPS_PROXY
        $env:JAVA_TOOL_OPTIONS = "$env:JAVA_TOOL_OPTIONS -Dhttps.proxyHost=$($proxy.Host) -Dhttps.proxyPort=$($proxy.Port) -Dhttp.proxyHost=$($proxy.Host) -Dhttp.proxyPort=$($proxy.Port) -Dhttp.nonProxyHosts=localhost|127.*"
    }

    $web = Join-Path $projectRoot 'dist/web'
    if (-not (Test-Path -LiteralPath (Join-Path $web 'index.html'))) { throw 'Build the frontend before packaging Android.' }
    $entry = Join-Path $projectRoot 'backend/cmd/android'
    $embedded = [IO.Path]::GetFullPath((Join-Path $entry 'dist'))
    $allowed = [IO.Path]::GetFullPath($entry).TrimEnd('\','/') + [IO.Path]::DirectorySeparatorChar
    if (-not $embedded.StartsWith($allowed, [StringComparison]::OrdinalIgnoreCase)) { throw 'Invalid generated asset directory.' }
    if (Test-Path -LiteralPath $embedded) {
        if ((Get-Item -LiteralPath $embedded).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Generated asset directory cannot be a link.' }
        Remove-Item -LiteralPath $embedded -Recurse -Force
    }
    New-Item -ItemType Directory -Path $embedded -Force | Out-Null
    Copy-Item -Path (Join-Path $web '*') -Destination $embedded -Recurse -Force
    $native = Join-Path $projectRoot 'android/app/src/main/jniLibs/arm64-v8a'
    New-Item -ItemType Directory -Path $native -Force | Out-Null
    $env:GOOS = 'android'; $env:GOARCH = 'arm64'; $env:CGO_ENABLED = '1'; $env:CC = $compiler
    $env:CGO_LDFLAGS = '-Wl,-z,max-page-size=16384'
    Push-Location (Join-Path $projectRoot 'backend')
    try {
        & go build -buildvcs=false -trimpath -buildmode=c-shared -ldflags '-s -w' -o (Join-Path $native 'libtiantian.so') ./cmd/android
        if ($LASTEXITCODE -ne 0) { throw 'Embedded Go build failed.' }
    } finally { Pop-Location }
    $android = Join-Path $projectRoot 'android'
    [IO.File]::WriteAllText((Join-Path $android 'local.properties'), ('sdk.dir=' + $sdk.Replace('\','/')), (New-Object Text.UTF8Encoding($false)))
    # Keep the debug signing key in the ignored local build cache.
    $key = Join-Path $cache 'android-debug.keystore'
    if (-not (Test-Path -LiteralPath $key)) {
        & (Join-Path $javaHome 'bin/keytool.exe') -genkeypair -keystore $key -storepass android -keypass android -alias androiddebugkey -dname 'CN=Android Debug,O=Android,C=US' -keyalg RSA -keysize 2048 -validity 10000
        if ($LASTEXITCODE -ne 0) { throw 'Debug signing key generation failed.' }
    }
    $gradleArgs = @('-p', $android, '--no-daemon', '--console=plain', 'assembleDebug')
    if ($Offline) { $gradleArgs += '--offline' }
    & $gradle @gradleArgs
    if ($LASTEXITCODE -ne 0) { throw 'Android package build failed.' }
    $apk = Join-Path $android 'app/build/outputs/apk/debug/app-debug.apk'
    & (Join-Path $sdk 'build-tools/36.0.0/apksigner.bat') verify $apk
    if ($LASTEXITCODE -ne 0) { throw 'APK signature verification failed.' }
    $release = Join-Path $projectRoot 'release/Android'
    New-Item -ItemType Directory -Path $release -Force | Out-Null
    $target = Join-Path $release "$($metadata.android.displayName)-$($metadata.version)-debug.apk"
    Copy-Item -LiteralPath $apk -Destination $target -Force
    $sha256 = [Security.Cryptography.SHA256]::Create()
    $stream = [IO.File]::OpenRead($target)
    try { $digest = [BitConverter]::ToString($sha256.ComputeHash($stream)).Replace('-', '').ToLowerInvariant() }
    finally { $stream.Dispose(); $sha256.Dispose() }
    Write-Host "SHA256: $digest"
    Write-Host "APK ready: $target"
} finally {
    foreach ($name in $names) { [Environment]::SetEnvironmentVariable($name, $saved[$name], 'Process') }
}
