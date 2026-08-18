-- ============================================================================
--  Liquidations — SDL Rates
--  calculator-prices · 002 · operator identity on liquidations_sdl_rates
-- ----------------------------------------------------------------------------
--  The same three columns as the components table in file-compiler, and for a
--  concrete reason: this database CANNOT join against public.agents, which lives
--  in the other one. The screen reads this table, so the name has to be here.
--
--  It is the same choice the STR tables make: operator_name is stored resolved,
--  not looked up.
--
--  The market is what tells apart the rows that share a legal entity — Celsia
--  Valle from Celsia Tolima, EEP Pereira from EEP Cartago. Four rows that would
--  otherwise read as two duplicated pairs.
-- ============================================================================

ALTER TABLE public.liquidations_sdl_rates
  ADD COLUMN IF NOT EXISTS operator_name TEXT,
  ADD COLUMN IF NOT EXISTS agent_code    TEXT,
  ADD COLUMN IF NOT EXISTS market        TEXT;

COMMENT ON COLUMN public.liquidations_sdl_rates.operator_name IS
  'Legal name of the operator, resolved against public.agents in the file-compiler database. Stored and not joined: the two databases are separate. Same source and filter as the STR tables.';
COMMENT ON COLUMN public.liquidations_sdl_rates.agent_code IS
  'Agent code from the network-use file. Two operators may share it (EEPD, EPSD): the market is what separates them.';
COMMENT ON COLUMN public.liquidations_sdl_rates.market IS
  'Trading market of the row. Required to distinguish operators that share a legal entity.';
