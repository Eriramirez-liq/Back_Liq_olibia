package controllers

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"bia-bills/repositories"
	"bia-bills/services/tarifas_sdl"

	contextBia "github.com/biaenergy/bia-commons-go/context"
	// gin/errors y NO gin: el paquete completo arrastra handlers → cache →
	// go-redis, que no está en el go.sum de este repo. Es el mismo import que usa
	// el controller de Cargos STR.
	ginCommons "github.com/biaenergy/bia-commons-go/gin/errors"
	"github.com/biaenergy/bia-commons-go/tracing"
	"github.com/gin-gonic/gin"
)

// Endpoints de Tarifas SDL.
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_` para no pisar ninguno de los
// controllers del servicio. Ver docs/backend/migracion-a-go.md.

// El lote son 33 archivos. El tope de STR (32 MB) alcanzaría, pero se sube a 64
// para no quedar al límite: si se pasa, el error tiene que ser explícito y no un
// cuerpo truncado.
const maxMultipartSdl = 64 << 20

type LiquidationsSdlController struct {
	service tarifas_sdl.TarifasSdlService
}

func NewLiquidationsSdlController(service tarifas_sdl.TarifasSdlService) LiquidationsSdlController {
	return LiquidationsSdlController{service: service}
}

type confirmarSdlRequest struct {
	// El período lo elige la persona en Nueva carga y aplica a todo el lote. NO
	// sale de los nombres de los archivos: los ADD y los de uso de la red pueden
	// ser de meses distintos, y eso es correcto.
	Period string                   `json:"period" binding:"required"`
	Rows   []tarifas_sdl.PreviewRow `json:"rows" binding:"required"`

	// Metadatos del cargue, para el historial. Opcionales: sin ellos el cargue se
	// guarda igual y el historial lo muestra sin usuario.
	CreatedBy string `json:"created_by"`
}

// Preview parsea los archivos del lote y devuelve lo que se guardaría.
//
// POST /liquidations/tarifas-sdl/preview  (multipart: files[], period opcional)
func (controller LiquidationsSdlController) Preview(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsSdlController.Preview")
	defer span.End()

	if err := c.Request.ParseMultipartForm(maxMultipartSdl); err != nil {
		c.JSON(http.StatusBadRequest, ginCommons.NewBadRequest(fmt.Sprintf(
			"no se pudo leer el formulario con los archivos: %v", err)))
		return
	}

	formulario := c.Request.MultipartForm
	if formulario == nil || len(formulario.File["files"]) == 0 {
		c.JSON(http.StatusBadRequest, ginCommons.NewBadRequest(
			"hay que enviar al menos un archivo en el campo files"))
		return
	}

	subidos := make([]tarifas_sdl.UploadedFile, 0, len(formulario.File["files"]))
	for _, cabecera := range formulario.File["files"] {
		archivo, err := cabecera.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, ginCommons.NewBadRequest(fmt.Sprintf(
				"no se pudo abrir el archivo %q: %v", cabecera.Filename, err)))
			return
		}

		contenido, err := io.ReadAll(archivo)
		_ = archivo.Close()
		if err != nil {
			c.JSON(http.StatusBadRequest, ginCommons.NewBadRequest(fmt.Sprintf(
				"no se pudo leer el archivo %q: %v", cabecera.Filename, err)))
			return
		}

		subidos = append(subidos, tarifas_sdl.UploadedFile{Name: cabecera.Filename, Content: contenido})
	}

	// El período es el que se eligió en Nueva carga. Es OPCIONAL en el request y no
	// se guarda acá —eso pasa en Confirm—, pero si viene se contrasta contra los
	// archivos de uso de la red y corta si no son de ese mes. Opcional a propósito:
	// un front anterior a esta validación seguiría funcionando en vez de romperse
	// con un 400, y el propio preview avisa de que no pudo verificar.
	resultado, err := controller.service.Preview(ctx, subidos, strings.TrimSpace(c.PostForm("period")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginCommons.NewInternalServerError(err.Error()))
		return
	}

	// Con errores críticos el lote no se puede cargar: 422, no 200 con un cuerpo
	// que el front tenga que inspeccionar para darse cuenta.
	if len(resultado.CriticalErrors) > 0 {
		c.JSON(http.StatusUnprocessableEntity, resultado)
		return
	}

	c.JSON(http.StatusOK, resultado)
}

// Confirm persiste el lote que la persona validó en pantalla.
//
// POST /liquidations/tarifas-sdl/confirm
func (controller LiquidationsSdlController) Confirm(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsSdlController.Confirm")
	defer span.End()

	var body confirmarSdlRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, ginCommons.NewBadRequest(err.Error()))
		return
	}

	// El id del usuario sale del header, no del cuerpo: es el único dato de
	// identidad que no puede falsear quien arma el request.
	meta := tarifas_sdl.LoadMeta{
		CreatedBy:   body.CreatedBy,
		CreatedByID: c.GetHeader("x-user-id"),
	}

	loadID, err := controller.service.Confirm(ctx, body.Period, body.Rows, meta)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginCommons.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"load_id": loadID, "rows": len(body.Rows)})
}

// Rates devuelve las tarifas vigentes por (período, operador).
//
// GET /liquidations/tarifas-sdl?periods=2026-01&operators=CENS,CHEC
func (controller LiquidationsSdlController) Rates(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsSdlController.Rates")
	defer span.End()

	filtro := repositories.SdlRateFilter{
		Periods:       listaDelQuery(c, "periods"),
		OperatorCodes: listaDelQuery(c, "operators"),
	}

	tarifas, err := controller.service.CurrentRates(ctx, filtro)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginCommons.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"rates": tarifas})
}

// Periods lista los períodos que ya tienen tarifas.
//
// GET /liquidations/tarifas-sdl/periods
func (controller LiquidationsSdlController) Periods(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsSdlController.Periods")
	defer span.End()

	periodos, err := controller.service.Periods(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginCommons.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, periodos)
}

// Loads devuelve el historial de cargues.
//
// GET /liquidations/tarifas-sdl/loads?periods=2026-01
func (controller LiquidationsSdlController) Loads(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsSdlController.Loads")
	defer span.End()

	cargues, err := controller.service.Loads(ctx, listaDelQuery(c, "periods"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginCommons.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"loads": cargues})
}

// Audit recalcula las tarifas desde los componentes guardados y reporta cualquier
// desacuerdo.
//
// Es diagnóstico, no parte del flujo: sirve para demostrar que una tarifa
// guardada es consecuencia de los insumos que están al lado. Responde 200 si todo
// cuadra y 409 si encontró algo, para que un chequeo automático lo note sin leer
// el cuerpo.
//
// GET /liquidations/tarifas-sdl/audit?periods=2026-01
func (controller LiquidationsSdlController) Audit(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsSdlController.Audit")
	defer span.End()

	resultado, err := controller.service.Audit(ctx, listaDelQuery(c, "periods"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginCommons.NewInternalServerError(err.Error()))
		return
	}

	if len(resultado.Findings) > 0 || len(resultado.Orphans) > 0 {
		c.JSON(http.StatusConflict, resultado)
		return
	}

	c.JSON(http.StatusOK, resultado)
}
