package entities

import "github.com/biaenergy/bia-commons-go/utils"

// Constantes del módulo de Liquidaciones.
//
// ⚠️ TRASVASE: este archivo se llama `liquidations_const.go` y NO `const.go`
// justamente para no pisar el de bia-bills al copiarlo. Por la misma razón todos
// los nombres llevan prefijo `Liq`: viven en el mismo paquete `entities` que sus
// ~100 constantes y una colisión no compilaría.
// Ver docs/backend/migracion-a-go.md.
var (
	// ─── Bases externas del módulo ──────────────────────────────────────────
	// Cargos STR no guarda en la base del servicio: el insumo crudo va a
	// file-compiler y el valor a pagar a calculator-prices. Las dos viven en el
	// mismo RDS que la base del servicio, con otro usuario: el de bia-bills no
	// tiene permisos ahí.
	LiqDbHost = utils.GetEnv("liq_db_host", "")
	LiqDbPort = utils.GetEnv("liq_db_port", "5432")
	LiqDbUser = utils.GetEnv("liq_db_user", "")
	LiqDbPass = utils.GetEnv("liq_db_password", "")

	// Nombres de base, configurables por ambiente. En el RDS de dev no llevan
	// prefijo: el "dev" está en el servidor, no en la base.
	LiqDbNameFileCompiler     = utils.GetEnv("liq_db_name_file_compiler", "file-compiler")
	LiqDbNameCalculatorPrices = utils.GetEnv("liq_db_name_calculator_prices", "calculator-prices")

	LiqDbSSLMode = utils.GetEnv("liq_db_ssl_mode", "require")

	// Log de las consultas del módulo. Lee la MISMA variable que el
	// `ProdEnviroment` de bia-bills, pero con nombre propio a propósito: si
	// usáramos su constante, este repo no compilaría (no la tiene) y si la
	// declaráramos acá, colisionaría al copiar el archivo a su paquete
	// `entities`. Regla general: el módulo no referencia símbolos de bia-bills
	// que no declare él mismo.
	LiqSQLDebug = utils.GetEnv("GO_ENVIRONMENT", "production") != "production"

	// ─── Cargos STR ─────────────────────────────────────────────────────────
	// Tope de archivos de ajuste por lote: es el ancho de la tabla de insumos.
	// Un cuarto archivo corta la carga con error explícito en vez de
	// descartarse en silencio.
	LiqStrMaxReinvoices = 3
)
