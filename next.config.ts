import type { NextConfig } from "next"

const nextConfig: NextConfig = {
  // Build autocontenido para la imagen Docker: deja en .next/standalone un
  // server.js con solo las dependencias que el runtime usa. Sin esto habría que
  // copiar node_modules entero a la imagen.
  output: "standalone",

  // Prefijo de ruta cuando el servicio queda detrás del enrutado por path de
  // cactus (ej. /ms-liquidations, como /ms-bill o /ms-file-compiler). Ojo: Next
  // lo resuelve en BUILD, así que va como build-arg del Dockerfile y no como
  // variable de entorno del servicio.
  //
  // Vacío = el servicio responde en la raíz de su host, y no hay que tocar nada
  // más: el proxy de olibia arma la URL con
  // NEXT_PUBLIC_LIQUIDATIONS_BACKEND_URL + el path del endpoint.
  ...(process.env.NEXT_PUBLIC_BASE_PATH
    ? { basePath: process.env.NEXT_PUBLIC_BASE_PATH }
    : {}),

  eslint: {
    ignoreDuringBuilds: true,
  },
  typescript: {
    ignoreBuildErrors: false,
  },
}

export default nextConfig
