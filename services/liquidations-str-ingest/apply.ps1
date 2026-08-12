# ============================================================================
#  Liquidations — STR Charges
#  apply.ps1 · Crea el schema `liquidations_str` y sus tablas en Postgres.
# ----------------------------------------------------------------------------
#  Credenciales: lee $env:DATABASE_URL o un archivo .env local (KEY=VALUE).
#  Requisito: `psql` en el PATH.
#  Uso:
#    1) Copiá .env.example a .env y completá DATABASE_URL
#    2) ./apply.ps1
#  o directo:  $env:DATABASE_URL="postgres://..."; ./apply.ps1
# ============================================================================
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $MyInvocation.MyCommand.Path

# Cargar .env si existe (no pisa variables ya definidas en el entorno).
$envFile = Join-Path $root '.env'
if (Test-Path $envFile) {
  Get-Content $envFile | Where-Object { $_ -match '^\s*[^#].*=' } | ForEach-Object {
    $parts = $_ -split '=', 2
    $key = $parts[0].Trim()
    $val = $parts[1].Trim()
    if (-not [System.Environment]::GetEnvironmentVariable($key)) {
      [System.Environment]::SetEnvironmentVariable($key, $val)
    }
  }
}

if (-not $env:DATABASE_URL -and -not $env:PGDATABASE) {
  throw "Falta DATABASE_URL (o variables PG*). Copiá .env.example a .env y completá."
}
if (-not (Get-Command psql -ErrorAction SilentlyContinue)) {
  throw "psql no está en el PATH. Instalá el cliente de PostgreSQL."
}

$sql   = Join-Path $root 'sql'
$files = @('000_schema.sql', '001_tables.sql', '002_seed_operators.sql')

Write-Host "Aplicando esquema STR a la base BIA-BI..." -ForegroundColor Cyan
$target = if ($env:DATABASE_URL) { $env:DATABASE_URL } else { '' }

foreach ($f in $files) {
  $path = Join-Path $sql $f
  Write-Host "  → $f"
  if ($target) {
    psql $target -v ON_ERROR_STOP=1 -f $path
  } else {
    psql -v ON_ERROR_STOP=1 -f $path   # usa PGHOST/PGDATABASE/... del entorno
  }
  if ($LASTEXITCODE -ne 0) { throw "psql falló en $f (exit $LASTEXITCODE)." }
}

Write-Host "OK - schema liquidations_str creado: network_operators + str_charges (23 operadores)." -ForegroundColor Green
