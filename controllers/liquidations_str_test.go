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
	"bia-bills/services/cargos_str"

	"github.com/gin-gonic/gin"
)

// Tests de la capa HTTP. No tocan base: el servicio es un doble.
//
// Cubren lo que solo se puede romper acá: el parseo del multipart, la validación
// de parámetros y los códigos de estado. Un 200 con el cuerpo equivocado o un 500
// donde correspondía un 400 son bugs que el front paga.

type servicioFake struct {
	resultado cargos_str.ParseResult
	loadID    string
	cargos    []repositories.StrCharge
	periodos  []string
	cargues   []repositories.StrLoad
	err       error

	filtroRecibido    repositories.StrChargeFilter
	yearRecibido      int
	monthRecibido     int
	archivos          []cargos_str.UploadedFile
	metaRecibida      cargos_str.LoadMeta
	periodosRecibidos []string
}

func (f *servicioFake) Preview(_ context.Context, files []cargos_str.UploadedFile, year, month int) (cargos_str.ParseResult, error) {
	f.archivos, f.yearRecibido, f.monthRecibido = files, year, month
	return f.resultado, f.err
}

func (f *servicioFake) Confirm(_ context.Context, _ []cargos_str.StrRow, meta cargos_str.LoadMeta) (string, error) {
	f.metaRecibida = meta
	return f.loadID, f.err
}

func (f *servicioFake) Loads(_ context.Context, periods []string) ([]repositories.StrLoad, error) {
	f.periodosRecibidos = periods
	return f.cargues, f.err
}

func (f *servicioFake) CurrentCharges(_ context.Context, filtro repositories.StrChargeFilter) ([]repositories.StrCharge, error) {
	f.filtroRecibido = filtro
	return f.cargos, f.err
}

func (f *servicioFake) TotalsByPeriod(context.Context, []string) (map[string]float64, error) {
	return nil, f.err
}

func (f *servicioFake) Periods(context.Context) ([]string, error) {
	return f.periodos, f.err
}

func motorDePrueba(servicio cargos_str.CargosStrService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	controller := controllers.NewLiquidationsStrController(servicio)

	motor := gin.New()
	grupo := motor.Group("/ms-bill/liquidations/cargos-str")
	grupo.POST("/preview", controller.Preview)
	grupo.POST("/confirm", controller.Confirm)
	grupo.GET("", controller.Charges)
	grupo.GET("/periods", controller.Periods)
	grupo.GET("/loads", controller.Loads)

	return motor
}

// multipartDePrueba arma el cuerpo tal como lo manda el wizard del front.
func multipartDePrueba(t *testing.T, campos map[string]string, archivos map[string][]byte) (*bytes.Buffer, string) {
	t.Helper()

	cuerpo := &bytes.Buffer{}
	escritor := multipart.NewWriter(cuerpo)

	for clave, valor := range campos {
		if err := escritor.WriteField(clave, valor); err != nil {
			t.Fatalf("WriteField(%s): %v", clave, err)
		}
	}
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

func TestPreview_Parametros(t *testing.T) {
	archivo := map[string][]byte{"BalanceSTRTipoFactu2026-MAY.xlsx": []byte("x")}

	casos := []struct {
		nombre       string
		campos       map[string]string
		archivos     map[string][]byte
		codigo       int
		mensajeTiene string
	}{
		{
			nombre: "sin year", campos: map[string]string{"month": "5"}, archivos: archivo,
			codigo: http.StatusBadRequest, mensajeTiene: "falta el parámetro year",
		},
		{
			nombre: "sin month", campos: map[string]string{"year": "2026"}, archivos: archivo,
			codigo: http.StatusBadRequest, mensajeTiene: "falta el parámetro month",
		},
		{
			nombre: "year fuera de rango", campos: map[string]string{"year": "1999", "month": "5"}, archivos: archivo,
			codigo: http.StatusBadRequest, mensajeTiene: "fuera del rango",
		},
		{
			nombre: "month fuera de rango", campos: map[string]string{"year": "2026", "month": "13"}, archivos: archivo,
			codigo: http.StatusBadRequest, mensajeTiene: "fuera del rango",
		},
		{
			nombre: "year no numérico", campos: map[string]string{"year": "abc", "month": "5"}, archivos: archivo,
			codigo: http.StatusBadRequest, mensajeTiene: "no es un número",
		},
		{
			nombre: "sin archivos", campos: map[string]string{"year": "2026", "month": "5"}, archivos: nil,
			codigo: http.StatusBadRequest, mensajeTiene: "al menos un archivo",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			cuerpo, contentType := multipartDePrueba(t, caso.campos, caso.archivos)
			req := httptest.NewRequest(http.MethodPost, "/ms-bill/liquidations/cargos-str/preview", cuerpo)
			req.Header.Set("Content-Type", contentType)

			w := httptest.NewRecorder()
			motorDePrueba(&servicioFake{}).ServeHTTP(w, req)

			if w.Code != caso.codigo {
				t.Errorf("código = %d, se esperaba %d — cuerpo: %s", w.Code, caso.codigo, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), caso.mensajeTiene) {
				t.Errorf("el mensaje no menciona %q: %s", caso.mensajeTiene, w.Body.String())
			}
		})
	}
}

func TestPreview_PasaElPeriodoYLosArchivosAlServicio(t *testing.T) {
	servicio := &servicioFake{resultado: cargos_str.ParseResult{
		Rows: []cargos_str.StrRow{{OperatorCode: "CHEC", AmountPayable: 70_807_698}},
	}}

	cuerpo, contentType := multipartDePrueba(t,
		map[string]string{"year": "2026", "month": "5"},
		map[string][]byte{"factu.xlsx": []byte("contenido"), "refactu.xlsx": []byte("otro")})

	req := httptest.NewRequest(http.MethodPost, "/ms-bill/liquidations/cargos-str/preview", cuerpo)
	req.Header.Set("Content-Type", contentType)

	w := httptest.NewRecorder()
	motorDePrueba(servicio).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("código = %d: %s", w.Code, w.Body.String())
	}
	if servicio.yearRecibido != 2026 || servicio.monthRecibido != 5 {
		t.Errorf("el servicio recibió %d-%d", servicio.yearRecibido, servicio.monthRecibido)
	}
	if len(servicio.archivos) != 2 {
		t.Fatalf("el servicio recibió %d archivos, se esperaban 2", len(servicio.archivos))
	}
	// El nombre importa: el parser clasifica factura y ajustes por el nombre.
	for _, a := range servicio.archivos {
		if a.Name == "" {
			t.Error("un archivo llegó sin nombre")
		}
		if len(a.Content) == 0 {
			t.Errorf("el archivo %s llegó vacío", a.Name)
		}
	}
}

func TestPreview_ErroresCriticosDan422(t *testing.T) {
	servicio := &servicioFake{resultado: cargos_str.ParseResult{
		CriticalErrors: []string{"El lote trae 4 archivos de refactura y el máximo admitido es 3."},
	}}

	cuerpo, contentType := multipartDePrueba(t,
		map[string]string{"year": "2026", "month": "5"},
		map[string][]byte{"factu.xlsx": []byte("x")})

	req := httptest.NewRequest(http.MethodPost, "/ms-bill/liquidations/cargos-str/preview", cuerpo)
	req.Header.Set("Content-Type", contentType)

	w := httptest.NewRecorder()
	motorDePrueba(servicio).ServeHTTP(w, req)

	// 422 y no 200: si fuera 200, el front tendría que inspeccionar el cuerpo para
	// darse cuenta de que el lote no se puede cargar.
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("código = %d, se esperaba 422", w.Code)
	}
	if !strings.Contains(w.Body.String(), "máximo admitido") {
		t.Errorf("no devolvió el detalle del error: %s", w.Body.String())
	}
}

func TestConfirm(t *testing.T) {
	t.Run("devuelve el load_id y la cantidad de filas", func(t *testing.T) {
		servicio := &servicioFake{loadID: "abc-123"}
		cuerpo := `{"rows":[{"operator_code":"CHEC","period":"2026-05","amount_payable":100}]}`

		req := httptest.NewRequest(http.MethodPost, "/ms-bill/liquidations/cargos-str/confirm",
			strings.NewReader(cuerpo))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		motorDePrueba(servicio).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("código = %d: %s", w.Code, w.Body.String())
		}
		var res map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("respuesta no es JSON: %v", err)
		}
		if res["load_id"] != "abc-123" {
			t.Errorf("load_id = %v", res["load_id"])
		}
		if res["rows"] != float64(1) {
			t.Errorf("rows = %v", res["rows"])
		}
	})

	t.Run("JSON malformado da 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/ms-bill/liquidations/cargos-str/confirm",
			strings.NewReader("{no es json"))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		motorDePrueba(&servicioFake{}).ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("código = %d, se esperaba 400", w.Code)
		}
	})

	t.Run("si el servicio falla, 500 con el motivo", func(t *testing.T) {
		servicio := &servicioFake{err: errors.New("no se pudo resolver el nombre de CHEC")}
		cuerpo := `{"rows":[{"operator_code":"CHEC","period":"2026-05"}]}`

		req := httptest.NewRequest(http.MethodPost, "/ms-bill/liquidations/cargos-str/confirm",
			strings.NewReader(cuerpo))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		motorDePrueba(servicio).ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("código = %d, se esperaba 500", w.Code)
		}
		// El motivo tiene que llegar: un 500 opaco no se puede diagnosticar.
		if !strings.Contains(w.Body.String(), "CHEC") {
			t.Errorf("el 500 no dice qué falló: %s", w.Body.String())
		}
	})
}

func TestCharges(t *testing.T) {
	t.Run("parsea los filtros de la query y suma el total", func(t *testing.T) {
		servicio := &servicioFake{cargos: []repositories.StrCharge{
			{OperatorCode: "CHEC", AmountPayable: 70_807_698},
			{OperatorCode: "EPM", AmountPayable: 208_598_815},
		}}

		req := httptest.NewRequest(http.MethodGet,
			"/ms-bill/liquidations/cargos-str?periods=2026-07,+2026-06+&operators=CHEC,EPM", nil)

		w := httptest.NewRecorder()
		motorDePrueba(servicio).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("código = %d: %s", w.Code, w.Body.String())
		}

		// Los espacios alrededor de cada valor se descartan: si no, "2026-06 " no
		// matchearía ninguna fila y el filtro devolvería vacío en silencio.
		if len(servicio.filtroRecibido.Periods) != 2 ||
			servicio.filtroRecibido.Periods[0] != "2026-07" ||
			servicio.filtroRecibido.Periods[1] != "2026-06" {
			t.Errorf("períodos recibidos: %#v", servicio.filtroRecibido.Periods)
		}
		if len(servicio.filtroRecibido.OperatorCodes) != 2 {
			t.Errorf("operadores recibidos: %#v", servicio.filtroRecibido.OperatorCodes)
		}

		var res struct {
			Total float64 `json:"total"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("respuesta no es JSON: %v", err)
		}
		if res.Total != 279_406_513 {
			t.Errorf("total = %.0f, se esperaba 279.406.513", res.Total)
		}
	})

	t.Run("sin filtros no manda ninguno", func(t *testing.T) {
		servicio := &servicioFake{}
		req := httptest.NewRequest(http.MethodGet, "/ms-bill/liquidations/cargos-str", nil)

		w := httptest.NewRecorder()
		motorDePrueba(servicio).ServeHTTP(w, req)

		if servicio.filtroRecibido.Periods != nil || servicio.filtroRecibido.OperatorCodes != nil {
			t.Errorf("mandó filtros vacíos en vez de nada: %#v", servicio.filtroRecibido)
		}
	})

	t.Run("si el servicio falla, 500", func(t *testing.T) {
		servicio := &servicioFake{err: errors.New("base caída")}
		req := httptest.NewRequest(http.MethodGet, "/ms-bill/liquidations/cargos-str", nil)

		w := httptest.NewRecorder()
		motorDePrueba(servicio).ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("código = %d, se esperaba 500", w.Code)
		}
	})
}

func TestPeriods(t *testing.T) {
	servicio := &servicioFake{periodos: []string{"2026-07", "2026-06"}}
	req := httptest.NewRequest(http.MethodGet, "/ms-bill/liquidations/cargos-str/periods", nil)

	w := httptest.NewRecorder()
	motorDePrueba(servicio).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("código = %d", w.Code)
	}

	var periodos []string
	if err := json.Unmarshal(w.Body.Bytes(), &periodos); err != nil {
		t.Fatalf("respuesta no es un array JSON: %v", err)
	}
	if len(periodos) != 2 || periodos[0] != "2026-07" {
		t.Errorf("períodos = %v", periodos)
	}
}

// El id del usuario tiene que salir del header, NO del cuerpo: es el único dato
// de identidad que quien arma el request no puede falsear. El nombre sí viene
// del cuerpo, y por eso solo sirve para mostrar.
func TestConfirm_LaIdentidadSaleDelHeader(t *testing.T) {
	servicio := &servicioFake{loadID: "carga-1"}

	cuerpo := `{"rows":[{"operator_code":"CHEC","period":"2026-05","invoice_amount":100,"amount_payable":100}],
	            "created_by":"Erika Ramírez",
	            "source_files":["BalanceSTRTipoFactu2026-MAY.xlsx","BalanceSTRTipoReFactu2026-MAR-1.xlsx"]}`

	req := httptest.NewRequest(http.MethodPost, "/ms-bill/liquidations/cargos-str/confirm", strings.NewReader(cuerpo))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-user-id", "usuario-del-header")

	w := httptest.NewRecorder()
	motorDePrueba(servicio).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("código = %d: %s", w.Code, w.Body.String())
	}
	if servicio.metaRecibida.CreatedByID != "usuario-del-header" {
		t.Errorf("created_by_id = %q; debe salir del header", servicio.metaRecibida.CreatedByID)
	}
	if servicio.metaRecibida.CreatedBy != "Erika Ramírez" {
		t.Errorf("created_by = %q", servicio.metaRecibida.CreatedBy)
	}
	if len(servicio.metaRecibida.SourceFiles) != 2 {
		t.Errorf("archivos = %v", servicio.metaRecibida.SourceFiles)
	}
}

// Sin metadatos el cargue se guarda igual. Perder el historial no justifica
// rechazar una carga válida.
func TestConfirm_SinMetadatosGuardaIgual(t *testing.T) {
	servicio := &servicioFake{loadID: "carga-1"}

	cuerpo := `{"rows":[{"operator_code":"CHEC","period":"2026-05","invoice_amount":100,"amount_payable":100}]}`
	req := httptest.NewRequest(http.MethodPost, "/ms-bill/liquidations/cargos-str/confirm", strings.NewReader(cuerpo))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	motorDePrueba(servicio).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("código = %d: %s", w.Code, w.Body.String())
	}
}

func TestLoads(t *testing.T) {
	servicio := &servicioFake{cargues: []repositories.StrLoad{
		{LoadID: "c-2", Period: "2026-07", CreatedBy: "Erika", SourceFiles: "a.xlsx", Operators: 23},
		{LoadID: "c-1", Period: "2026-06", Operators: 23},
	}}

	req := httptest.NewRequest(http.MethodGet, "/ms-bill/liquidations/cargos-str/loads?periods=2026-07,%202026-06", nil)
	w := httptest.NewRecorder()
	motorDePrueba(servicio).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("código = %d: %s", w.Code, w.Body.String())
	}

	// El filtro llega limpio, sin el espacio de la coma.
	if len(servicio.periodosRecibidos) != 2 || servicio.periodosRecibidos[1] != "2026-06" {
		t.Errorf("períodos recibidos = %q", servicio.periodosRecibidos)
	}

	var respuesta struct {
		Loads []repositories.StrLoad `json:"loads"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &respuesta); err != nil {
		t.Fatalf("respuesta ilegible: %v", err)
	}
	if len(respuesta.Loads) != 2 || respuesta.Loads[0].LoadID != "c-2" {
		t.Errorf("cargues = %+v", respuesta.Loads)
	}
	// Una carga vieja, sin metadatos, viaja con los campos vacíos y no rompe.
	if respuesta.Loads[1].CreatedBy != "" {
		t.Errorf("esperaba sin usuario: %+v", respuesta.Loads[1])
	}
}
