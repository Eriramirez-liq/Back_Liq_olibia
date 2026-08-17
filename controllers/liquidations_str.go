package controllers

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"bia-bills/repositories"
	"bia-bills/services/cargos_str"

	contextBia "github.com/biaenergy/bia-commons-go/context"
	ginCommons "github.com/biaenergy/bia-commons-go/gin/errors"
	"github.com/biaenergy/bia-commons-go/tracing"
	"github.com/gin-gonic/gin"
)

// Handlers de Cargos STR.
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_` para no pisar ninguno de los
// 75 controllers del servicio. Ver docs/backend/migracion-a-go.md.
//
// Delgados a propósito, igual que los suyos: parsean el request, delegan en el
// servicio y serializan.

// tamañoMáximoPorArchivo acota lo que se acepta en el multipart. Los BalanceSTR
// reales pesan entre 140 KB y 290 KB; 32 MB deja margen de sobra y evita que un
// archivo enorme consuma memoria del servicio.
const tamañoMaximoPorArchivo = 32 << 20 // 32 MB

type LiquidationsStrController struct {
	service cargos_str.CargosStrService
}

func NewLiquidationsStrController(service cargos_str.CargosStrService) LiquidationsStrController {
	return LiquidationsStrController{service: service}
}

type confirmarRequest struct {
	Rows []cargos_str.StrRow `json:"rows" binding:"required"`
}

// Preview parsea los archivos del lote y devuelve lo que se guardaría.
//
// POST /liquidations/cargos-str/preview  (multipart: files[], year, month)
func (controller LiquidationsStrController) Preview(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsStrController.Preview")
	defer span.End()

	year, month, err := periodoDelRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, ginCommons.NewBadRequest(err.Error()))
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, ginCommons.NewBadRequest("se esperaba un multipart con los archivos"))
		return
	}

	archivos := form.File["files"]
	if len(archivos) == 0 {
		c.JSON(http.StatusBadRequest, ginCommons.NewBadRequest("Insumos STR requiere al menos un archivo"))
		return
	}

	subidos := make([]cargos_str.UploadedFile, 0, len(archivos))
	for _, encabezado := range archivos {
		if encabezado.Size > tamañoMaximoPorArchivo {
			c.JSON(http.StatusBadRequest, ginCommons.NewBadRequest(
				"el archivo "+encabezado.Filename+" supera el máximo de 32 MB"))
			return
		}

		abierto, err := encabezado.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, ginCommons.NewBadRequest(
				"no se pudo leer el archivo "+encabezado.Filename))
			return
		}

		contenido, err := io.ReadAll(abierto)
		_ = abierto.Close()
		if err != nil {
			c.JSON(http.StatusBadRequest, ginCommons.NewBadRequest(
				"no se pudo leer el archivo "+encabezado.Filename))
			return
		}

		subidos = append(subidos, cargos_str.UploadedFile{Name: encabezado.Filename, Content: contenido})
	}

	resultado, err := controller.service.Preview(ctx, subidos, year, month)
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

// Confirm persiste el lote que el usuario validó en pantalla.
//
// POST /liquidations/cargos-str/confirm
func (controller LiquidationsStrController) Confirm(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsStrController.Confirm")
	defer span.End()

	var body confirmarRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, ginCommons.NewBadRequest(err.Error()))
		return
	}

	loadID, err := controller.service.Confirm(ctx, body.Rows)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginCommons.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"load_id": loadID, "rows": len(body.Rows)})
}

// Charges devuelve el valor a pagar vigente por (período, operador).
//
// GET /liquidations/cargos-str?periods=2026-07,2026-06&operators=CHEC,EPM
func (controller LiquidationsStrController) Charges(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsStrController.Charges")
	defer span.End()

	filtro := repositories.StrChargeFilter{
		Periods:       listaDelQuery(c, "periods"),
		OperatorCodes: listaDelQuery(c, "operators"),
	}

	cargos, err := controller.service.CurrentCharges(ctx, filtro)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginCommons.NewInternalServerError(err.Error()))
		return
	}

	var total float64
	for _, cargo := range cargos {
		total += cargo.AmountPayable
	}

	c.JSON(http.StatusOK, gin.H{"charges": cargos, "total": total})
}

// Periods lista los períodos con datos cargados.
//
// GET /liquidations/cargos-str/periods
func (controller LiquidationsStrController) Periods(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsStrController.Periods")
	defer span.End()

	periodos, err := controller.service.Periods(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginCommons.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, periodos)
}

func periodoDelRequest(c *gin.Context) (int, int, error) {
	year, err := strconv.Atoi(c.PostForm("year"))
	if err != nil || year < 2000 || year > 2100 {
		return 0, 0, errParametro("year")
	}

	month, err := strconv.Atoi(c.PostForm("month"))
	if err != nil || month < 1 || month > 12 {
		return 0, 0, errParametro("month")
	}

	return year, month, nil
}

func listaDelQuery(c *gin.Context, clave string) []string {
	crudo := strings.TrimSpace(c.Query(clave))
	if crudo == "" {
		return nil
	}

	partes := strings.Split(crudo, ",")
	limpias := make([]string, 0, len(partes))
	for _, p := range partes {
		if p = strings.TrimSpace(p); p != "" {
			limpias = append(limpias, p)
		}
	}

	return limpias
}

type errParametro string

func (e errParametro) Error() string {
	return "parámetro " + string(e) + " inválido o ausente"
}
