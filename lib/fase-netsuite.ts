import { NextResponse } from "next/server"

/**
 * Interruptor de la fase 2 (integración con NetSuite).
 *
 * El backend es una sola app Next: desplegarla despliega sus 44 rutas, no hay
 * forma de publicar solo algunas. Este módulo cierra las de NetSuite para que,
 * aunque el código viaje en la imagen, la integración **no esté disponible**
 * mientras solo se despliega la fase 1 (cargue, cálculo y visualización).
 *
 * Está cerrado por defecto: hay que poner `NETSUITE_ENABLED=true` a propósito.
 * Olvidarse de la variable deja la fase 2 apagada, que es el error seguro.
 *
 * Las rutas de escritura responden 503 con un mensaje claro. Las de lectura que
 * la pantalla de Cargos STR consulta sola —lote activo, estados, historial de
 * lotes— responden "no hay nada" en vez de un error, así la pestaña sigue
 * funcionando para lo que sí está desplegado.
 */
export function netsuiteHabilitado(): boolean {
  return (process.env.NETSUITE_ENABLED ?? "false").toLowerCase() === "true"
}

const MENSAJE =
  "La integración con NetSuite no está desplegada en este ambiente. " +
  "Corresponde a la fase 2 del módulo de Cargos STR."

/** 503 para las rutas que crean o modifican algo en NetSuite. */
export function respuestaFase2NoDisponible(): NextResponse {
  return NextResponse.json(
    { error: "NETSUITE_NO_DISPONIBLE", message: MENSAJE },
    { status: 503 },
  )
}
