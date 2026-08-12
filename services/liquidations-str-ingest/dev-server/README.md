# STR dev-server — probar el cargue de insumos en local (Fase 1)

Backend **local** de prueba para el módulo de Cargas STR de olibia-web. Implementa
los endpoints que la UI ya llama (`/api/cargas/preview`, `/confirmar`, etc.) para
que puedas subir los `BalanceSTR*.xlsx` desde la interfaz real y verlos leídos.

```
Navegador (Cargas STR)  →  olibia-web  →  /api/liquidations-proxy  →  dev-server (:4000)
                                                                         ├─ parsea el Excel
                                                                         └─ (confirmar) UPSERT en bia-bi
```

## Cómo correrlo

**1. Levantar el dev-server** (puerto 4000):
```bash
cd services/liquidations-str-ingest/dev-server
npm install       # una vez
npm start
```

**2. Apuntar olibia-web al dev-server.** Ya está hecho: se creó
`olibia-web/.env.local` con
`NEXT_PUBLIC_LIQUIDATIONS_BACKEND_URL=http://localhost:4000`.
(Si Next.js ya estaba corriendo, reiniciá `npm run dev` para que tome la variable.)

**3. Levantar olibia-web** (otra terminal, puerto 3000):
```bash
cd olibia-web
npm run dev
```

**4. Probar en la UI:**
1. Iniciá sesión (Firebase).
2. Andá a **Finance → Liquidaciones → pestaña Cargas**.
3. **Nueva carga** → elegí el período → tarjeta **Insumos STR**.
4. Subí uno o más `BalanceSTR*.xlsx` (con `tipofactu` / `tiporefactu` en el nombre).
5. **Ver preview** → se muestra la tabla por operador: Factura, Refactura y A pagar.
6. **Confirmar** → hace UPSERT en `bia-bi.liquidations_str.str_charges`.

## Endpoints

| Método | Ruta | Qué hace |
|---|---|---|
| POST | `/api/cargas/preview` | Parsea los BalanceSTR y devuelve el cálculo (no guarda) |
| POST | `/api/cargas/confirmar` | UPSERT en `str_charges` (necesita bia-bi) |
| GET | `/api/cargas/estado-periodo` · `/api/cargas` · `/api/operadores` · `/api/periodos` | Stubs para que la vista renderice |
| GET | `/api/str/cargos?periodo=YYYY-MM` | Debug: lee lo guardado |
| GET | `/health` | Estado + si hay DB |

## Notas
- El **preview funciona sin base de datos**; solo `confirmar` necesita bia-bi
  (lee las credenciales del `.env` raíz de olibia-web).
- Solo `INSUMOS_STR` está implementado (Fase 1). Las demás fuentes devuelven un
  aviso. La Fase 2 (envío a NetSuite) es aparte.
- La lógica de parseo está en `parser.js` (portada de `insumos-str.ts`):
  factu → `BalSTR01/02`, refactu → `*_Ajuste`, columna B `BIAC-BIAE`,
  homologación de columnas por operador (`AIRE = CSID + CSSD`), y
  `a pagar = Σ factura + Σ refactura`.
- Puerto configurable: `PORT=4001 npm start`.
