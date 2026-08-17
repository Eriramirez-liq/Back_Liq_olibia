import { TipoFuente } from "@prisma/client"

/**
 * Qué fuentes de carga ya viven en las bases de BIA y cuáles siguen en Supabase.
 *
 * La migración del módulo de liquidaciones va fuente por fuente. Mientras dure,
 * hay endpoints que muestran las siete juntas —el historial de cargas, el estado
 * del período, el dashboard, la proyección— y necesitan saber de dónde leer cada
 * una.
 *
 * En vez de repetir esa decisión en cada endpoint (y tener que tocarlos todos
 * cada vez que se migra una fuente), la decisión vive acá. Migrar la próxima
 * fuente es agregar una línea a este set.
 */
export const FUENTES_EN_BIA: ReadonlySet<TipoFuente> = new Set<TipoFuente>([
  // Insumos STR: el desglose va a file-compiler y el valor a pagar a
  // calculator-prices. Ya no se escribe `registros_str` de Supabase.
  TipoFuente.INSUMOS_STR,
])

/** true si esta fuente ya se lee y escribe contra las bases de BIA. */
export function estaMigrada(fuente: TipoFuente): boolean {
  return FUENTES_EN_BIA.has(fuente)
}

/** Las que todavía viven en Supabase. Útil para filtrar consultas Prisma. */
export function fuentesEnSupabase(): TipoFuente[] {
  return Object.values(TipoFuente).filter((f) => !FUENTES_EN_BIA.has(f))
}
