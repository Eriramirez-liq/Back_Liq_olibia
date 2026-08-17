package controllers

import (
	"net/http"

	"bia-bills/providers/postgres"

	contextBia "github.com/biaenergy/bia-commons-go/context"
	"github.com/biaenergy/bia-commons-go/tracing"
	"github.com/gin-gonic/gin"
)

// Diagnóstico del módulo de Liquidaciones.
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_` para no pisar ninguno de los
// 75 controllers del servicio. Ver docs/backend/migracion-a-go.md.

type LiquidationsHealthController struct {
	db postgres.LiquidationsDB
}

func NewLiquidationsHealthController(db postgres.LiquidationsDB) LiquidationsHealthController {
	return LiquidationsHealthController{db: db}
}

type estadoBase struct {
	Base string `json:"base"`
	OK   bool   `json:"ok"`
	Erro string `json:"error,omitempty"`
}

type respuestaHealth struct {
	OK    bool         `json:"ok"`
	Bases []estadoBase `json:"bases"`
}

// Health verifica que las bases externas del módulo respondan.
//
// Es DIAGNÓSTICO, no liveness: consulta las bases, así que no sirve como health
// check de la plataforma — si dependiera de Postgres, un hipo de red haría
// reiniciar un contenedor sano. La sonda de cactus apunta al liveness del
// servicio, no acá.
//
// Responde 200 si las dos contestan, 503 si alguna falla.
func (controller LiquidationsHealthController) Health(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsHealthController.Health")
	defer span.End()

	bases := []postgres.LiquidationsDatabase{
		postgres.LiqDBFileCompiler,
		postgres.LiqDBCalculatorPrices,
	}

	respuesta := respuestaHealth{OK: true, Bases: make([]estadoBase, 0, len(bases))}

	for _, base := range bases {
		estado := estadoBase{Base: string(base), OK: true}

		if err := controller.db.Connection(base).WithContext(ctx).Exec("SELECT 1").Error; err != nil {
			estado.OK = false
			estado.Erro = err.Error()
			respuesta.OK = false
		}

		respuesta.Bases = append(respuesta.Bases, estado)
	}

	codigo := http.StatusOK
	if !respuesta.OK {
		codigo = http.StatusServiceUnavailable
	}

	c.JSON(codigo, respuesta)
}
