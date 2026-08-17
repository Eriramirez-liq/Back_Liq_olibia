import { consultar } from "@/lib/db-bia"
import { CODIGOS_AGENTE_STR, HOMOLOGACION_STR } from "@/lib/parsers/insumos-str"

/**
 * Nombres de los operadores de red de Cargos STR.
 *
 * No hay catálogo propio: los nombres salen de `public.agents` en la base
 * `file-compiler`, que es el registro de agentes de XM y ya contiene las 24
 * abreviaturas que aparecen como encabezados en los archivos BalanceSTR.
 *
 * Cuando un operador llega en varias columnas (AIRE = CSID + CSSD), el nombre se
 * toma del agente con actividad OPERADOR DE RED. Verificado contra la base: hay
 * exactamente uno por operador en los 23, y para AIRE elige el registro vigente
 * en vez del que figura como intervenido.
 */

const ACTIVIDAD_PRINCIPAL = "OPERADOR DE RED"

interface FilaAgente {
  code: string
  name: string
}

/**
 * Devuelve `or_codigo` → nombre legal. Los operadores sin agente principal en el
 * catálogo simplemente no aparecen en el mapa; el llamador decide qué hacer.
 */
export async function resolverNombresOperadores(): Promise<Map<string, string>> {
  const filas = await consultar<FilaAgente>(
    "file-compiler",
    `SELECT upper(trim(code)) AS code, name
       FROM public.agents
      WHERE upper(trim(code)) = ANY($1)
        AND activity = $2
        AND deleted_at IS NULL`,
    [CODIGOS_AGENTE_STR, ACTIVIDAD_PRINCIPAL],
  )

  const porOperador = new Map<string, string>()
  for (const { code, name } of filas) {
    const orCodigo = HOMOLOGACION_STR[code]
    if (orCodigo && name) porOperador.set(orCodigo, name)
  }
  return porOperador
}

/**
 * Completa `or_nombre` en las filas del parser. Si el catálogo no responde,
 * devuelve las filas sin nombre y el motivo — el preview puede mostrarse igual,
 * porque lo que el usuario valida son los montos.
 */
export async function completarNombres<T extends { or_codigo: string; or_nombre?: string }>(
  filas: T[],
): Promise<{ filas: T[]; advertencia?: string }> {
  if (filas.length === 0) return { filas }

  try {
    const nombres = await resolverNombresOperadores()
    const sinNombre: string[] = []

    const conNombre = filas.map((f) => {
      const nombre = nombres.get(f.or_codigo)
      if (!nombre) sinNombre.push(f.or_codigo)
      return { ...f, or_nombre: nombre ?? f.or_codigo }
    })

    const advertencia =
      sinNombre.length > 0
        ? `Sin nombre en el catálogo de agentes (se usa el código): ${sinNombre.join(", ")}.`
        : undefined

    return { filas: conNombre, advertencia }
  } catch (e) {
    return {
      filas,
      advertencia:
        "No se pudo consultar el catálogo de agentes para los nombres de los operadores: " +
        `${e instanceof Error ? e.message : String(e)}. Los montos no se ven afectados.`,
    }
  }
}
