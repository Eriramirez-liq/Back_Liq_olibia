package repositories_test

import (
	"context"
	"strings"
	"testing"

	"bia-bills/providers/postgres"
	"bia-bills/repositories"

	"github.com/DATA-DOG/go-sqlmock"
	postgresGorm "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Nombres por código de agente, y los filtros de las dos lecturas de SDL que
// quedaban sin cubrir.
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_`. Ver docs/backend/migracion-a-go.md.
//
// NamesByAgentCode es la consulta que hace que un operador se llame IGUAL en las
// tablas de los dos módulos: mismo catálogo, mismo filtro de actividad. Si se le
// escapara el filtro, Air-e aparecería como "AIR- E S.A.S. E.S.P. - INTERVENIDO"
// en Tarifas SDL y con el nombre limpio en Cargos STR.

// mocksAgentesConCaptura arma el repositorio de agentes con las dos conexiones
// falsas y el SQL capturado.
func mocksAgentesConCaptura(t *testing.T) (
	repositories.LiquidationsAgentsRepository, sqlmock.Sqlmock, sqlmock.Sqlmock, func() []string,
) {
	t.Helper()

	ejecutado := []string{}
	espia := sqlmock.QueryMatcherFunc(func(esperado, actual string) error {
		ejecutado = append(ejecutado, actual)
		return sqlmock.QueryMatcherRegexp.Match(esperado, actual)
	})

	abrir := func() (*gorm.DB, sqlmock.Sqlmock) {
		sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(espia))
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

	return repositories.NewLiquidationsAgentsRepository(db), mockFC, mockCP,
		func() []string { return ejecutado }
}

func TestUnitAgentes_NamesByAgentCode(t *testing.T) {
	repo, mockFC, mockCP, sqlEjecutado := mocksAgentesConCaptura(t)

	mockFC.ExpectQuery(`FROM public.agents`).
		WillReturnRows(sqlmock.NewRows([]string{"code", "name"}).
			AddRow("CNSD", "CENTRALES ELECTRICAS DEL NORTE DE SANTANDER S.A. E.S.P.").
			AddRow("  cssd  ", "AIR- E S.A.S. E.S.P.").
			AddRow("EPSD", ""))

	nombres, err := repo.NamesByAgentCode(context.Background(), []string{"CNSD", "CSSD", "EPSD"})
	if err != nil {
		t.Fatalf("NamesByAgentCode: %v", err)
	}

	if nombres["CNSD"] == "" {
		t.Error("CNSD debería resolver nombre")
	}
	// Normaliza el código que vuelve: sin eso, un espacio en la base pierde el
	// nombre y el operador queda sin identificar.
	if nombres["CSSD"] != "AIR- E S.A.S. E.S.P." {
		t.Errorf("no normalizó el código con espacios y minúsculas: %+v", nombres)
	}
	// Un nombre vacío no se guarda: el servicio necesita distinguir "sin nombre"
	// para cortar la carga.
	if _, hay := nombres["EPSD"]; hay {
		t.Error("guardó un nombre vacío como si fuera un nombre")
	}

	query := sqlEjecutado()[0]
	for _, fragmento := range []string{"upper(trim(code)) IN (", "activity = ", "deleted_at IS NULL"} {
		if !strings.Contains(query, fragmento) {
			t.Errorf("al SQL le falta %q: %s", fragmento, query)
		}
	}
	if strings.Contains(query, "ANY(") {
		t.Errorf("volvió el ANY(?) que GORM no expande: %s", query)
	}

	if err := mockCP.ExpectationsWereMet(); err != nil {
		t.Errorf("calculator-prices recibió tráfico que no le corresponde: %v", err)
	}
}

func TestUnitAgentes_NamesByAgentCodeSinCodigosNoConsulta(t *testing.T) {
	repo, mockFC, _, _ := mocksAgentesConCaptura(t)

	nombres, err := repo.NamesByAgentCode(context.Background(), nil)
	if err != nil {
		t.Fatalf("NamesByAgentCode: %v", err)
	}
	if len(nombres) != 0 {
		t.Errorf("devolvió %d nombres sin pedir códigos", len(nombres))
	}
	if err := mockFC.ExpectationsWereMet(); err != nil {
		t.Errorf("file-compiler: %v", err)
	}
}

// Los filtros por período de las dos lecturas de SDL que faltaban cubrir.
func TestUnitSDL_CurrentInputsYLoadsFiltranPorPeriodo(t *testing.T) {
	casos := map[string]func(repositories.LiquidationsSdlRepository) error{
		"CurrentInputs": func(r repositories.LiquidationsSdlRepository) error {
			_, err := r.CurrentInputs(context.Background(), []string{"2026-01", "2025-12"})
			return err
		},
		"Loads": func(r repositories.LiquidationsSdlRepository) error {
			_, err := r.Loads(context.Background(), []string{"2026-01", "2025-12"})
			return err
		},
	}

	for nombre, leer := range casos {
		t.Run(nombre, func(t *testing.T) {
			repo, mockFC, _, sqlEjecutado := mocksSDL(t)
			mockFC.ExpectQuery(`.*`).WillReturnRows(sqlmock.NewRows([]string{"period"}))

			if err := leer(repo); err != nil {
				t.Fatalf("%s: %v", nombre, err)
			}

			query := sqlEjecutado()[0]
			if !strings.Contains(query, "WHERE period IN (") {
				t.Errorf("no filtró por período: %s", query)
			}
			// Dos períodos, dos placeholders: si GORM no expandiera el slice habría
			// uno solo y Postgres devolvería un error de sintaxis.
			if n := strings.Count(query, "$"); n != 2 {
				t.Errorf("hay %d placeholders, se esperaban 2: %s", n, query)
			}
		})
	}
}
