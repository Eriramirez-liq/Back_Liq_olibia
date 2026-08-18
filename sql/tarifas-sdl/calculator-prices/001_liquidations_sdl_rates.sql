-- ============================================================================
--  Liquidations — SDL Rates
--  calculator-prices · 001 · liquidations_sdl_rates
-- ----------------------------------------------------------------------------
--  The ten SDL rates of each network operator per period: five active and five
--  reactive, one per voltage level and asset ownership combination that exists.
--
--  Only five combinations exist, not nine: level 1 splits by asset ownership
--  (operator, shared, user) and levels 2 and 3 only have a user rate. That is
--  why the rates are columns and not rows — the shape is fixed and known.
--
--  ── Where the numbers come from ──────────────────────────────────────────
--  From liquidations_sdl_inputs in the file-compiler database, same `load_id`:
--
--    ACTIVE                              REACTIVE
--      level 1 operator  = NT1 - CDN4/(1-PR1)     NT1
--      level 1 shared    = (…) - CDI*0.5          NT1 - CDI*0.5
--      level 1 user      = (…) - CDI              NT1 - CDI
--      level 2 user      = NT2 - CDN4/(1-PR2)     NT2
--      level 3 user      = NT3 - CDN4/(1-PR3)     NT3
--
--  NT comes from the area ADD charge or from the operator own file depending on
--  the operator; CDI, CDN4 and PR always from the operator own file.
--
--  ── Full precision on purpose ────────────────────────────────────────────
--  The screen shows two decimals, but rounding here would break the audit:
--  recomputing from the stored components would no longer reproduce the stored
--  rate. Rounding belongs to the presentation layer.
--
--  ── APPEND-ONLY ─────────────────────────────────────────────────────────
--  Same model as the components table: nothing is replaced, readers take the
--  most recent row per (period, operator_code).
-- ============================================================================

CREATE TABLE IF NOT EXISTS public.liquidations_sdl_rates (
  id                        TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  load_id                   TEXT NOT NULL,
  period                    TEXT NOT NULL,     -- "YYYY-MM"
  operator_code             TEXT NOT NULL,

  active_level_1_operator   NUMERIC(24, 12) NOT NULL,
  active_level_1_shared     NUMERIC(24, 12) NOT NULL,
  active_level_1_user       NUMERIC(24, 12) NOT NULL,
  active_level_2_user       NUMERIC(24, 12) NOT NULL,
  active_level_3_user       NUMERIC(24, 12) NOT NULL,

  reactive_level_1_operator NUMERIC(24, 12) NOT NULL,
  reactive_level_1_shared   NUMERIC(24, 12) NOT NULL,
  reactive_level_1_user     NUMERIC(24, 12) NOT NULL,
  reactive_level_2_user     NUMERIC(24, 12) NOT NULL,
  reactive_level_3_user     NUMERIC(24, 12) NOT NULL,

  created_at                TIMESTAMPTZ NOT NULL DEFAULT now(),

  CONSTRAINT uq_liquidations_sdl_rates_load_operator
    UNIQUE (load_id, operator_code)
);

COMMENT ON TABLE public.liquidations_sdl_rates IS
  'Liquidations/SDL rates: the ten rates of each network operator per period, computed from liquidations_sdl_inputs in the file-compiler database. Append-only: holds both current values and full history. Full precision is kept so that recomputing from the components reproduces these values exactly.';
COMMENT ON COLUMN public.liquidations_sdl_rates.load_id IS
  'Upload identifier, shared with liquidations_sdl_inputs in file-compiler. It is what lets a rate be traced back to the components and the files that produced it.';
COMMENT ON COLUMN public.liquidations_sdl_rates.period IS
  'Settlement period (YYYY-MM) selected in the Uploads screen, not read from the file names.';
COMMENT ON COLUMN public.liquidations_sdl_rates.active_level_1_operator IS
  'Active rate, voltage level 1, operator-owned assets: NT1 minus CDN4/(1-PR1). Full precision; round only for display.';
COMMENT ON COLUMN public.liquidations_sdl_rates.reactive_level_1_operator IS
  'Reactive rate, voltage level 1, operator-owned assets: the NT1 itself, with nothing subtracted. Useful when auditing: it shows which file the NT was taken from.';

-- Supports the "current values" read.
CREATE INDEX IF NOT EXISTS idx_liquidations_sdl_rates_current
  ON public.liquidations_sdl_rates (period, operator_code, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_liquidations_sdl_rates_load
  ON public.liquidations_sdl_rates (load_id);
