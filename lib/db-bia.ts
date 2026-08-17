import { Pool, type PoolClient, type QueryResultRow } from "pg"

/**
 * Acceso a las bases de datos de BIA.
 *
 * El módulo de liquidaciones está migrando de Supabase a las bases propias de
 * BIA, módulo por módulo. Prisma no sirve para esto: es un datasource por
 * cliente generado, y acá hacen falta dos conexiones más además de la que ya
 * usa `lib/db.ts`. Por eso este archivo va con `pg` directo.
 *
 *   file-compiler      insumos crudos + catálogos de XM (public.agents)
 *   calculator-prices  resultados consolidados
 *
 * Es infraestructura compartida: los módulos que se migren después (SDL,
 * Balance, TC1, COT, Tarifas SDL) usan esto mismo. Ver `lib/fuentes-migradas.ts`
 * para saber qué fuentes ya están de este lado.
 */

export type BaseBia = "file-compiler" | "calculator-prices"

// ─── Configuración ──────────────────────────────────────────────────────────
// Estas bases usan un usuario distinto al de bia-bi: DB_USER2/DB_PASSWORD2.
// Si no están definidas, cae a DB_USER/DB_PASSWORD para no romper entornos
// que todavía no las tengan separadas.
const nombreBase: Record<BaseBia, string> = {
  "file-compiler": process.env.FILE_COMPILER_DB_NAME ?? "file-compiler",
  "calculator-prices": process.env.CALCULATOR_PRICES_DB_NAME ?? "calculator-prices",
}

function configuracion(base: BaseBia) {
  const host = process.env.DB_HOST
  const user = process.env.DB_USER2 ?? process.env.DB_USER
  const password = process.env.DB_USER2 ? process.env.DB_PASSWORD2 : process.env.DB_PASSWORD

  if (!host || !user || !password) {
    throw new Error(
      "Faltan credenciales de las bases de BIA: definí DB_HOST, DB_PORT, DB_USER2 y DB_PASSWORD2.",
    )
  }

  return {
    host,
    port: Number(process.env.DB_PORT ?? 5432),
    user,
    password,
    database: nombreBase[base],
    ssl: /^(localhost|127\.0\.0\.1)$/.test(host) ? undefined : { rejectUnauthorized: false },
    // El destino es una instancia de BIA (proceso largo), no serverless: un pool
    // chico y estable alcanza. `idleTimeoutMillis` libera conexiones ociosas para
    // no retener slots del RDS entre cargas.
    max: Number(process.env.BIA_DB_POOL_MAX ?? 5),
    idleTimeoutMillis: 30_000,
    connectionTimeoutMillis: 15_000,
  }
}

// Singleton por base, sobreviviendo al hot reload de Next en desarrollo —
// mismo patrón que `lib/db.ts` usa para Prisma.
const globalForBia = globalThis as unknown as { poolsBia?: Partial<Record<BaseBia, Pool>> }
const pools: Partial<Record<BaseBia, Pool>> = globalForBia.poolsBia ?? {}
if (process.env.NODE_ENV !== "production") globalForBia.poolsBia = pools

function pool(base: BaseBia): Pool {
  let p = pools[base]
  if (!p) {
    p = new Pool(configuracion(base))
    // Sin este handler, un error de red en una conexión ociosa tumba el proceso.
    p.on("error", (e) => console.error(`[db-bia:${base}] error en conexión ociosa:`, e.message))
    pools[base] = p
  }
  return p
}

// ─── API ────────────────────────────────────────────────────────────────────

/** Consulta parametrizada. Nunca interpolar valores en el SQL: usar $1, $2, … */
export async function consultar<T extends QueryResultRow = QueryResultRow>(
  base: BaseBia,
  sql: string,
  params: unknown[] = [],
): Promise<T[]> {
  const { rows } = await pool(base).query<T>(sql, params)
  return rows
}

/** Primera fila, o null. Para lookups puntuales. */
export async function consultarUno<T extends QueryResultRow = QueryResultRow>(
  base: BaseBia,
  sql: string,
  params: unknown[] = [],
): Promise<T | null> {
  const rows = await consultar<T>(base, sql, params)
  return rows[0] ?? null
}

/**
 * Transacción dentro de UNA base. Ojo: no hay transacción distribuida entre
 * file-compiler y calculator-prices — si una escritura falla después de que la
 * otra confirmó, hay que resolverlo por reintento. Por eso todas las tablas del
 * módulo son append-only con `carga_id`: reintentar es seguro.
 */
export async function enTransaccion<T>(
  base: BaseBia,
  fn: (cliente: PoolClient) => Promise<T>,
): Promise<T> {
  const cliente = await pool(base).connect()
  try {
    await cliente.query("BEGIN")
    const resultado = await fn(cliente)
    await cliente.query("COMMIT")
    return resultado
  } catch (e) {
    await cliente.query("ROLLBACK").catch(() => {})
    throw e
  } finally {
    cliente.release()
  }
}

export interface EstadoBase {
  base: BaseBia
  ok: boolean
  database?: string
  latenciaMs?: number
  error?: string
}

/** Ping a una base. No lanza: devuelve el estado para poder reportarlo. */
export async function verificar(base: BaseBia): Promise<EstadoBase> {
  const inicio = Date.now()
  try {
    const fila = await consultarUno<{ db: string }>(base, "SELECT current_database() AS db")
    return { base, ok: true, database: fila?.db, latenciaMs: Date.now() - inicio }
  } catch (e) {
    return { base, ok: false, error: e instanceof Error ? e.message : String(e) }
  }
}

export async function verificarTodas(): Promise<EstadoBase[]> {
  return Promise.all(
    (["file-compiler", "calculator-prices"] as BaseBia[]).map((b) => verificar(b)),
  )
}
