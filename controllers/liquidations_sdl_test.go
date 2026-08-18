package controllers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bia-bills/controllers"
	"bia-bills/repositories"
	"bia-bills/services/tarifas_sdl"

	"github.com/gin-gonic/gin"
)

// Tests de la capa HTTP de Tarifas SDL. No tocan base: el servicio es un doble.
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_`. Ver docs/backend/migracion-a-go.md.
//
// Cubren lo que solo se puede romper acá: el parseo del multipart de 33 archivos,
// de dónde sale la identidad del usuario, y los códigos de estado. Un 200 donde
// correspondía un 422 hace que el front cargue un lote inválido.

type servicioSdlFake struct {
	resultado tarifas_sdl.PreviewResult
	loadID    string
	tarifas   []repositories.SdlRate
	periodos  []string
	cargues   []repositories.SdlLoad
	auditoria tarifas_sdl.AuditResult
	err       error

	archivos          []tarifas_sdl.UploadedFile
	periodoRecibido   string
	metaRecibida      tarifas_sdl.LoadMeta
	filtroRecibido    repositories.SdlRateFilter
	periodosRecibidos []string
}

func (f *servicioSdlFake) Preview(_ context.Context, files []tarifas_sdl.UploadedFile) (tarifas_sdl.PreviewResult, error) {
	f.archivos = files
	return f.resultado, f.err
}

func (f *servicioSdlFake) Confirm(_ context.Context, period string, _ []tarifas_sdl.PreviewRow, meta tarifas_sdl.LoadMeta) (string, error) {
	f.periodoRecibido = period
	f.metaRecibida = meta
	return f.loadID, f.err
}

func (f *servicioSdlFake) CurrentRates(_ context.Context, filtro repositories.SdlRateFilter) ([]repositories.SdlRate, error) {
	f.filtroRecibido = filtro
	return f.tarifas, f.err
}

func (f *servicioSdlFake) Periods(context.Context) ([]string, error) {
	return f.periodos, f.err
}

func (f *servicioSdlFake) Loads(_ context.Context, periods []string) ([]repositories.SdlLoad, error) {
	f.periodosRecibidos = periods
	return f.cargues, f.err
}

func (f *servicioSdlFake) Audit(_ context.Context, periods []string) (tarifas_sdl.AuditResult, error) {
	f.periodosRecibidos = periods
	return f.auditoria, f.err
}

func motorSdl(servicio tarifas_sdl.TarifasSdlService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	controller := controllers.NewLiquidationsSdlController(servicio)

	motor := gin.New()
	grupo := motor.Group("/ms-bill/liquidations/tarifas-sdl")
	grupo.POST("/preview", controller.Preview)
	grupo.POST("/confirm", controller.Confirm)
	grupo.GET("", controller.Rates)
	grupo.GET("/periods", controller.Periods)
	grupo.GET("/loads", controller.Loads)
	grupo.GET("/audit", controller.Audit)

	return motor
}

func multipartSdl(t *testing.T, archivos map[string][]byte) (*bytes.Buffer, string) {
	t.Helper()

	cuerpo := &bytes.Buffer{}
	escritor := multipart.NewWriter(cuerpo)
	for nombre, contenido := range archivos {
		parte, err := escritor.CreateFormFile("files", nombre)
		if err != nil {
			t.Fatalf("CreateFormFile(%s): %v", nombre, err)
		}
		if _, err := parte.Write(contenido); err != nil {
			t.Fatalf("Write(%s): %v", nombre, err)
		}
	}
	if err := escritor.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	return cuerpo, escritor.FormDataContentType()
}

// ── Preview ─────────────────────────────────────────────────────────────────

func TestPreviewSdl_SinArchivos(t *testing.T) {
	cuerpo, contentType := multipartSdl(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/ms-bill/liquidations/tarifas-sdl/preview", cuerpo)
	req.Header.Set("Content-Type", contentType)

	w := httptest.NewRecorder()
	motorSdl(&servicioSdlFake{}).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("código = %d, se esperaba 400: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "al menos un archivo") {
		t.Errorf("el mensaje no orienta: %s", w.Body.String())
	}
}

// El lote son 33 archivos y el parser los clasifica POR EL NOMBRE, así que el
// nombre y el contenido de cada uno tienen que llegar completos al servicio.
func TestPreviewSdl_PasaLosArchivosConSuNombre(t *testing.T) {
	servicio := &servicioSdlFake{}

	cuerpo, contentType := multipartSdl(t, map[string][]byte{
		"LiquidacionDefinitivosCentroNivel1_202602.xlsx":    []byte("add"),
		"Cargo_Cobro_Uso_Red-DefinitivoCALM-202604.xlsx":    []byte("uso"),
		"LiquidacionDefinitivosOccidenteNivel2_202602.xlsx": []byte("otro"),
	})
	req := httptest.NewRequest(http.MethodPost, "/ms-bill/liquidations/tarifas-sdl/preview", cuerpo)
	req.Header.Set("Content-Type", contentType)

	w := httptest.NewRecorder()
	motorSdl(servicio).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("código = %d: %s", w.Code, w.Body.String())
	}
	if len(servicio.archivos) != 3 {
		t.Fatalf("el servicio recibió %d archivos, se esperaban 3", len(servicio.archivos))
	}
	for _, a := range servicio.archivos {
		if a.Name == "" {
			t.Error("un archivo llegó sin nombre; el parser clasifica por el nombre")
		}
		if len(a.Content) == 0 {
			t.Errorf("el archivo %s llegó vacío", a.Name)
		}
	}
}

// Con errores críticos el lote no se puede cargar: 422 y no 200. Si fuera 200, el
// front tendría que inspeccionar el cuerpo para darse cuenta.
func TestPreviewSdl_ErroresCriticosDan422(t *testing.T) {
	servicio := &servicioSdlFake{resultado: tarifas_sdl.PreviewResult{
		CriticalErrors: []string{"Falta el archivo ADD de CENTRO para el nivel 2."},
	}}

	cuerpo, contentType := multipartSdl(t, map[string][]byte{"x.xlsx": []byte("x")})
	req := httptest.NewRequest(http.MethodPost, "/ms-bill/liquidations/tarifas-sdl/preview", cuerpo)
	req.Header.Set("Content-Type", contentType)

	w := httptest.NewRecorder()
	motorSdl(servicio).ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("código = %d, se esperaba 422", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Falta el archivo ADD") {
		t.Errorf("el motivo no viaja: %s", w.Body.String())
	}
}

func TestPreviewSdl_ErrorDelServicioDa500(t *testing.T) {
	servicio := &servicioSdlFake{err: errors.New("algo se rompió")}

	cuerpo, contentType := multipartSdl(t, map[string][]byte{"x.xlsx": []byte("x")})
	req := httptest.NewRequest(http.MethodPost, "/ms-bill/liquidations/tarifas-sdl/preview", cuerpo)
	req.Header.Set("Content-Type", contentType)

	w := httptest.NewRecorder()
	motorSdl(servicio).ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("código = %d, se esperaba 500", w.Code)
	}
	if !strings.Contains(w.Body.String(), "algo se rompió") {
		t.Errorf("el 500 no lleva la causa: %s", w.Body.String())
	}
}

// ── Confirm ─────────────────────────────────────────────────────────────────

// El período es obligatorio: sin él no se sabe a qué mes pertenece el lote, y el
// modelo append-only no permite corregirlo después sin recargar.
func TestConfirmSdl_ElPeriodoEsObligatorio(t *testing.T) {
	casos := map[string]string{
		"sin período": `{"rows":[]}`,
		"JSON roto":   `{"period":`,
	}

	for nombre, cuerpo := range casos {
		t.Run(nombre, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost,
				"/ms-bill/liquidations/tarifas-sdl/confirm", strings.NewReader(cuerpo))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			motorSdl(&servicioSdlFake{}).ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("código = %d, se esperaba 400: %s", w.Code, w.Body.String())
			}
		})
	}
}

// El id del usuario sale del header y NO del cuerpo: es el único dato de identidad
// que quien arma el request no puede falsear. El nombre sí viene del cuerpo y solo
// sirve para mostrar.
func TestConfirmSdl_LaIdentidadSaleDelHeader(t *testing.T) {
	servicio := &servicioSdlFake{loadID: "carga-1"}

	cuerpo := `{"period":"2026-01","created_by":"Erika Ramírez",
	            "rows":[{"operator_code":"CENS","dt1":1,"dt2":1,"dt3":1,"cdi":1,"cdn4":1,
	                     "pr1":0.1,"pr2":0.1,"pr3":0.1}]}`

	req := httptest.NewRequest(http.MethodPost,
		"/ms-bill/liquidations/tarifas-sdl/confirm", strings.NewReader(cuerpo))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-user-id", "usuario-del-header")

	w := httptest.NewRecorder()
	motorSdl(servicio).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("código = %d: %s", w.Code, w.Body.String())
	}
	if servicio.periodoRecibido != "2026-01" {
		t.Errorf("el período llegó como %q", servicio.periodoRecibido)
	}
	if servicio.metaRecibida.CreatedByID != "usuario-del-header" {
		t.Errorf("created_by_id = %q; debe salir del header", servicio.metaRecibida.CreatedByID)
	}
	if servicio.metaRecibida.CreatedBy != "Erika Ramírez" {
		t.Errorf("created_by = %q", servicio.metaRecibida.CreatedBy)
	}
}

func TestConfirmSdl_ErrorDelServicioLlevaLaCausa(t *testing.T) {
	servicio := &servicioSdlFake{err: errors.New("el lote no cubre a todos los operadores de red")}

	cuerpo := `{"period":"2026-01","rows":[{"operator_code":"CENS"}]}`
	req := httptest.NewRequest(http.MethodPost,
		"/ms-bill/liquidations/tarifas-sdl/confirm", strings.NewReader(cuerpo))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	motorSdl(servicio).ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("código = %d, se esperaba 500", w.Code)
	}
	if !strings.Contains(w.Body.String(), "no cubre a todos") {
		t.Errorf("el motivo no viaja al front: %s", w.Body.String())
	}
}

// ── Lecturas ────────────────────────────────────────────────────────────────

func TestRatesSdl_FiltraYDevuelve(t *testing.T) {
	servicio := &servicioSdlFake{tarifas: []repositories.SdlRate{
		{Period: "2026-01", OperatorCode: "CENS", OperatorName: "CENS S.A.", Market: "NORTE DE SANTANDER",
			ActiveLevel1Operator: 276.6876},
	}}

	// Con espacio después de la coma, como lo manda un navegador.
	req := httptest.NewRequest(http.MethodGet,
		"/ms-bill/liquidations/tarifas-sdl?periods=2026-01,%202025-12&operators=CENS", nil)

	w := httptest.NewRecorder()
	motorSdl(servicio).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("código = %d: %s", w.Code, w.Body.String())
	}
	if len(servicio.filtroRecibido.Periods) != 2 || servicio.filtroRecibido.Periods[1] != "2025-12" {
		t.Errorf("el filtro de períodos llegó %q; el espacio debería recortarse",
			servicio.filtroRecibido.Periods)
	}
	if len(servicio.filtroRecibido.OperatorCodes) != 1 {
		t.Errorf("el filtro de operadores llegó %q", servicio.filtroRecibido.OperatorCodes)
	}

	var respuesta struct {
		Rates []repositories.SdlRate `json:"rates"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &respuesta); err != nil {
		t.Fatalf("respuesta ilegible: %v", err)
	}
	if len(respuesta.Rates) != 1 || respuesta.Rates[0].Market != "NORTE DE SANTANDER" {
		t.Errorf("tarifas = %+v", respuesta.Rates)
	}
}

func TestPeriodsSdl(t *testing.T) {
	servicio := &servicioSdlFake{periodos: []string{"2026-01", "2025-12"}}
	req := httptest.NewRequest(http.MethodGet, "/ms-bill/liquidations/tarifas-sdl/periods", nil)

	w := httptest.NewRecorder()
	motorSdl(servicio).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("código = %d", w.Code)
	}

	var periodos []string
	if err := json.Unmarshal(w.Body.Bytes(), &periodos); err != nil {
		t.Fatalf("no es un array JSON: %v", err)
	}
	if len(periodos) != 2 || periodos[0] != "2026-01" {
		t.Errorf("períodos = %v", periodos)
	}
}

func TestLoadsSdl(t *testing.T) {
	servicio := &servicioSdlFake{cargues: []repositories.SdlLoad{
		{LoadID: "c-1", Period: "2026-01", Operators: 21},
	}}
	req := httptest.NewRequest(http.MethodGet, "/ms-bill/liquidations/tarifas-sdl/loads?periods=2026-01", nil)

	w := httptest.NewRecorder()
	motorSdl(servicio).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("código = %d", w.Code)
	}
	if len(servicio.periodosRecibidos) != 1 {
		t.Errorf("períodos recibidos = %q", servicio.periodosRecibidos)
	}

	var respuesta struct {
		Loads []repositories.SdlLoad `json:"loads"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &respuesta); err != nil {
		t.Fatalf("respuesta ilegible: %v", err)
	}
	if len(respuesta.Loads) != 1 || respuesta.Loads[0].Operators != 21 {
		t.Errorf("cargues = %+v", respuesta.Loads)
	}
}

// ── Auditoría ───────────────────────────────────────────────────────────────

// El código de estado es la señal: 200 si todo cuadra, 409 si no. Así un chequeo
// automático se da cuenta sin leer el cuerpo.
func TestAuditSdl_CodigoSegunElResultado(t *testing.T) {
	casos := []struct {
		nombre    string
		auditoria tarifas_sdl.AuditResult
		codigo    int
	}{
		{
			nombre:    "todo cuadra",
			auditoria: tarifas_sdl.AuditResult{Checked: 21},
			codigo:    http.StatusOK,
		},
		{
			nombre: "una tarifa no sale de sus componentes",
			auditoria: tarifas_sdl.AuditResult{Checked: 21, Findings: []tarifas_sdl.AuditFinding{
				{Period: "2026-01", OperatorCode: "CENS", Column: "active_level_1_user",
					Stored: 222.91, Recomputed: 223.41},
			}},
			codigo: http.StatusConflict,
		},
		{
			nombre: "hay huérfanos",
			auditoria: tarifas_sdl.AuditResult{Checked: 20,
				Orphans: []string{"2026-01|CHEC (componentes sin tarifa)"}},
			codigo: http.StatusConflict,
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			servicio := &servicioSdlFake{auditoria: caso.auditoria}
			req := httptest.NewRequest(http.MethodGet, "/ms-bill/liquidations/tarifas-sdl/audit", nil)

			w := httptest.NewRecorder()
			motorSdl(servicio).ServeHTTP(w, req)

			if w.Code != caso.codigo {
				t.Errorf("código = %d, se esperaba %d: %s", w.Code, caso.codigo, w.Body.String())
			}

			// El cuerpo tiene que traer el detalle en los dos casos: un 409 sin
			// detalle no sirve para investigar.
			var res tarifas_sdl.AuditResult
			if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
				t.Fatalf("respuesta ilegible: %v", err)
			}
			if res.Checked != caso.auditoria.Checked {
				t.Errorf("checked = %d, se esperaba %d", res.Checked, caso.auditoria.Checked)
			}
			if len(res.Findings) != len(caso.auditoria.Findings) {
				t.Errorf("hallazgos = %+v", res.Findings)
			}
		})
	}
}

func TestAuditSdl_ErrorDeLecturaDa500(t *testing.T) {
	servicio := &servicioSdlFake{err: errors.New("base caída")}
	req := httptest.NewRequest(http.MethodGet, "/ms-bill/liquidations/tarifas-sdl/audit", nil)

	w := httptest.NewRecorder()
	motorSdl(servicio).ServeHTTP(w, req)

	// 500 y no 409: no se pudo verificar, que es distinto de "encontré un problema".
	if w.Code != http.StatusInternalServerError {
		t.Errorf("código = %d, se esperaba 500", w.Code)
	}
}
