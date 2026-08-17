/* ============================================================================
 *  Liquidaciones — Cargos STR
 *  apply.js · Crea las tablas del módulo en las dos bases de BIA:
 *             file-compiler.liquidations_str_inputs   (insumo crudo)
 *             calculator-prices.liquidations_str_charges (valor a pagar)
 * ----------------------------------------------------------------------------
 *  Reemplaza al apply.js de services/liquidations-str-ingest, que apuntaba a
 *  bia-bi (base que se deja de usar).
 *
 *  Credenciales — se resuelven en este orden, sin que este script las lea nunca
 *  desde el chat ni las escriba a ningún lado:
 *
 *    1. FILE_COMPILER_DATABASE_URL / CALCULATOR_PRICES_DATABASE_URL
 *    2. Las mismas DB_HOST / DB_PORT / DB_USER / DB_PASSWORD que ya se usan
 *       para bia-bi (mismo RDS, mismo usuario), cambiando solo el nombre de
 *       la base. Ese nombre se puede fijar con FILE_COMPILER_DB_NAME y
 *       CALCULATOR_PRICES_DB_NAME.
 *
 *  Se leen del entorno o del .env de este repo (gitignored).
 *
 *  Uso:
 *    node sql/cargos-str/apply.js file-compiler
 *    node sql/cargos-str/apply.js calculator-prices
 *    node sql/cargos-str/apply.js all
 *    node sql/cargos-str/apply.js all --dry-run   (no se conecta)
 *    node sql/cargos-str/apply.js --list-dbs      (qué bases hay en el servidor)
 * ========================================================================== */
const fs = require('fs');
const path = require('path');

// `pg` no es dependencia de este repo (usa Prisma). Si no está en la raíz,
// caemos a la copia del dev-server STR antes de pedir un npm install.
let Client;
try {
  ({ Client } = require('pg'));
} catch {
  try {
    ({ Client } = require(path.join(
      __dirname, '..', '..', 'services', 'liquidations-str-ingest', 'dev-server', 'node_modules', 'pg',
    )));
  } catch {
    console.error('Falta el driver `pg`. Instalalo con:  npm install pg');
    process.exit(1);
  }
}

// Se leen los dos .env del workspace: el de este repo y el de olibia-web
// (donde viven las credenciales compartidas de las bases de BIA). El de este
// repo tiene prioridad. Ninguno se versiona.
const ENV_FILES = [
  path.join(__dirname, '..', '..', '.env'),
  path.join(__dirname, '..', '..', '..', 'olibia-web', '.env'),
];

const TARGETS = {
  'file-compiler': {
    urlVar: 'FILE_COMPILER_DATABASE_URL',
    dbVar: 'FILE_COMPILER_DB_NAME',
    // El servidor ya es el de dev (c4-rds-bia-dev): las bases NO llevan prefijo.
    dbDefault: 'file-compiler',
    dir: path.join(__dirname, 'file-compiler'),
    files: ['001_liquidations_str_inputs.sql'],
  },
  'calculator-prices': {
    urlVar: 'CALCULATOR_PRICES_DATABASE_URL',
    dbVar: 'CALCULATOR_PRICES_DB_NAME',
    dbDefault: 'calculator-prices',
    dir: path.join(__dirname, 'calculator-prices'),
    files: ['001_liquidations_str_charges.sql'],
  },
};

function parseEnv(file) {
  const out = {};
  if (!fs.existsSync(file)) return out;
  for (const raw of fs.readFileSync(file, 'utf8').split(/\r?\n/)) {
    const line = raw.trim();
    if (!line || line.startsWith('#')) continue;
    const i = line.indexOf('=');
    if (i === -1) continue;
    out[line.slice(0, i).trim()] = line.slice(i + 1).trim().replace(/^["']|["']$/g, '');
  }
  return out;
}

// El primer archivo que define una clave gana.
const fileEnv = ENV_FILES.reduceRight((acc, f) => Object.assign(acc, parseEnv(f)), {});
const getEnv = (k) => process.env[k] || fileEnv[k];

// Las bases file-compiler y calculator-prices usan un usuario distinto al de
// bia-bi (DB_USER/DB_PASSWORD), porque ese no tiene permisos ahí.
const dbUser = () => getEnv('DB_USER2') || getEnv('DB_USER');
const dbPassword = () => (getEnv('DB_USER2') ? getEnv('DB_PASSWORD2') : getEnv('DB_PASSWORD'));
const sslFor = (host) => (/^(localhost|127\.0\.0\.1)$/.test(host) ? false : { rejectUnauthorized: false });

/** Configuración de conexión + una etiqueta legible que NUNCA incluye la contraseña. */
function resolveConfig(target, dbNameOverride) {
  const url = getEnv(target.urlVar);
  if (url) {
    const host = (url.match(/@([^:/]+)/) || [])[1] || '';
    return {
      cfg: { connectionString: url, ssl: sslFor(host), connectionTimeoutMillis: 15000 },
      label: url.replace(/:\/\/([^:]+):[^@]+@/, '://$1:***@'),
      database: (url.match(/\/([^/?]+)(\?|$)/) || [])[1] || '',
    };
  }

  const host = getEnv('DB_HOST');
  const user = dbUser();
  const password = dbPassword();
  if (!host || !user || !password) {
    throw new Error(
      `Faltan credenciales. Definí ${target.urlVar}, o DB_HOST/DB_PORT/DB_USER2/DB_PASSWORD2 ` +
      'en el entorno o en alguno de los .env del workspace.',
    );
  }

  const database = dbNameOverride || getEnv(target.dbVar) || target.dbDefault;
  return {
    cfg: {
      host,
      port: Number(getEnv('DB_PORT') || 5432),
      user,
      password,
      database,
      ssl: sslFor(host),
      connectionTimeoutMillis: 15000,
    },
    label: `${host}:${getEnv('DB_PORT') || 5432}/${database} (usuario ${user})`,
    database,
  };
}

/** Lista las bases del servidor. Sirve cuando el nombre esperado no existe. */
async function listarBases() {
  const host = getEnv('DB_HOST');
  const user = dbUser();
  if (!host || !user) throw new Error('Faltan DB_HOST/DB_USER2 para listar las bases.');

  const client = new Client({
    host,
    port: Number(getEnv('DB_PORT') || 5432),
    user,
    password: dbPassword(),
    database: 'postgres',
    ssl: sslFor(host),
    connectionTimeoutMillis: 15000,
  });
  await client.connect();
  const r = await client.query(
    `SELECT datname, pg_catalog.pg_get_userbyid(datdba) AS owner
       FROM pg_database WHERE datistemplate = false ORDER BY datname`,
  );
  await client.end();
  return r.rows;
}

async function aplicar(nombre, dryRun) {
  const target = TARGETS[nombre];
  console.log(`\n===  ${nombre}  ===`);

  let resolved;
  try {
    resolved = resolveConfig(target);
  } catch (e) {
    console.error(`  ${e.message}`);
    return false;
  }

  console.log(`  Destino: ${resolved.label}`);

  if (dryRun) {
    let completo = true;
    for (const f of target.files) {
      const existe = fs.existsSync(path.join(target.dir, f));
      if (!existe) completo = false;
      console.log(`  ${existe ? '->' : '!!'} ${f}${existe ? '' : '  (NO ENCONTRADO)'}`);
    }
    return completo;
  }

  const client = new Client(resolved.cfg);
  // Los scripts 002/003 comunican por RAISE NOTICE/WARNING — sin esto no se ven.
  client.on('notice', (msg) => console.log(`     ${msg.severity}: ${msg.message}`));

  try {
    await client.connect();
  } catch (e) {
    console.error(`  No se pudo conectar: ${e.message}`);
    if (/does not exist|no existe/i.test(e.message)) {
      console.error(`  La base "${resolved.database}" no existe con ese nombre. Bases disponibles:`);
      try {
        for (const r of await listarBases()) console.error(`     ${r.datname}`);
        console.error(`  Fijá el nombre correcto con ${target.dbVar}=<nombre>`);
      } catch (e2) {
        console.error(`     (no se pudieron listar: ${e2.message})`);
      }
    }
    return false;
  }

  for (const f of target.files) {
    process.stdout.write(`  -> ${f} ... `);
    await client.query(fs.readFileSync(path.join(target.dir, f), 'utf8'));
    console.log('OK');
  }

  // ── Verificación ────────────────────────────────────────────────────────
  const objetos = await client.query(
    `SELECT table_name, table_type FROM information_schema.tables
      WHERE table_schema = 'public' AND table_name LIKE 'liquidations\\_%'
      ORDER BY table_type, table_name`,
  );
  for (const o of objetos.rows) {
    console.log(`     ${o.table_type === 'VIEW' ? 'vista' : 'tabla'}: public.${o.table_name}`);
  }

  if (nombre === 'file-compiler') {
    // No creamos catálogo propio: la homologación se resuelve contra
    // public.agents. Verificamos que las 24 abreviaturas del parser sigan
    // existiendo ahí y que cada operador tenga UN agente "OPERADOR DE RED",
    // que es de donde sale el nombre legal.
    const CODIGOS = ['CMMD', 'CSID', 'CSSD', 'ENID', 'CHCD', 'CDND', 'CNSD', 'ESSD', 'CQTD',
      'HLAD', 'EMSD', 'EBSD', 'CASD', 'EEPD', 'EBPD', 'EPSD', 'EPTD', 'EDQD', 'EGVD', 'EDPD',
      'EPMD', 'EMID', 'CEOD', 'ENDD'];
    const a = await client.query(
      `SELECT count(*)::int AS encontrados,
              count(*) FILTER (WHERE activity = 'OPERADOR DE RED')::int AS operadores_red
         FROM public.agents WHERE upper(trim(code)) = ANY($1)`,
      [CODIGOS],
    );
    const { encontrados, operadores_red } = a.rows[0];
    console.log(`  public.agents: ${encontrados}/24 abreviaturas presentes, ${operadores_red} con actividad OPERADOR DE RED`);
    if (encontrados < CODIGOS.length) {
      console.log('  ATENCIÓN: faltan abreviaturas en el catálogo de agentes — el parser no las encontraría.');
    }
  }

  await client.end();
  return true;
}

(async () => {
  const args = process.argv.slice(2);

  if (args.includes('--list-dbs')) {
    console.log('Bases en el servidor:');
    for (const r of await listarBases()) console.log(`  ${r.datname.padEnd(32)} owner: ${r.owner}`);
    return;
  }

  const dryRun = args.includes('--dry-run');
  const pedido = args.find((a) => !a.startsWith('--'));

  if (!pedido || (pedido !== 'all' && !TARGETS[pedido])) {
    console.error('Uso: node sql/cargos-str/apply.js <file-compiler|calculator-prices|all> [--dry-run]');
    console.error('     node sql/cargos-str/apply.js --list-dbs');
    process.exit(1);
  }

  const objetivos = pedido === 'all' ? Object.keys(TARGETS) : [pedido];
  let ok = true;
  for (const t of objetivos) ok = (await aplicar(t, dryRun)) && ok;

  console.log(ok ? '\nListo.' : '\nTerminó con pendientes — ver mensajes de arriba.');
  process.exit(ok ? 0 : 1);
})().catch((e) => {
  console.error('\nERROR:', e.message);
  process.exit(1);
});
