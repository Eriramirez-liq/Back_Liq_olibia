-- ============================================================================
--  Liquidations — TC1
--  file-compiler · 001 · liquidations_tc1_inputs
-- ----------------------------------------------------------------------------
--  The technical border configuration each network operator reports (TC1),
--  one row per commercial border.
--
--    21 files → parser → liquidations_tc1_inputs   (here, and nowhere else)
--
--  ── No calculation, only storage ─────────────────────────────────────────
--  Unlike STR charges or SDL rates, TC1 produces no derived values: there is no
--  mirror table in calculator-prices. The file is normalised and stored. What
--  reads it downstream —reconciliation against Billing, congruence— crosses it
--  by `cod_frontera_comercial`.
--
--  ── Every column is TEXT, on purpose ─────────────────────────────────────
--  The CREG layout is a fixed sequence of fields, not a typed schema, and the
--  files arrive with values that only look numeric: latitude comes as
--  "35003989" (micro-degrees), altitude as "974", and several operators leave
--  fields empty rather than zero. Casting here would be the one place where the
--  original value can be lost, and "store what arrived" is the whole point of
--  this table. Whoever consumes it casts what it needs.
--
--  ── The parser runs in the BROWSER ───────────────────────────────────────
--  Not a whim: CELSIA_VALLE's file is 98 MB and 743,530 rows, and CHEC's 82 MB.
--  The parser filters by ID_COMERCIALIZADOR = 62371 (BIA), which leaves 156 rows
--  of those 743,530 — so what reaches this table is small, but only because the
--  filtering happens before the upload. Sending the raw file would mean pushing
--  98 MB through the gateway.
--
--  ── The period does NOT come from the file ───────────────────────────────
--  It is picked in the Uploads screen, exactly like STR and SDL. The TC1 file
--  itself carries no period column.
--
--  ── APPEND-ONLY ─────────────────────────────────────────────────────────
--  Nothing is replaced or deleted. The TypeScript version deleted the previous
--  rows of the same operator and period before inserting, on the grounds that
--  "TC1 is a snapshot of the current state". The snapshot reading is right; the
--  deleting is not — it threw away who loaded what and when.
--
--  So every upload inserts with its own `load_id`, and readers take the rows of
--  the most recent `load_id` per (period, operator_code). Note the difference
--  from STR and SDL: there the current row is the newest row, because there is
--  ONE row per period and operator. Here there are MANY —one per border— so
--  what is current is the newest LOAD, not the newest row. Picking row by row
--  would mix borders from two different uploads.
--
--  Requires PostgreSQL 13+ (built-in gen_random_uuid()).
-- ============================================================================

CREATE TABLE IF NOT EXISTS public.liquidations_tc1_inputs (
  id                           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  load_id                      TEXT NOT NULL,
  period                       TEXT NOT NULL,   -- "YYYY-MM", same as STR and SDL
  operator_code                TEXT NOT NULL,   -- CENS, CHEC, EPM…

  -- ── The 33 canonical TC1 columns, in the CREG order ────────────────────
  --
  -- The order matters and is not decoration: the parser maps the file's columns
  -- to these BY POSITION, because operators name their headers differently
  -- (CEDENAR types "FROTERA", EMCALI calls the border "SIC", ENEL truncates
  -- "Nivel de T"). Renaming or reordering these breaks that mapping.
  niu                          TEXT,
  codigo_de_conexion           TEXT,
  tipo_de_conexion             TEXT,
  nivel_de_tension             TEXT,
  nivel_de_tension_primario    TEXT,
  porc_propiedad_del_activo    TEXT,
  conexion_red                 TEXT,
  id_comercializador           TEXT,
  id_mercado                   TEXT,
  grupo_de_calidad             TEXT,
  cod_frontera_comercial       TEXT NOT NULL,
  codigo_circuito_o_linea      TEXT,
  codigo_transformador         TEXT,
  codigo_dane_niu              TEXT,
  ubicacion                    TEXT,
  direccion                    TEXT,
  condicion_especial           TEXT,
  tipo_area_especial           TEXT,
  codigo_area_especial         TEXT,
  estrato_id                   TEXT,
  altitud                      TEXT,
  longitud                     TEXT,
  latitud                      TEXT,
  autogenerador                TEXT,
  exporta_energia              TEXT,
  potencia                     TEXT,
  tipo_generacion              TEXT,
  codigo_frontera_auto_gen     TEXT,
  inicio_operacion             TEXT,
  contrato_respaldo            TEXT,
  capacidad_contrato_respaldo  TEXT,
  ciclo                        TEXT,
  nodo                         TEXT,

  source_file                  TEXT,
  created_by                   TEXT,
  created_by_id                TEXT,
  created_at                   TIMESTAMPTZ NOT NULL DEFAULT now(),

  -- Period as the rest of the module writes it. Without this a "2-2026" —the
  -- shape the old Metabase export used— would slip in and stop sorting and
  -- joining with the other two tables.
  CONSTRAINT ck_liquidations_tc1_inputs_period
    CHECK (period ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'),

  -- The border is the crossing key with Billing. A blank one is a row nobody
  -- can reconcile, so it does not get in.
  CONSTRAINT ck_liquidations_tc1_inputs_border_not_blank
    CHECK (btrim(cod_frontera_comercial) <> '')
);

-- Serves the read rule: newest load per period and operator, then all its rows.
CREATE INDEX IF NOT EXISTS ix_liquidations_tc1_inputs_current
  ON public.liquidations_tc1_inputs (period, operator_code, created_at DESC, id DESC);

-- Reconciliation and congruence cross by border.
CREATE INDEX IF NOT EXISTS ix_liquidations_tc1_inputs_border
  ON public.liquidations_tc1_inputs (cod_frontera_comercial);

-- The upload history reads by load.
CREATE INDEX IF NOT EXISTS ix_liquidations_tc1_inputs_load
  ON public.liquidations_tc1_inputs (load_id);

COMMENT ON TABLE public.liquidations_tc1_inputs IS
  'Liquidations/TC1: the technical border configuration reported by each network operator, one row per commercial border. Stored as reported — no calculation and no mirror table. Append-only: readers take the rows of the most recent load_id per (period, operator_code).';
COMMENT ON COLUMN public.liquidations_tc1_inputs.load_id IS
  'Upload identifier. What makes a row current is belonging to the newest load of its period and operator, not being the newest row: there are many rows per period and operator, one per border, and mixing two uploads would mix borders.';
COMMENT ON COLUMN public.liquidations_tc1_inputs.period IS
  'Settlement period (YYYY-MM) picked in the Uploads screen. The TC1 file carries no period of its own.';
COMMENT ON COLUMN public.liquidations_tc1_inputs.cod_frontera_comercial IS
  'Commercial border. The crossing key with Billing, and the reason the parser resolves this column by name and not by position: CEDENAR types it "COD_FROTERA_COMERCIAL" and EMCALI calls it "SIC".';
COMMENT ON COLUMN public.liquidations_tc1_inputs.id_comercializador IS
  'Retailer id. The parser keeps only BIA rows (62371) when the file carries this column; when it does not, the file is assumed pre-filtered and every row is loaded.';
