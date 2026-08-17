-- ============================================================================
--  Liquidations — STR Charges
--  file-compiler · 001 · liquidations_str_inputs
-- ----------------------------------------------------------------------------
--  Raw values extracted from the BalanceSTR*.xlsx files, with no consolidation:
--
--    BalanceSTR*.xlsx → parser → liquidations_str_inputs   (here, raw)
--                             → calculator-prices          (amount payable)
--
--  That is why there is no payable column: the invoice and each adjustment are
--  kept apart. The sum lives in the other database, linked by `load_id`.
--
--  ── APPEND-ONLY ──────────────────────────────────────────────────────────
--  Nothing is replaced or deleted. Every upload inserts new rows with its own
--  `load_id` and `created_at`, so this single table holds BOTH the current
--  values and the full history. Re-uploading a period does not overwrite the
--  previous one.
--
--  Readers must take the most recent row per (period, operator_code). That rule
--  lives in `lib/cargos-str.ts` — every consumer goes through it.
--
--  Consequences of the model:
--   · No `updated_at`: an inserted row is never modified.
--   · If an upload brings fewer operators than the previous one, the missing
--     ones keep the value from the earlier upload. That is intended.
--   · An operator uploaded by mistake stays current until another upload
--     supersedes it. It is not deleted: it is corrected by re-uploading.
--
--  Requires PostgreSQL 13+ (built-in gen_random_uuid()). On older versions:
--    CREATE EXTENSION IF NOT EXISTS pgcrypto;
-- ============================================================================

CREATE TABLE IF NOT EXISTS public.liquidations_str_inputs (
  id                 TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  load_id            TEXT NOT NULL,
  period             TEXT NOT NULL,                     -- "YYYY-MM"
  operator_code      TEXT NOT NULL,                     -- AFINIA, AIRE, CHEC…
  invoice_amount     NUMERIC(18, 2) NOT NULL DEFAULT 0, -- "factu" file
  reinvoice_1_amount NUMERIC(18, 2),                    -- oldest adjustment
  reinvoice_2_amount NUMERIC(18, 2),
  reinvoice_3_amount NUMERIC(18, 2),                    -- newest adjustment
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_liquidations_str_inputs_load_operator UNIQUE (load_id, operator_code)
);

COMMENT ON TABLE public.liquidations_str_inputs IS
  'Liquidations/STR charges: raw values extracted from the BalanceSTR files, not consolidated. Append-only: holds both current values and full history. The amount payable is computed and stored in the calculator-prices database.';
COMMENT ON COLUMN public.liquidations_str_inputs.load_id IS
  'Upload identifier. Shared with the mirror table in calculator-prices so any payable amount can be traced back to the inputs that produced it.';
COMMENT ON COLUMN public.liquidations_str_inputs.period IS
  'Settlement period (YYYY-MM) selected in the Uploads module filter. Applies to every file in the batch, regardless of the month named in an adjustment file.';
COMMENT ON COLUMN public.liquidations_str_inputs.operator_code IS
  'Network operator, from homologating the Excel column codes, which are public.agents codes (e.g. CHCD -> CHEC). Agents belonging to the same operator are already summed (AIRE = CSID + CSSD).';
COMMENT ON COLUMN public.liquidations_str_inputs.invoice_amount IS
  'Value extracted from the TipoFactu file (sheets BalSTR01 + BalSTR02).';
COMMENT ON COLUMN public.liquidations_str_inputs.reinvoice_1_amount IS
  'Oldest adjustment in the batch (TipoReFactu file, *_Ajuste sheets). NULL when that file was not uploaded; 0 when it was and the operator had zero. May be negative.';

-- Supports the "current values" read: filter by period and operator, keep the
-- most recent row.
CREATE INDEX IF NOT EXISTS idx_liquidations_str_inputs_current
  ON public.liquidations_str_inputs (period, operator_code, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_liquidations_str_inputs_load
  ON public.liquidations_str_inputs (load_id);
