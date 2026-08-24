package router

import (
	"bia-bills/controllers"
	"bia-bills/providers/postgres"
	"bia-bills/repositories"
	"bia-bills/services/cargos_str"
	"bia-bills/services/tarifas_sdl"
	"bia-bills/services/tc1"

	"github.com/gin-gonic/gin"
)

// Rutas del módulo de Liquidaciones.
//
// ⚠️ TRASVASE: archivo `liquidations_router.go`, aparte de `router.go`, que tiene
// ~600 líneas del servicio y NO se puede pisar.
//
// Toda la inyección de dependencias del módulo vive acá adentro. Así la única
// edición manual en el `router.go` de bia-bills es una línea:
//
//	RegisterLiquidations(apiPrefix)
//
// Todo lo demás es copiar archivos. Ver docs/backend/migracion-a-go.md.
//
// Las rutas quedan bajo el prefijo del servicio, o sea /ms-bill/liquidations/...
// Ese es el path que el front usa en su endpoints.ts para lo ya migrado.
func RegisterLiquidations(apiPrefix *gin.RouterGroup) {
	// ── Providers ───────────────────────────────────────────────────────────
	db := postgres.NewLiquidationsDB()

	// ── Repositories ────────────────────────────────────────────────────────
	strRepository := repositories.NewLiquidationsStrRepository(db)
	agentsRepository := repositories.NewLiquidationsAgentsRepository(db)

	sdlRepository := repositories.NewLiquidationsSdlRepository(db)

	tc1Repository := repositories.NewLiquidationsTc1Repository(db)

	// ── Services ────────────────────────────────────────────────────────────
	cargosStrService := cargos_str.NewCargosStrService(strRepository, agentsRepository)
	tarifasSdlService := tarifas_sdl.NewTarifasSdlService(sdlRepository, agentsRepository)
	// Los operadores que se esperan cargar cada período. Se reusa el catálogo de
	// Tarifas SDL porque es el mismo universo: se verificó contra los 21 archivos
	// reales de TC1 y los códigos coinciden uno a uno. Entra como dato, no como
	// dependencia del servicio.
	tc1Service := tc1.NewTc1Service(tc1Repository, tarifas_sdl.OperatorCodes())

	// ── Controllers ─────────────────────────────────────────────────────────
	healthController := controllers.NewLiquidationsHealthController(db)
	cargosStrController := controllers.NewLiquidationsStrController(cargosStrService)
	tarifasSdlController := controllers.NewLiquidationsSdlController(tarifasSdlService)
	tc1Controller := controllers.NewLiquidationsTc1Controller(tc1Service)

	// ── Rutas ───────────────────────────────────────────────────────────────
	liquidationsGroup := apiPrefix.Group("/liquidations")
	liquidationsGroup.GET("/health", healthController.Health)

	cargosStrGroup := liquidationsGroup.Group("/cargos-str")
	cargosStrGroup.POST("/preview", cargosStrController.Preview)
	cargosStrGroup.POST("/confirm", cargosStrController.Confirm)
	cargosStrGroup.GET("", cargosStrController.Charges)
	cargosStrGroup.GET("/periods", cargosStrController.Periods)
	cargosStrGroup.GET("/loads", cargosStrController.Loads)

	tarifasSdlGroup := liquidationsGroup.Group("/tarifas-sdl")
	tarifasSdlGroup.POST("/preview", tarifasSdlController.Preview)
	tarifasSdlGroup.POST("/confirm", tarifasSdlController.Confirm)
	tarifasSdlGroup.GET("", tarifasSdlController.Rates)
	tarifasSdlGroup.GET("/periods", tarifasSdlController.Periods)
	tarifasSdlGroup.GET("/loads", tarifasSdlController.Loads)
	// Diagnóstico: recalcula desde los componentes guardados y compara.
	tarifasSdlGroup.GET("/audit", tarifasSdlController.Audit)

	// TC1 no tiene preview ni sube archivos: se parsea en el navegador —el de
	// CELSIA_VALLE pesa 98 MB— y llegan las filas ya normalizadas.
	tc1Group := liquidationsGroup.Group("/tc1")
	tc1Group.POST("/confirm", tc1Controller.Confirm)
	tc1Group.GET("", tc1Controller.Inputs)
	tc1Group.GET("/periods", tc1Controller.Periods)
	tc1Group.GET("/loads", tc1Controller.Loads)
	tc1Group.GET("/status", tc1Controller.Status)
	tc1Group.GET("/operators", tc1Controller.Operators)
}
