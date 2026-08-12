-- ============================================================================
--  Liquidations — STR Charges
--  002 · Seed — network operators + their BalanceSTR column codes
-- ----------------------------------------------------------------------------
--  str_column_codes taken from the parser homologation (insumos-str.ts).
--  AIRE has two columns (CSID, CSSD) that are summed.
--  netsuite_vendor_id stays NULL: Finance fills it per operator.
--  Idempotent (ON CONFLICT on code).
-- ============================================================================

SET search_path TO liquidations_str, public;

INSERT INTO network_operators (id, code, name, str_column_codes) VALUES
  (gen_random_uuid()::text, 'AFINIA',         'Afinia',          ARRAY['CMMD']),
  (gen_random_uuid()::text, 'AIRE',           'Aire',            ARRAY['CSID','CSSD']),
  (gen_random_uuid()::text, 'ENELAR',         'Enelar',          ARRAY['ENID']),
  (gen_random_uuid()::text, 'CHEC',           'CHEC',            ARRAY['CHCD']),
  (gen_random_uuid()::text, 'CEDENAR',        'Cedenar',         ARRAY['CDND']),
  (gen_random_uuid()::text, 'CENS',           'CENS',            ARRAY['CNSD']),
  (gen_random_uuid()::text, 'ESSA',           'ESSA',            ARRAY['ESSD']),
  (gen_random_uuid()::text, 'ELECTROCAQUETA', 'Electrocaquetá',  ARRAY['CQTD']),
  (gen_random_uuid()::text, 'ELECTROHUILA',   'Electrohuila',    ARRAY['HLAD']),
  (gen_random_uuid()::text, 'EMSA',           'EMSA',            ARRAY['EMSD']),
  (gen_random_uuid()::text, 'EBSA',           'EBSA',            ARRAY['EBSD']),
  (gen_random_uuid()::text, 'ENERCA',         'Enerca',          ARRAY['CASD']),
  (gen_random_uuid()::text, 'EEP_PEREIRA',    'EEP Pereira',     ARRAY['EEPD']),
  (gen_random_uuid()::text, 'BAJO_PUTUMAYO',  'Bajo Putumayo',   ARRAY['EBPD']),
  (gen_random_uuid()::text, 'CELSIA_VALLE',   'Celsia Valle',    ARRAY['EPSD']),
  (gen_random_uuid()::text, 'PUTUMAYO',       'Putumayo',        ARRAY['EPTD']),
  (gen_random_uuid()::text, 'EDEQ',           'EDEQ',            ARRAY['EDQD']),
  (gen_random_uuid()::text, 'ENERGUAVIARE',   'Energuaviare',    ARRAY['EGVD']),
  (gen_random_uuid()::text, 'DISPAC',         'Dispac',          ARRAY['EDPD']),
  (gen_random_uuid()::text, 'EPM',            'EPM',             ARRAY['EPMD']),
  (gen_random_uuid()::text, 'EMCALI',         'Emcali',          ARRAY['EMID']),
  (gen_random_uuid()::text, 'CEO',            'CEO',             ARRAY['CEOD']),
  (gen_random_uuid()::text, 'ENEL',           'Enel',            ARRAY['ENDD'])
ON CONFLICT (code) DO UPDATE SET
  name             = EXCLUDED.name,
  str_column_codes = EXCLUDED.str_column_codes,
  updated_at       = now();
