package controllers_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bia-bills/controllers"
	"bia-bills/providers/postgres"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	postgresGorm "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Tests del diagnóstico del módulo.
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_`. Ver docs/backend/migracion-a-go.md.
//
// Este endpoint es lo primero que se va a mirar cuando el módulo esté en cactus y
// algo no responda, así que tiene que decir la verdad: 503 con el detalle de CUÁL
// de las dos bases falló. Un 200 optimista mandaría a buscar el problema al lado
// equivocado.

func healthConMocks(t *testing.T) (controllers.LiquidationsHealthController, sqlmock.Sqlmock, sqlmock.Sqlmock) {
	t.Helper()

	abrir := func() (*gorm.DB, sqlmock.Sqlmock) {
		sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
		if err != nil {
			t.Fatalf("sqlmock.New: %v", err)
		}
		t.Cleanup(func() { _ = sqlDB.Close() })

		gormDB, err := gorm.Open(
			postgresGorm.New(postgresGorm.Config{Conn: sqlDB}),
			&gorm.Config{DisableAutomaticPing: true},
		)
		if err != nil {
			t.Fatalf("gorm.Open: %v", err)
		}

		return gormDB, mock
	}

	fileCompiler, mockFC := abrir()
	calculatorPrices, mockCP := abrir()

	db := postgres.NewLiquidationsDB(map[postgres.LiquidationsDatabase]*gorm.DB{
		postgres.LiqDBFileCompiler:     fileCompiler,
		postgres.LiqDBCalculatorPrices: calculatorPrices,
	})

	return controllers.NewLiquidationsHealthController(db), mockFC, mockCP
}

func llamarHealth(t *testing.T, controller controllers.LiquidationsHealthController) (int, map[string]any) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/health", controller.Health)

	grabadora := httptest.NewRecorder()
	engine.ServeHTTP(grabadora, httptest.NewRequest(http.MethodGet, "/health", nil))

	var cuerpo map[string]any
	if err := json.Unmarshal(grabadora.Body.Bytes(), &cuerpo); err != nil {
		t.Fatalf("respuesta ilegible (%d): %s", grabadora.Code, grabadora.Body.String())
	}

	return grabadora.Code, cuerpo
}

func TestHealth_LasDosBasesResponden(t *testing.T) {
	controller, mockFC, mockCP := healthConMocks(t)

	mockFC.ExpectExec("SELECT 1").WillReturnResult(sqlmock.NewResult(0, 0))
	mockCP.ExpectExec("SELECT 1").WillReturnResult(sqlmock.NewResult(0, 0))

	codigo, cuerpo := llamarHealth(t, controller)

	if codigo != http.StatusOK {
		t.Errorf("código = %d, se esperaba 200: %+v", codigo, cuerpo)
	}
	if cuerpo["ok"] != true {
		t.Errorf("ok = %v", cuerpo["ok"])
	}

	bases, _ := cuerpo["bases"].([]any)
	if len(bases) != 2 {
		t.Fatalf("reportó %d bases, se esperaban 2: %+v", len(bases), cuerpo)
	}
	for _, b := range bases {
		base, _ := b.(map[string]any)
		if base["ok"] != true {
			t.Errorf("%v quedó marcada como caída: %+v", base["base"], base)
		}
	}
}

// Con una sola base caída tiene que responder 503 y decir cuál. Si dijera "todo
// mal" sin distinguir, habría que revisar las dos a mano.
func TestHealth_UnaBaseCaidaDa503YSeñalaCual(t *testing.T) {
	controller, mockFC, mockCP := healthConMocks(t)

	mockFC.ExpectExec("SELECT 1").WillReturnResult(sqlmock.NewResult(0, 0))
	mockCP.ExpectExec("SELECT 1").WillReturnError(errors.New("connection refused"))

	codigo, cuerpo := llamarHealth(t, controller)

	if codigo != http.StatusServiceUnavailable {
		t.Errorf("código = %d, se esperaba 503: %+v", codigo, cuerpo)
	}
	if cuerpo["ok"] != false {
		t.Errorf("ok = %v con una base caída", cuerpo["ok"])
	}

	bases, _ := cuerpo["bases"].([]any)
	porNombre := map[string]map[string]any{}
	for _, b := range bases {
		base, _ := b.(map[string]any)
		porNombre[base["base"].(string)] = base
	}

	if porNombre["file-compiler"]["ok"] != true {
		t.Errorf("file-compiler respondía y quedó marcada como caída: %+v", porNombre["file-compiler"])
	}
	if porNombre["calculator-prices"]["ok"] != false {
		t.Errorf("calculator-prices estaba caída y quedó como OK: %+v", porNombre["calculator-prices"])
	}

	// El motivo tiene que viajar: sin él, el 503 no dice nada útil.
	motivo, _ := porNombre["calculator-prices"]["error"].(string)
	if !strings.Contains(motivo, "connection refused") {
		t.Errorf("no reportó la causa: %q", motivo)
	}
}

// Aunque la primera base falle, hay que consultar la segunda: el diagnóstico
// completo en una sola llamada es todo el punto de este endpoint.
func TestHealth_ConsultaLasDosAunqueLaPrimeraFalle(t *testing.T) {
	controller, mockFC, mockCP := healthConMocks(t)

	mockFC.ExpectExec("SELECT 1").WillReturnError(errors.New("timeout"))
	mockCP.ExpectExec("SELECT 1").WillReturnResult(sqlmock.NewResult(0, 0))

	codigo, cuerpo := llamarHealth(t, controller)

	if codigo != http.StatusServiceUnavailable {
		t.Errorf("código = %d, se esperaba 503", codigo)
	}
	// Si no hubiera consultado la segunda, esta expectativa quedaría incumplida.
	if err := mockCP.ExpectationsWereMet(); err != nil {
		t.Errorf("no consultó calculator-prices tras fallar la primera: %v", err)
	}

	bases, _ := cuerpo["bases"].([]any)
	if len(bases) != 2 {
		t.Errorf("reportó %d bases: %+v", len(bases), cuerpo)
	}
}
