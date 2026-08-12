-- ============================================================================
--  Liquidations — STR Charges
--  000 · Schema
-- ----------------------------------------------------------------------------
--  Run this connected to the existing BIA-BI database.
--  Creates the `liquidations_str` schema that holds the module tables.
--  Files 001/002 assume this schema (via SET search_path).
-- ============================================================================

CREATE SCHEMA IF NOT EXISTS liquidations_str;

COMMENT ON SCHEMA liquidations_str IS
  'STR charges ingestion (Liquidations): network_operators + str_charges.';
