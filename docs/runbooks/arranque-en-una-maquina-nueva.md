# Arrancar el módulo de Liquidaciones en una máquina nueva

> Escrito al mudarse de Windows a macOS, pero sirve para cualquier equipo nuevo.

Lo que **sí** viaja por git es el código. Lo que **no** viaja está listado abajo, y
es justo lo que hace que el entorno funcione: credenciales, archivos de ejemplo y
la sesión de la VPN.

---

## 1. Los tres repos y sus ramas

| Repo | Rama | Para qué |
|---|---|---|
| `Back_Liq_olibia` | `feat/migracion-str-bases-bia` | donde se desarrolla el backend en Go |
| `olibia-web` | `feat/liquidations-str-ajustes` | el front |
| `bia-bills` | `feat/liquidations-cargos-str` | el trasvase, de donde sale el deploy |
| `bia-bills` | `temp/eri-kate` | la rama que se despliega: lo de Liquidaciones **más** lo de Kate |

Son **siempre las mismas ramas**. No se crean ramas por módulo.

```bash
git clone git@github.com:Eriramirez-liq/Back_Liq_olibia.git
git clone git@github.com:biaenergy/olibia-web.git
git clone git@github.com:biaenergy/bia-bills.git
```

---

## 2. Lo que NO viaja por git

### Los archivos de entorno

Están en `.gitignore` y contienen credenciales. Hay que copiarlos de la máquina
vieja —por un canal seguro, no por chat— o pedirlos al equipo.

| Archivo | Qué necesita el módulo |
|---|---|
| `Back_Liq_olibia/.env` | `DB_HOST`, `DB_PORT`, **`DB_USER2`**, **`DB_PASSWORD2`** (las bases de BIA), `FIREBASE_PROJECT_ID` |
| `olibia-web/.env` | `NEXT_PUBLIC_FIREBASE_PROJECT_ID` — tiene que ser el MISMO que el de arriba |
| `olibia-web/.env.local` | `NEXT_PUBLIC_BACKEND_URL`, `NEXT_PUBLIC_LIQUIDATIONS_BACKEND_URL` (ver §4) |
| `bia-bills/.env` | el suyo propio, para correr ese servicio |

**Ojo con los nombres.** El arnés lee variables `liq_db_*`, pero en el `.env` se
llaman distinto: el usuario y la contraseña de las bases de Liquidaciones son
**`DB_USER2` y `DB_PASSWORD2`**, no `DB_USER`/`DB_PASSWORD`, que son de otra base.
Confundirlas da un error de conexión que parece de red.

### Los archivos de ejemplo de XM

`archivos_ejemplo/formatos_or/` pesa cientos de MB y solo una parte está
versionada:

- **Sí está en git**: `Insumos SDL/` (696 K, los 33 archivos que usa el test dorado).
- **No está**: `TC1/` (274 MB, 21 archivos) y algunos sueltos de SDL.

Sin ellos, los tests que los usan **se saltean** en vez de fallar, y lo dicen. Si
se quieren correr, hay que copiar la carpeta aparte.

### La VPN

Las bases (`c4-rds-bia-dev.dev.bia.app`) solo se alcanzan por la **VPN de AWS**.
Sin ella el arnés arranca igual pero `/health` responde 503 y las consultas tardan
21 segundos en fallar con `dial tcp ... timed out`.

Y al revés: **con la VPN puesta, Supabase no responde**. Su host resuelve solo a
IPv6 y ese camino no funciona por la VPN. Eso deja caído el backend TypeScript
—y con él las fuentes todavía no migradas— mientras se trabaja con las bases de
BIA. Es esperable, no una falla.

---

## 3. Herramientas

- **Go 1.24+** — `brew install go`
- **Node 20+** — el repo usa nvm; `nvm use` en cada repo
- Acceso a los módulos privados de la organización:
  ```bash
  export GOPRIVATE="github.com/biaenergy/*"
  git config --global url."git@github.com:".insteadOf https://github.com
  ```

---

## 4. Levantar el entorno local

Son **tres** procesos. En macOS los comandos son los mismos; lo único que cambia
es que no hace falta el `.exe`.

### El arnés en Go (puerto 4110)

Atiende `/ms-bill/liquidations/*` y **reenvía todo lo demás** al gateway de
desarrollo. Ese reenvío no es opcional: el front manda TODO su tráfico de
`/ms-bill` a una sola URL, así que sin él se cortan los permisos y el selector de
equipo queda bloqueado.

```bash
cd Back_Liq_olibia
export GOPRIVATE="github.com/biaenergy/*"
export liq_db_host="$(grep -m1 '^DB_HOST='      .env | cut -d= -f2- | tr -d '"')"
export liq_db_port="$(grep -m1 '^DB_PORT='      .env | cut -d= -f2- | tr -d '"')"
export liq_db_user="$(grep -m1 '^DB_USER2='     .env | cut -d= -f2- | tr -d '"')"
export liq_db_password="$(grep -m1 '^DB_PASSWORD2=' .env | cut -d= -f2- | tr -d '"')"
export LIQ_DEV_PORT=4110
go run ./cmd/liquidations-dev
```

Verificar:

```bash
curl -s localhost:4110/ms-bill/liquidations/health
# {"ok":true,"bases":[{"base":"file-compiler","ok":true},{"base":"calculator-prices","ok":true}]}
```

### El backend TypeScript (puerto 4000)

Sirve las fuentes que todavía no se migraron. `npm run dev` en `Back_Liq_olibia`
—usa el 4000 a propósito—. Necesita Supabase, así que con la VPN puesta va a dar
401 en todo. Es esperable.

### El front (puerto 3000)

`npm run dev` en `olibia-web`. Su `.env.local` reparte el tráfico:

```
NEXT_PUBLIC_BACKEND_URL=http://localhost:4110              # el arnés en Go
NEXT_PUBLIC_LIQUIDATIONS_BACKEND_URL=http://localhost:4000 # el backend viejo
```

Entrar por `http://localhost:3000/finance/liquidations/finance`.

---

## 5. Cosas que ya costaron una sesión

**Reiniciar el server del front desloguea.** Si el dev server se reinicia con una
pestaña abierta, su `POST /api/auth/session` falla y el `SessionProvider` **borra
la cookie a propósito**. Todo empieza a dar 401 en 50 ms. Se arregla recargando la
pestaña completa con el server arriba. No es el backend.

**El gate del front es `npm run build`, no `tsc --noEmit`.** `tsconfig.json` tiene
`"incremental": true` y tsc reusa el `tsconfig.tsbuildinfo` cacheado, así que se
saltea archivos que cree sin cambios. Un error de tipos pasó local y tumbó el
deploy. Antes de decir que el front está listo, correr el build de verdad.

**Los tres sabores de 404 del gateway.** Cuerpo vacío con `server: envoy` = el
gateway no tiene la ruta. `400 authorization header is empty` = llegó a la app,
pero **no dice nada sobre si la ruta existe**: el middleware de auth corta antes de
rutear, y una ruta inventada da lo mismo. `404 page not found` en texto = es Gin,
la ruta no está compilada.

**El slot de develop en cactus es uno solo y gana el último deploy.** Un deploy de
otra rama pisa el nuestro sin avisar.

---

## 6. Tests

```bash
# Back — sin red ni base, todo mockeado
cd Back_Liq_olibia && go test $(go list ./... | grep -v node_modules)

# Front
cd olibia-web && npx jest src/modules/finance/liquidations

# Los pesados, detrás de variables porque tardan o necesitan el entorno arriba
TC1_ARCHIVOS_REALES=1 npx jest zz-tc1-real   # 7,5 min, necesita los 21 archivos
TC1_E2E=1            npx jest zz-tc1-e2e     # necesita el arnés y la VPN
```

---

## 7. Dónde está el resto

- [`INTEGRACION.md`](../../INTEGRACION.md) — el contrato con el front, la bitácora
  de cambios y los pendientes abiertos. **Es lo primero que hay que leer.**
- [`docs/backend/migracion-a-go.md`](../backend/migracion-a-go.md) — cómo funciona
  el trasvase a bia-bills y las trampas del port.
- [`docs/runbooks/despliegue-cactus.md`](./despliegue-cactus.md) — el deploy.
