-- ============================================================================
--  Liquidations — SDL Rates
--  file-compiler · 001 · liquidations_sdl_inputs
-- ----------------------------------------------------------------------------
--  Everything that feeds the SDL rate calculation, one row per network operator.
--
--    33 files → parser → liquidations_sdl_inputs   (here, the components)
--                     → calculator-prices          (the ten rates)
--
--  Per period XM publishes 33 files in two formats:
--    · 12 "Cargos ADD" — LiquidacionDefinitivos{Area}Nivel{N}_*.xlsx.
--      One charge per distribution area and voltage level, the same for every
--      operator of that area.
--    · 21 "Uso de la red" — Cargo_Cobro_Uso_Red-Definitivo{CODE}-*.xlsx.
--      DT1/2/3, CDI, CDN4 and PR1/2/3, per operator.
--
--  ── Why both sets of DT are stored ───────────────────────────────────────
--  The NT that goes into the formula comes from ONE of them, depending on the
--  operator: the area's ADD charge for the 14 "ADD" operators, its own file for
--  the 7 "USO" ones. Both are kept anyway, so a rate can be audited: seeing
--  dt1 and dt1_add differ is expected, not a data error.
--
--  ── The period does NOT come from the files ──────────────────────────────
--  It is picked in the Uploads screen and applies to the whole batch. The ADD
--  files and the network-use files can be from different months, and that is
--  correct: that is how the calculation works.
--
--  ── APPEND-ONLY ─────────────────────────────────────────────────────────
--  Nothing is replaced or deleted. Every upload inserts new rows with its own
--  `load_id` and `created_at`, so this single table holds both the current
--  values and the full history. Readers take the most recent row per
--  (period, operator_code); that rule lives in one place in the repository.
--
--  Requires PostgreSQL 13+ (built-in gen_random_uuid()).
-- ============================================================================

CREATE TABLE IF NOT EXISTS public.liquidations_sdl_inputs (
  id                TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  load_id           TEXT NOT NULL,
  period            TEXT NOT NULL,             -- "YYYY-MM"
  operator_code     TEXT NOT NULL,             -- CENS, CHEC, EPM…

  -- Only for operators whose NT comes from the ADD files. NULL for the rest.
  distribution_area TEXT,

  -- From the ADD file of the operator's area. NULL when it does not apply.
  dt1_add           NUMERIC(24, 12),
  dt2_add           NUMERIC(24, 12),
  dt3_add           NUMERIC(24, 12),

  -- From the operator's own network-use file.
  dt1               NUMERIC(24, 12) NOT NULL,
  dt2               NUMERIC(24, 12) NOT NULL,
  dt3               NUMERIC(24, 12) NOT NULL,
  cdi               NUMERIC(24, 12) NOT NULL,
  cdn4              NUMERIC(24, 12) NOT NULL,

  -- Recognised losses, as FRACTIONS. See the CHECK below.
  pr1               NUMERIC(24, 12) NOT NULL,
  pr2               NUMERIC(24, 12) NOT NULL,
  pr3               NUMERIC(24, 12) NOT NULL,

  source_files      TEXT,
  created_by        TEXT,
  created_by_id     TEXT,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

  CONSTRAINT uq_liquidations_sdl_inputs_load_operator
    UNIQUE (load_id, operator_code),

  -- The losses must be fractions: 12.55% is stored as 0.1255.
  --
  -- This is the single most valuable guard in the table. The formula divides by
  -- (1 - PR), so a value of 12.55 would make the divisor negative and every
  -- active rate absurd — while nothing fails and the numbers still look like
  -- numbers. In XM's files the cell holds the fraction and the "%" is only
  -- Excel formatting, so a percentage here means something read it wrong.
  CONSTRAINT ck_liquidations_sdl_inputs_pr_are_fractions
    CHECK (pr1 >= 0 AND pr1 < 1 AND pr2 >= 0 AND pr2 < 1 AND pr3 >= 0 AND pr3 < 1),

  CONSTRAINT ck_liquidations_sdl_inputs_area
    CHECK (distribution_area IS NULL
           OR distribution_area IN ('CENTRO', 'OCCIDENTE', 'ORIENTE', 'SUR')),

  -- Either the operator takes its NT from the ADD files — and then it has an
  -- area AND the three charges — or it takes it from its own file and has
  -- neither. A half-filled row means the parser lost something on the way.
  CONSTRAINT ck_liquidations_sdl_inputs_add_is_complete
    CHECK ((distribution_area IS NULL) = (dt1_add IS NULL)
       AND (dt1_add IS NULL) = (dt2_add IS NULL)
       AND (dt2_add IS NULL) = (dt3_add IS NULL))
);

COMMENT ON TABLE public.liquidations_sdl_inputs IS
  'Liquidations/SDL rates: the components extracted from the 33 files XM publishes per period, one row per network operator. Append-only: holds both current values and full history. The ten rates are computed from these and stored in the calculator-prices database.';
COMMENT ON COLUMN public.liquidations_sdl_inputs.load_id IS
  'Upload identifier. Shared with the mirror table in calculator-prices so any rate can be traced back to the components that produced it.';
COMMENT ON COLUMN public.liquidations_sdl_inputs.period IS
  'Settlement period (YYYY-MM) selected in the Uploads screen. Applies to every file in the batch, regardless of the month named in each file: the ADD files and the network-use files may belong to different months by design.';
COMMENT ON COLUMN public.liquidations_sdl_inputs.distribution_area IS
  'Distribution area whose ADD charge feeds this operator NT. NULL when the operator takes its NT from its own network-use file.';
COMMENT ON COLUMN public.liquidations_sdl_inputs.dt1_add IS
  'Network use charge for voltage level 1 from the area ADD file. The same value for every operator of that area. NULL when it does not apply.';
COMMENT ON COLUMN public.liquidations_sdl_inputs.dt1 IS
  'Network use charge for voltage level 1 from the operator own file. Stored even when the calculation uses dt1_add instead: keeping both is what makes a rate auditable.';
COMMENT ON COLUMN public.liquidations_sdl_inputs.pr1 IS
  'Recognised losses at voltage level 1, as a FRACTION (0.1255 = 12.55%). Zero is a valid value and appears in real data.';
COMMENT ON COLUMN public.liquidations_sdl_inputs.source_files IS
  'Files that produced this row, comma separated: the operator network-use file plus the three ADD files of its area when they apply.';
COMMENT ON COLUMN public.liquidations_sdl_inputs.created_by IS
  'Display name of whoever confirmed the upload, as sent by the front end. Not an identity check: see created_by_id.';
COMMENT ON COLUMN public.liquidations_sdl_inputs.created_by_id IS
  'User id taken from the x-user-id header, server side. This is the trustworthy one.';

-- Supports the "current values" read: filter by period and operator, keep the
-- most recent row.
CREATE INDEX IF NOT EXISTS idx_liquidations_sdl_inputs_current
  ON public.liquidations_sdl_inputs (period, operator_code, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_liquidations_sdl_inputs_load
  ON public.liquidations_sdl_inputs (load_id);
-- Uploads history: by period, most recent first.
CREATE INDEX IF NOT EXISTS idx_liquidations_sdl_inputs_history
  ON public.liquidations_sdl_inputs (period, created_at DESC);
