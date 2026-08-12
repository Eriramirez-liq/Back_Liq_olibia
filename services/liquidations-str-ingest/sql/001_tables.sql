-- ============================================================================
--  Liquidations — STR Charges
--  001 · Tables (PostgreSQL 13+)
-- ----------------------------------------------------------------------------
--  Flow:
--    Excel BalanceSTR*.xlsx  →  extraction (parser)  →  str_charges  →  NetSuite
--                                                                      (purchase orders)
--
--  str_charges is the table that travels to the DB and then to NetSuite: it
--  holds ONLY the network operator, the consumption period and the exact
--  amount to pay.
--
--  Requires PostgreSQL 13+ (built-in gen_random_uuid()). On older versions,
--  run:  CREATE EXTENSION IF NOT EXISTS pgcrypto;
-- ============================================================================

SET search_path TO liquidations_str, public;

-- ─────────────────────────────────────────────────────────────────────────
--  network_operators  (grid operator catalog — "Operador de Red")
--  - str_column_codes: column codes in the BalanceSTR files that belong to
--    this operator. Extraction SUMS them. e.g. AIRE = {CSID, CSSD}.
--    (Previously the hardcoded HOMOLOGACION dictionary in the parser.)
--  - netsuite_vendor_id: vendor internalId used to create the purchase order.
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS network_operators (
  id                 TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  code               TEXT NOT NULL UNIQUE,          -- AFINIA, AIRE, ...
  name               TEXT NOT NULL,
  str_column_codes   TEXT[] NOT NULL DEFAULT '{}',  -- Excel column codes
  netsuite_vendor_id TEXT,                          -- for the purchase order
  active             BOOLEAN NOT NULL DEFAULT true,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─────────────────────────────────────────────────────────────────────────
--  str_charges  (amount to pay — load target and source of the purchase order)
--  One row per (operator, consumption period). Stores the breakdown:
--    · invoice_amount   = sum from the "factu" file  (sheets BalSTR01 + BalSTR02)
--    · reinvoice_amount = sum from the "refactu" file (sheets *_Ajuste)
--    · amount_payable   = invoice_amount + reinvoice_amount  (GENERATED column)
--  Amounts may be negative (adjustments/reinvoices). amount_payable is what is
--  sent to NetSuite as a purchase order.
--
--  UNIQUE(operator_id, consumption_period): a single record per operator and
--  period. Re-loading the same period is an UPSERT (overwrites invoice/reinvoice;
--  amount_payable is recomputed automatically).
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS str_charges (
  id                 TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  operator_id        TEXT NOT NULL,
  consumption_period TEXT NOT NULL,                     -- "YYYY-MM"
  invoice_amount     NUMERIC(18, 2) NOT NULL DEFAULT 0, -- "factu" file
  reinvoice_amount   NUMERIC(18, 2) NOT NULL DEFAULT 0, -- "refactu" file (_Ajuste)
  amount_payable     NUMERIC(18, 2)
                     GENERATED ALWAYS AS (invoice_amount + reinvoice_amount) STORED,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT fk_str_charge_operator
    FOREIGN KEY (operator_id) REFERENCES network_operators (id) ON DELETE RESTRICT,
  CONSTRAINT uq_str_charge_operator_period UNIQUE (operator_id, consumption_period)
);

CREATE INDEX IF NOT EXISTS idx_str_charges_period   ON str_charges (consumption_period);
CREATE INDEX IF NOT EXISTS idx_str_charges_operator ON str_charges (operator_id);

-- ─────────────────────────────────────────────────────────────────────────
--  (Optional — NOT created) NetSuite submission tracking.
--  If you later need to track the purchase order created for each charge, add
--  a separate table so str_charges stays minimal:
--
--    CREATE TABLE str_netsuite_submissions (
--      id            TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
--      charge_id     TEXT NOT NULL REFERENCES str_charges (id),
--      status        TEXT NOT NULL,          -- PENDING | PROCESSED | ERROR
--      po_number     TEXT,
--      submitted_at  TIMESTAMPTZ,
--      error_message TEXT
--    );
-- ─────────────────────────────────────────────────────────────────────────
