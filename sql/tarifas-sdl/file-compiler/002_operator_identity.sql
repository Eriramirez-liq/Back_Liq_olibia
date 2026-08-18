-- ============================================================================
--  Liquidations — SDL Rates
--  file-compiler · 002 · operator identity on liquidations_sdl_inputs
-- ----------------------------------------------------------------------------
--  Adds who the operator is, beyond the internal code.
--
--  ── Why the legal name is stored and not joined ──────────────────────────
--  It comes from `public.agents`, which lives in THIS database — but the rates
--  table lives in calculator-prices, so a join across the two is not possible.
--  Both tables carry the name for the same reason the STR tables do.
--
--  ── Why the market matters ───────────────────────────────────────────────
--  Two agents serve two markets each, and the business wants them as separate
--  rows:
--
--    EEPD  → Pereira (PEIM) and Cartago (CRCM)
--    EPSD  → Valle (VACM) and Tolima (TOLM)
--
--  Same legal entity, two markets. Without the market, the two Celsia rows —
--  and the two EEP rows — would both read "CELSIA COLOMBIA S.A. E.S.P." and
--  look like duplicates.
--
--  ── The agent code is the one from the file ──────────────────────────────
--  Cell B1 of each network-use file carries it, so no hand-written operator to
--  agent mapping is needed. One alias exists: Air-e's file says CSID, which in
--  `public.agents` is activity DISTRIBUCIÓN and named "… - INTERVENIDO"; the
--  current network operator is CSSD. The alias lives in the Go code and is
--  covered by a test.
--
--  Nullable on purpose: rows loaded before this migration have no identity and
--  are not backfilled with invented values.
-- ============================================================================

ALTER TABLE public.liquidations_sdl_inputs
  ADD COLUMN IF NOT EXISTS operator_name TEXT,
  ADD COLUMN IF NOT EXISTS agent_code    TEXT,
  ADD COLUMN IF NOT EXISTS market        TEXT;

COMMENT ON COLUMN public.liquidations_sdl_inputs.operator_name IS
  'Legal name of the operator, resolved against public.agents by agent_code filtering activity = OPERADOR DE RED. Same source and same filter as the STR tables, so the two modules always show the same name for the same operator.';
COMMENT ON COLUMN public.liquidations_sdl_inputs.agent_code IS
  'Agent code as it appears in cell B1 of the network-use file (EEPD, CSID, EPSD…). Kept for traceability: it is what the name was resolved from.';
COMMENT ON COLUMN public.liquidations_sdl_inputs.market IS
  'Trading market the row belongs to (PEREIRA, CARTAGO, VALLE DEL CAUCA, TOLIMA…). Two agents serve two markets each, so without this the rows of the same legal entity are indistinguishable.';
