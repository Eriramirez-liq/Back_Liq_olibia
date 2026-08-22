package repositories_test

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"bia-bills/models"
	"bia-bills/providers/postgres"
	"bia-bills/repositories"

	"github.com/DATA-DOG/go-sqlmock"
	postgresGorm "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Tests del repositorio de TC1 con sqlmock, SIN base de datos.
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_`. Ver docs/backend/migracion-a-go.md.
//
// Mismo motivo que en Cargos STR y Tarifas SDL: file-compiler es un RDS externo
// que el CI de bia-bills no alcanza, así que sin estos tests el paquete reporta
// 0% de cobertura allá.

func mocksTC1(t *testing.T) (repositories.LiquidationsTc1Repository, sqlmock.Sqlmock, func() []string) {
	t.Helper()

	ejecutado := []string{}
	espia := sqlmock.QueryMatcherFunc(func(esperado, actual string) error {
		ejecutado = append(ejecutado, actual)
		return sqlmock.QueryMatcherRegexp.Match(esperado, actual)
	})

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

	// TC1 usa una sola base. calculator-prices se registra igual porque el
	// provider espera el mapa completo, pero este repositorio no la toca —y que
	// no la toque es parte de lo que se prueba.
	db := postgres.NewLiquidationsDB(map[postgres.LiquidationsDatabase]*gorm.DB{
		postgres.LiqDBFileCompiler:     gormDB,
		postgres.LiqDBCalculatorPrices: gormDB,
	})

	return repositories.NewLiquidationsTc1Repository(db), mock, func() []string { return ejecutado }
}

func filaTc1() models.LiquidationsTc1Input {
	return models.LiquidationsTc1Input{
		ID: "in-1", LoadID: "load-1", Period: "2026-02", OperatorCode: "CENS",
		Niu:                  "1940639",
		CodFronteraComercial: "Frt32152",
		NivelDeTension:       "2",
		Latitud:              "35003989",
		Longitud:             "-765199207",
	}
}

func TestUnitTC1_InsertInputs_VaAFileCompiler(t *testing.T) {
	repositorio, mock, _ := mocksTC1(t)

	// Query y no Exec: la tabla genera el id por defecto, así que GORM cierra el
	// INSERT con RETURNING "id".
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "public"\."liquidations_tc1_inputs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("in-1"))
	mock.ExpectCommit()

	if err := repositorio.InsertInputs(context.Background(), []models.LiquidationsTc1Input{filaTc1()}); err != nil {
		t.Fatalf("InsertInputs: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("no se ejecutó lo esperado: %v", err)
	}
}

// Sin filas no se abre transacción: un cargue vacío no debería tocar la base.
func TestUnitTC1_InsertSinFilasNoConsulta(t *testing.T) {
	repositorio, mock, ejecutado := mocksTC1(t)

	if err := repositorio.InsertInputs(context.Background(), nil); err != nil {
		t.Fatalf("InsertInputs: %v", err)
	}
	if len(ejecutado()) != 0 {
		t.Errorf("consultó la base sin filas: %v", ejecutado())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectativas: %v", err)
	}
}

// La regla que distingue a TC1 de los otros dos módulos.
//
// Lo vigente es el ÚLTIMO CARGUE, no la última fila: hay muchas filas por período
// y operador —una por frontera— y desempatar fila por fila mezclaría dos cargues.
// El DISTINCT ON tiene que devolver load_id, y el WHERE de afuera filtrar por él.
func TestUnitTC1_CurrentInputs_TomaElUltimoCargueEntero(t *testing.T) {
	repositorio, mock, ejecutado := mocksTC1(t)

	mock.ExpectQuery(`SELECT \*\s+FROM public\.liquidations_tc1_inputs`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "load_id", "cod_frontera_comercial"}).
			AddRow("in-1", "load-9", "Frt32152"))

	if _, err := repositorio.CurrentInputs(context.Background(), repositories.Tc1Filter{}); err != nil {
		t.Fatalf("CurrentInputs: %v", err)
	}

	sql := strings.Join(ejecutado(), " ")
	normal := regexp.MustCompile(`\s+`).ReplaceAllString(sql, " ")

	if !strings.Contains(normal, "DISTINCT ON (period, operator_code) load_id") {
		t.Errorf("el subquery no elige el cargue vigente por período y operador: %s", normal)
	}
	if !strings.Contains(normal, "WHERE load_id IN (") {
		t.Errorf("no filtra por el cargue vigente, así que mezclaría cargues: %s", normal)
	}
	// Desempate estable: dos cargues en el mismo instante no pueden dar un
	// resultado distinto en cada corrida.
	if !strings.Contains(normal, "ORDER BY period, operator_code, created_at DESC, load_id DESC") {
		t.Errorf("el desempate del cargue vigente no es estable: %s", normal)
	}
}

// GORM no expande un slice dentro de ANY(?): genera ANY('a','b') y Postgres
// responde "syntax error at or near ','". Ya costó una sesión en Cargos STR.
func TestUnitTC1_FiltrosUsanInYNoAny(t *testing.T) {
	repositorio, mock, ejecutado := mocksTC1(t)

	mock.ExpectQuery(`SELECT \*`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("in-1"))

	_, err := repositorio.CurrentInputs(context.Background(), repositories.Tc1Filter{
		Periods:       []string{"2026-01", "2026-02"},
		OperatorCodes: []string{"CENS", "CHEC"},
	})
	if err != nil {
		t.Fatalf("CurrentInputs: %v", err)
	}

	sql := strings.Join(ejecutado(), " ")
	if strings.Contains(sql, "ANY(") {
		t.Errorf("usa ANY con un slice, que Postgres rechaza: %s", sql)
	}
	for _, esperado := range []string{"period IN (", "operator_code IN ("} {
		if !strings.Contains(sql, esperado) {
			t.Errorf("falta el filtro %q: %s", esperado, sql)
		}
	}
}

// El historial agrupa por cargue Y operador: cada OR manda su propio archivo, así
// que un cargue es de uno solo. Y lo que se cuenta son fronteras.
func TestUnitTC1_Loads_AgrupaPorCargueYOperador(t *testing.T) {
	repositorio, mock, ejecutado := mocksTC1(t)

	mock.ExpectQuery(`FROM public\.liquidations_tc1_inputs`).
		WillReturnRows(sqlmock.NewRows([]string{"load_id", "period", "operator_code", "borders"}).
			AddRow("load-1", "2026-02", "CENS", 156))

	cargues, err := repositorio.Loads(context.Background(), []string{"2026-02"})
	if err != nil {
		t.Fatalf("Loads: %v", err)
	}
	if len(cargues) != 1 || cargues[0].Borders != 156 {
		t.Errorf("no mapeó el cargue: %+v", cargues)
	}

	normal := regexp.MustCompile(`\s+`).ReplaceAllString(strings.Join(ejecutado(), " "), " ")
	if !strings.Contains(normal, "GROUP BY load_id, period, operator_code") {
		t.Errorf("no agrupa por operador, así que dos OR del mismo cargue se sumarían: %s", normal)
	}
	if !strings.Contains(normal, "COUNT(*) AS borders") {
		t.Errorf("no cuenta fronteras: %s", normal)
	}
	if !strings.Contains(normal, "ORDER BY MIN(created_at) DESC") {
		t.Errorf("el historial no viene del más reciente al más viejo: %s", normal)
	}
}

func TestUnitTC1_PeriodsMasRecientePrimero(t *testing.T) {
	repositorio, mock, ejecutado := mocksTC1(t)

	mock.ExpectQuery(`SELECT DISTINCT period`).
		WillReturnRows(sqlmock.NewRows([]string{"period"}).AddRow("2026-02").AddRow("2026-01"))

	periodos, err := repositorio.Periods(context.Background())
	if err != nil {
		t.Fatalf("Periods: %v", err)
	}
	if len(periodos) != 2 || periodos[0] != "2026-02" {
		t.Errorf("períodos inesperados: %v", periodos)
	}
	if !strings.Contains(strings.Join(ejecutado(), " "), "ORDER BY period DESC") {
		t.Errorf("no ordena del más reciente al más viejo")
	}
}

// Un error de la base no se puede tragar: sin esto, la pantalla mostraría una
// lista vacía como si no hubiera datos.
func TestUnitTC1_LecturasPropaganElError(t *testing.T) {
	fallo := errors.New("se cayó la conexión")

	casos := map[string]func(repositories.LiquidationsTc1Repository) error{
		"CurrentInputs": func(r repositories.LiquidationsTc1Repository) error {
			_, err := r.CurrentInputs(context.Background(), repositories.Tc1Filter{})
			return err
		},
		"Loads": func(r repositories.LiquidationsTc1Repository) error {
			_, err := r.Loads(context.Background(), nil)
			return err
		},
		"Periods": func(r repositories.LiquidationsTc1Repository) error {
			_, err := r.Periods(context.Background())
			return err
		},
	}

	for nombre, llamar := range casos {
		t.Run(nombre, func(t *testing.T) {
			repositorio, mock, _ := mocksTC1(t)
			mock.ExpectQuery(`.*`).WillReturnError(fallo)

			if err := llamar(repositorio); err == nil {
				t.Errorf("%s se tragó el error de la base", nombre)
			}
		})
	}
}
