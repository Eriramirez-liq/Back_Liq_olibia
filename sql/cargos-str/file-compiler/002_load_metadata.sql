-- ============================================================================
--  Liquidations — STR Charges
--  file-compiler · 002 · upload metadata on liquidations_str_inputs
-- ----------------------------------------------------------------------------
--  Adds who uploaded and which files were uploaded.
--
--  ── Why here and not in a separate "uploads" table ───────────────────────
--  This module keeps ONE table per database on purpose. The upload is already
--  identified by `load_id`, and every row of an upload shares these values, so
--  grouping by `load_id` yields the upload history without a second table:
--
--    SELECT load_id, period, min(created_at), count(*), max(created_by)
--      FROM public.liquidations_str_inputs
--     GROUP BY load_id, period
--
--  The repetition is the price of not adding a table, and it is cheap: an
--  upload is 23 rows.
--
--  ── Why the upload history has to live here at all ───────────────────────
--  It used to live in Supabase, written by the TypeScript backend. That backend
--  is not deployed, and the module now writes only to the BIA databases, so
--  without this the Uploads screen has no history to show.
--
--  ── Nullable on purpose ──────────────────────────────────────────────────
--  Rows uploaded before this migration have no metadata, and the model is
--  append-only: they are not backfilled with invented values. The reader shows
--  them without a user, which is truthful.
-- ============================================================================

ALTER TABLE public.liquidations_str_inputs
  ADD COLUMN IF NOT EXISTS created_by    TEXT,
  ADD COLUMN IF NOT EXISTS created_by_id TEXT,
  ADD COLUMN IF NOT EXISTS source_files  TEXT;

COMMENT ON COLUMN public.liquidations_str_inputs.created_by IS
  'Display name of whoever confirmed the upload, as sent by the front end. Shown in the uploads history. Not an identity check: see created_by_id.';
COMMENT ON COLUMN public.liquidations_str_inputs.created_by_id IS
  'User id taken from the x-user-id header, server side. This is the trustworthy one; created_by is only for display.';
COMMENT ON COLUMN public.liquidations_str_inputs.source_files IS
  'Names of the uploaded .xlsx files, comma separated, in the order they were sent. Lets an amount be traced back to the file it came from.';

-- The uploads history reads by period, most recent first.
CREATE INDEX IF NOT EXISTS idx_liquidations_str_inputs_history
  ON public.liquidations_str_inputs (period, created_at DESC);
