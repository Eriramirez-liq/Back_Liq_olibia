#!/usr/bin/env bash
# ============================================================================
#  Liquidations — STR Charges
#  apply.sh · Crea el schema `liquidations_str` y sus tablas en Postgres.
# ----------------------------------------------------------------------------
#  Credenciales: lee DATABASE_URL del entorno o de un archivo .env (KEY=VALUE).
#  Requisito: `psql` en el PATH.
#  Uso:
#    1) cp .env.example .env  &&  editar DATABASE_URL
#    2) ./apply.sh
#  o directo:  DATABASE_URL="postgres://..." ./apply.sh
# ============================================================================
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Cargar .env si existe (no pisa variables ya definidas).
if [[ -f "$DIR/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$DIR/.env"
  set +a
fi

if [[ -z "${DATABASE_URL:-}" && -z "${PGDATABASE:-}" ]]; then
  echo "ERROR: falta DATABASE_URL (o variables PG*). Copiá .env.example a .env." >&2
  exit 1
fi
command -v psql >/dev/null 2>&1 || { echo "ERROR: psql no está en el PATH." >&2; exit 1; }

SQL="$DIR/sql"
FILES=(000_schema.sql 001_tables.sql 002_seed_operators.sql)

echo "Aplicando esquema STR a la base BIA-BI..."
for f in "${FILES[@]}"; do
  echo "  -> $f"
  if [[ -n "${DATABASE_URL:-}" ]]; then
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$SQL/$f"
  else
    psql -v ON_ERROR_STOP=1 -f "$SQL/$f"   # usa PGHOST/PGDATABASE/... del entorno
  fi
done

echo "OK - schema liquidations_str creado: network_operators + str_charges (23 operadores)."
