# Smoke-test a published HowlFrame release on Windows x86-64.
#
#   powershell -ExecutionPolicy Bypass -File scripts\release_smoke.ps1 [-Version v0.3.0]   # default: latest
#
# Windows counterpart of scripts/release_smoke.sh, with the same checks. Downloads the windows_amd64 archive and
# SHA256SUMS from GitHub Releases, verifies the checksum, and runs the extracted howlframe.exe by absolute path.
# Results go to stdout and to <SmokeDir>\results.md, and to the job summary under GitHub Actions.
# Exit status: 0 when every check passed. Works on Windows PowerShell 5.1 and PowerShell 7.
param(
  [string]$Version = "",
  [string]$SmokeDir = ""
)
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"   # Invoke-WebRequest is very slow with the progress bar in 5.1
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$Repo = "howlcipher/howlframe"
if (-not $SmokeDir) { $SmokeDir = Join-Path ([IO.Path]::GetTempPath()) ("howlframe-smoke-" + [guid]::NewGuid().ToString("N").Substring(0, 8)) }
New-Item -ItemType Directory -Force -Path $SmokeDir | Out-Null
$SmokeDir = (Resolve-Path $SmokeDir).Path
$Results = Join-Path $SmokeDir "results.md"
[IO.File]::WriteAllText($Results, "")

$script:npass = 0; $script:nfail = 0
function Out-Line([string]$s) { Write-Output $s; [IO.File]::AppendAllText($Results, $s + "`n") }
function Check([string]$name, [scriptblock]$test) {
  $ok = $false
  try { $ok = [bool](& $test) } catch { $ok = $false }
  if ($ok) { $script:npass++; Out-Line "| PASS | $name |" } else { $script:nfail++; Out-Line "| FAIL | $name |" }
}
# Run howlframe; returns @{ Code; Out } with stdout lines joined by "`n" and stderr discarded.
function HF {
  $ErrorActionPreference = "Continue"   # under Stop, Windows PowerShell 5.1 throws on any native stderr output
  $o = & $script:HFExe @args 2>$null
  @{ Code = $LASTEXITCODE; Out = (($o | ForEach-Object { "$_" }) -join "`n").TrimEnd() }
}
# Write a .howl source without a BOM and with LF line endings.
function Write-Source([string]$path, [string]$text) {
  [IO.File]::WriteAllText((Join-Path $script:W $path), ($text -replace "`r`n", "`n"))
}

if (-not $Version) {
  $r = Invoke-WebRequest -UseBasicParsing -Uri "https://github.com/$Repo/releases/latest" -Method Head
  $final = $r.BaseResponse.ResponseUri
  if (-not $final) { $final = $r.BaseResponse.RequestMessage.RequestUri }   # PowerShell 7
  $Version = ($final.AbsoluteUri -split "/")[-1]
}
if ($Version -notmatch '^v[0-9]') { Write-Error "Version must look like v0.3.0, got '$Version'"; exit 2 }
$Num = $Version.Substring(1)
$Archive = "howlframe_${Version}_windows_amd64.zip"
$Base = "https://github.com/$Repo/releases/download/$Version"

$os = Get-CimInstance Win32_OperatingSystem -ErrorAction SilentlyContinue
Out-Line "## HowlFrame $Version release smoke: windows/amd64"
Out-Line ""
Out-Line "- Host: ``$($os.Caption) $($os.Version)``, PowerShell $($PSVersionTable.PSVersion), CPU $env:PROCESSOR_ARCHITECTURE"
if ($env:GITHUB_ACTIONS) { Out-Line "- Runner: GitHub Actions ``$env:RUNNER_NAME``, run $env:GITHUB_SERVER_URL/$env:GITHUB_REPOSITORY/actions/runs/$env:GITHUB_RUN_ID" }
Out-Line "- Work directory: ``$SmokeDir``"
Out-Line ""
Out-Line "| Result | Check |"
Out-Line "|---|---|"

$DL = Join-Path $SmokeDir "dl"; $script:W = Join-Path $SmokeDir "work"
New-Item -ItemType Directory -Force -Path $DL, $W | Out-Null

Check "download $Archive and SHA256SUMS" {
  Invoke-WebRequest -UseBasicParsing -Uri "$Base/$Archive" -OutFile (Join-Path $DL $Archive)
  Invoke-WebRequest -UseBasicParsing -Uri "$Base/SHA256SUMS" -OutFile (Join-Path $DL "SHA256SUMS")
  $true
}
Check "SHA256SUMS lists $Archive and it matches" {
  $line = Get-Content (Join-Path $DL "SHA256SUMS") | Where-Object { $_ -match "  $([regex]::Escape($Archive))$" }
  if (-not $line) { return $false }
  $want = ($line -split "\s+")[0].ToLower()
  $got = (Get-FileHash (Join-Path $DL $Archive) -Algorithm SHA256).Hash.ToLower()
  $want -eq $got
}
Check "archive extracts to one howlframe.exe" {
  Expand-Archive -Force -Path (Join-Path $DL $Archive) -DestinationPath $DL
  Test-Path (Join-Path $DL "howlframe.exe")
}
$script:HFExe = Join-Path $DL "howlframe.exe"
if (-not (Test-Path $HFExe)) { Out-Line ""; Out-Line "RESULT: FAILURES (no executable)"; exit 1 }
$ExeSha = (Get-FileHash $HFExe -Algorithm SHA256).Hash.ToLower()

Check "howlframe version is $Num / HFBC format 2" { (HF version).Out -eq "HowlFrame $Num`nHFBC format: 2" }
Check "help names $Num" { ((HF help).Out -split "`n")[0] -eq "HowlFrame $Num CLI" }
Check "interop --json: schema howl.interop/v1, version $Num, hfbc_format 2" {
  $j = (HF interop --json).Out
  $j.Contains('"schema": "howl.interop/v1"') -and $j.Contains("`"version`": `"$Num`"") -and $j.Contains('"hfbc_format": "2"')
}

Push-Location $W
try {
  Write-Source "hello.howl" "(cli_app`n  (print `"Hello from HowlFrame`"))`n"
  Check "check hello.howl" { (HF check hello.howl).Code -eq 0 }
  Check "build hello.howl writes an HFBC artifact" {
    (HF build hello.howl).Code -eq 0 -and ([Text.Encoding]::ASCII.GetString([IO.File]::ReadAllBytes((Join-Path $W "hello.hfbc")), 0, 4) -eq "HFBC")
  }
  Check "run hello.hfbc prints the greeting" { (HF run hello.hfbc).Out -eq "Hello from HowlFrame" }

  Write-Source "bad.howl" "(cli_app`n  (print (+ 1 `"two`")))`n"
  Write-Source "unbalanced.howl" "(cli_app`n  (print `"missing paren`")`n"
  Check "type error rejected by check" { (HF check bad.howl).Code -ne 0 }
  Check "parse error rejected by check" { (HF check unbalanced.howl).Code -ne 0 }
  Check "build refuses invalid source and writes no artifact" { (HF build bad.howl).Code -ne 0 -and -not (Test-Path (Join-Path $W "bad.hfbc")) }

  New-Item -ItemType Directory -Force -Path (Join-Path $W "data") | Out-Null
  [IO.File]::WriteAllText((Join-Path $W "data\note.txt"), "fixture text`n")
  [IO.File]::WriteAllText((Join-Path $W "outside.txt"), "outside`n")
  Write-Source "reader.howl" "(cli_app`n  (print (bytes_to_string (read_file `"data/note.txt`"))))`n"
  Write-Source "escape.howl" "(cli_app`n  (print (bytes_to_string (read_file `"outside.txt`"))))`n"
  HF build reader.howl | Out-Null; HF build escape.howl | Out-Null
  $DataDir = Join-Path $W "data"
  Check "read_file denied without a grant" { (HF run reader.hfbc).Code -ne 0 }
  Check "read_file allowed with filesystem:read=<data dir>" { (HF run --allow-caps "filesystem:read=$DataDir" reader.hfbc).Out -eq "fixture text" }
  Check "read outside the granted root denied" { (HF run --allow-caps "filesystem:read=$DataDir" escape.hfbc).Code -ne 0 }

  # A process:exec= grant needs the canonical absolute path; HowlFrame strings unquote like Go, so double each \.
  $Prog = (Get-Item (Join-Path $env:SystemRoot "System32\whoami.exe")).FullName
  Write-Source "runner.howl" ("(cli_app`n  (print (bytes_to_string (exec `"" + $Prog.Replace("\", "\\") + "`"))))`n")
  HF build runner.howl | Out-Null
  Check "exec denied without a grant" { (HF run runner.hfbc).Code -ne 0 }
  Check "bare process grant does not authorize exec" { (HF run --allow-caps process runner.hfbc).Code -ne 0 }
  Check "process:exec=$Prog authorizes exactly that program" {
    $r = HF run --allow-caps "process:exec=$Prog" runner.hfbc
    $r.Code -eq 0 -and $r.Out.Contains("\")
  }

  Write-Source "spin.howl" "(cli_app`n  (let (i 0)`n    (while true`n      (set i (+ i 1)))))`n"
  HF build spin.howl | Out-Null
  $errFile = Join-Path $W "spin.err"
  $p = Start-Process -FilePath $HFExe -ArgumentList "run", "--max-instructions", "10000", "spin.hfbc" -WorkingDirectory $W `
    -NoNewWindow -PassThru -RedirectStandardError $errFile -RedirectStandardOutput (Join-Path $W "spin.out")
  $null = $p.Handle   # keeps ExitCode readable after exit
  $done = $p.WaitForExit(60000)
  if (-not $done) { $p.Kill(); $rc = "killed" } else { $rc = $p.ExitCode }
  Check "unbounded loop stopped by --max-instructions (exit $rc, not killed)" {
    $done -and $rc -ne 0 -and (Get-Content -Raw $errFile).Contains("LIMIT_EXCEEDED")
  }
} finally { Pop-Location }

Out-Line ""
Out-Line "- Executable: ``$ExeSha``"
$sig = Get-AuthenticodeSignature $HFExe -ErrorAction SilentlyContinue
if ($sig) { Out-Line "- Authenticode: $($sig.Status) (information only; the release is unsigned)" }
Out-Line ""
if ($script:nfail -eq 0) { Out-Line "RESULT: ALL PASS ($script:npass checks)" } else { Out-Line "RESULT: FAILURES ($script:nfail of $($script:npass + $script:nfail) checks failed)" }
if ($env:GITHUB_STEP_SUMMARY) { Get-Content -Raw $Results | Add-Content -Path $env:GITHUB_STEP_SUMMARY }
if ($script:nfail -eq 0) { exit 0 } else { exit 1 }
