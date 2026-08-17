// Command liquidations-dev levanta SOLO el módulo de Liquidaciones, para probarlo
// local sin arrastrar todo bia-bills.
//
// ⚠️ NO SE TRASVASA. En bia-bills el entrypoint es su `main.go` de la raíz, que
// arma el router completo del servicio; ahí el módulo se registra con una línea
// (RegisterLiquidations). Este archivo vive en cmd/ —ruta que su repo no tiene—
// justamente para no pisarle nada y para que quede claro que es un arnés de
// desarrollo, no parte del servicio.
//
// Uso:
//
//	liq_db_host=... liq_db_user=... liq_db_password=... \
//	  go run ./cmd/liquidations-dev
//
// Las rutas quedan igual que en producción: /ms-bill/liquidations/...
package main

import (
	"log"
	"os"

	"bia-bills/router"

	"github.com/gin-gonic/gin"
)

func main() {
	// Variable propia y no PORT: el .env del repo trae PORT=5001 de la etapa Flask
	// y arrancó ahí sin que nadie se lo pidiera. Un nombre específico evita esa
	// clase de sorpresa.
	puerto := os.Getenv("LIQ_DEV_PORT")
	if puerto == "" {
		puerto = "8081" // 8080 lo usa bia-bills local; 4000 el backend TypeScript
	}

	gin.SetMode(gin.ReleaseMode)
	motor := gin.New()
	motor.Use(gin.Logger(), gin.Recovery())

	// Mismo prefijo que el servicio real, para que las URLs de la prueba sean las
	// mismas que va a ver el front.
	apiPrefix := motor.Group("/ms-bill")
	router.RegisterLiquidations(apiPrefix)

	log.Printf("liquidations-dev escuchando en :%s — probá /ms-bill/liquidations/health", puerto)
	if err := motor.Run(":" + puerto); err != nil {
		log.Fatal(err)
	}
}
