-- ============================================================================
--  Liquidations — STR Charges
--  calculator-prices · 001 · liquidations_str_charges
-- ----------------------------------------------------------------------------
--  The final figure: what each network operator is paid for a period. This is
--  the table that feeds the NetSuite purchase order.
--
--    amount_payable = invoice_amount + every adjustment in the batch
--
--  The breakdown (invoice and each adjustment separately) lives in the other
--  database, `file-compiler`, table `public.liquidations_str_inputs`, linked by
--  `load_id`.
--
--  ── APPEND-ONLY ──────────────────────────────────────────────────────────
--  Same as the inputs: nothing is replaced or deleted. Every upload inserts new
--  rows, so this single table holds both the current values and the history.
--  Readers take the most recent row per (period, operator_code) — that rule
--  lives in `lib/cargos-str.ts`.
--
--  Requires PostgreSQL 13+ (built-in gen_random_uuid()). On older versions:
--    CREATE EXTENSION IF NOT EXISTS pgcrypto;
-- ============================================================================

CREATE TABLE IF NOT EXISTS public.liquidations_str_charges (
  id             TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  load_id        TEXT NOT NULL,
  period         TEXT NOT NULL,                  -- "YYYY-MM"
  operator_code  TEXT NOT NULL,                  -- AFINIA, AIRE, CHEC…
  operator_name  TEXT NOT NULL,                  -- legal name, from agents.name
  amount_payable NUMERIC(18, 2) NOT NULL,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_liquidations_str_charges_load_operator UNIQUE (load_id, operator_code)
);

COMMENT ON TABLE public.liquidations_str_charges IS
  'Liquidations/STR charges: amount payable per operator and period (invoice + adjustments). Source of the NetSuite purchase order. Append-only; the raw breakdown lives in the file-compiler database, linked by load_id.';
COMMENT ON COLUMN public.liquidations_str_charges.load_id IS
  'Upload identifier. The same id exists in file-compiler.public.liquidations_str_inputs with the breakdown that produced this amount.';
COMMENT ON COLUMN public.liquidations_str_charges.period IS
  'Settlement period (YYYY-MM) selected in the Uploads module filter.';
COMMENT ON COLUMN public.liquidations_str_charges.operator_name IS
  'Legal name of the network operator, taken from public.agents.name in the file-compiler database (e.g. "CENTRAL HIDROELECTRICA DE CALDAS S.A. E.S.P."). Stored resolved because that catalog lives in another database.';
COMMENT ON COLUMN public.liquidations_str_charges.amount_payable IS
  'Invoice amount for the month plus every adjustment in the batch. May be negative when adjustments exceed the invoice.';

CREATE INDEX IF NOT EXISTS idx_liquidations_str_charges_current
  ON public.liquidations_str_charges (period, operator_code, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_liquidations_str_charges_load
  ON public.liquidations_str_charges (load_id);
