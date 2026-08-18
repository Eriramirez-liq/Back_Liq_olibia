package repositories_test

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"bia-bills/models"
	"bia-bills/providers/postgres"
	"bia-bills/repositories"

	"github.com/DATA-DOG/go-sqlmock"
	postgresGorm "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Tests del repositorio con sqlmock, SIN base de datos.
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_`. Ver docs/backend/migracion-a-go.md.
//
// ── Por qué existen, además de los de integración ────────────────────────────
// liquidations_str_integration_test.go prueba contra las bases reales de dev y es
// el que de verdad valida el comportamiento. Pero se salta solo cuando no hay
// credenciales, y en el CI de bia-bills NUNCA las va a haber: file-compiler y
// calculator-prices son RDS externos, no la base local con migraciones que su
// docker-compose levanta. Sin estos tests el paquete `repositories` reporta 0% de
// cobertura allá y el gate de SonarQube lo tumba.
//
// Así que acá se verifica lo que se puede verificar sin motor: el SQL que se arma
// y a qué base va cada operación. Lo que se ejecuta contra Postgres de verdad
// sigue siendo trabajo del test de integración.

// mocksSTR arma un repositorio con una conexión falsa por cada base, para poder
// afirmar cuál de las dos recibió cada consulta. Escribir el insumo en
// calculator-prices (o el resultado en file-compiler) sería un bug grave y
// silencioso: los datos quedan en la base equivocada sin que nada falle.
func mocksSTR(t *testing.T) (repositories.LiquidationsStrRepository, sqlmock.Sqlmock, sqlmock.Sqlmock) {
	repo, mockFC, mockCP, _ := mocksSTRConCaptura(t)
	return repo, mockFC, mockCP
}

// mocksSTRConCaptura hace lo mismo pero además devuelve el SQL que se ejecutó,
// para poder afirmar sobre la consulta armada y no solo sobre el resultado.
func mocksSTRConCaptura(t *testing.T) (repositories.LiquidationsStrRepository, sqlmock.Sqlmock, sqlmock.Sqlmock, func() []string) {
	t.Helper()

	ejecutado := []string{}

	// El matcher ve el SQL real antes de compararlo. Se anota de paso y se delega
	// en el matcher por regexp, el mismo que usa bia-bills.
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

	return repositories.NewLiquidationsStrRepository(db), mockFC, mockCP, func() []string { return ejecutado }
}

func filaInsumo() models.LiquidationsStrInput {
	ajuste := -4442.0

	return models.LiquidationsStrInput{
		ID:               "in-1",
		LoadID:           "load-1",
		Period:           "2026-05",
		OperatorCode:     "CHEC",
		InvoiceAmount:    70_812_140,
		Reinvoice1Amount: &ajuste,
	}
}

func filaCargo() models.LiquidationsStrCharge {
	return models.LiquidationsStrCharge{
		ID:            "ch-1",
		LoadID:        "load-1",
		Period:        "2026-05",
		OperatorCode:  "CHEC",
		OperatorName:  "CENTRAL HIDROELECTRICA DE CALDAS S.A. E.S.P.",
		AmountPayable: 70_807_698,
	}
}

// ── A qué base va cada escritura ─────────────────────────────────────────────

func TestUnit_InsertInputs_VaAFileCompiler(t *testing.T) {
	repo, mockFC, mockCP := mocksSTR(t)

	mockFC.ExpectBegin()
	mockFC.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "public"."liquidations_str_inputs"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("in-1"))
	mockFC.ExpectCommit()

	if err := repo.InsertInputs(context.Background(), []models.LiquidationsStrInput{filaInsumo()}); err != nil {
		t.Fatalf("InsertInputs: %v", err)
	}

	if err := mockFC.ExpectationsWereMet(); err != nil {
		t.Errorf("file-compiler: %v", err)
	}
	// calculator-prices no tenía expectativas: si hubiera recibido algo, el insert
	// habría fallado con "call to Query was not expected".
	if err := mockCP.ExpectationsWereMet(); err != nil {
		t.Errorf("calculator-prices recibió tráfico que no le corresponde: %v", err)
	}
}

func TestUnit_InsertCharges_VaACalculatorPrices(t *testing.T) {
	repo, mockFC, mockCP := mocksSTR(t)

	mockCP.ExpectBegin()
	mockCP.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "public"."liquidations_str_charges"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("ch-1"))
	mockCP.ExpectCommit()

	if err := repo.InsertCharges(context.Background(), []models.LiquidationsStrCharge{filaCargo()}); err != nil {
		t.Fatalf("InsertCharges: %v", err)
	}

	if err := mockCP.ExpectationsWereMet(); err != nil {
		t.Errorf("calculator-prices: %v", err)
	}
	if err := mockFC.ExpectationsWereMet(); err != nil {
		t.Errorf("file-compiler recibió tráfico que no le corresponde: %v", err)
	}
}

func TestUnit_DeleteInputsByLoad_BorraSoloEseCargue(t *testing.T) {
	repo, mockFC, _ := mocksSTR(t)

	mockFC.ExpectBegin()
	mockFC.ExpectExec(`DELETE FROM "public"\."liquidations_str_inputs" WHERE load_id = \$1`).
		WithArgs("load-1").
		WillReturnResult(sqlmock.NewResult(0, 23))
	mockFC.ExpectCommit()

	if err := repo.DeleteInputsByLoad(context.Background(), "load-1"); err != nil {
		t.Fatalf("DeleteInputsByLoad: %v", err)
	}

	if err := mockFC.ExpectationsWereMet(); err != nil {
		t.Errorf("file-compiler: %v", err)
	}
}

// Sin filas no hay que ir a la base. Importa porque el servicio llama a
// InsertCharges siempre, incluso cuando el lote quedó vacío.
func TestUnit_InsertSinFilasNoConsulta(t *testing.T) {
	repo, mockFC, mockCP := mocksSTR(t)

	if err := repo.InsertInputs(context.Background(), nil); err != nil {
		t.Errorf("InsertInputs con nil: %v", err)
	}
	if err := repo.InsertCharges(context.Background(), nil); err != nil {
		t.Errorf("InsertCharges con nil: %v", err)
	}

	if err := mockFC.ExpectationsWereMet(); err != nil {
		t.Errorf("file-compiler: %v", err)
	}
	if err := mockCP.ExpectationsWereMet(); err != nil {
		t.Errorf("calculator-prices: %v", err)
	}
}

// ── La regla de lo vigente ───────────────────────────────────────────────────

// El guard más importante del repositorio. Si una lectura perdiera el DISTINCT ON,
// sumaría las cargas viejas con las nuevas y DUPLICARÍA los montos. Con cifras de
// mil millones nadie lo nota mirando la pantalla, así que acá se afirma sobre el
// SQL armado: es lo único que detecta la regresión antes de producción.
func TestUnit_SQLDeLecturaContieneLaReglaVigente(t *testing.T) {
	casos := []struct {
		nombre       string
		leer         func(repositories.LiquidationsStrRepository) error
		debeContener []string
		noDebeTener  []string
	}{
		{
			nombre: "CurrentCharges sin filtros",
			leer: func(r repositories.LiquidationsStrRepository) error {
				_, err := r.CurrentCharges(context.Background(), repositories.StrChargeFilter{})
				return err
			},
			debeContener: []string{
				"DISTINCT ON (period, operator_code)",
				"public.liquidations_str_charges",
				"ORDER BY period, operator_code, created_at DESC, id DESC",
				"ORDER BY period, operator_name",
			},
			// Sin filtros no debe aparecer ningún IN: traería todo, que es lo pedido.
			noDebeTener: []string{"AND period IN", "AND operator_code IN"},
		},
		{
			nombre: "CurrentCharges solo períodos",
			leer: func(r repositories.LiquidationsStrRepository) error {
				_, err := r.CurrentCharges(context.Background(), repositories.StrChargeFilter{
					Periods: []string{"2026-05", "2026-04"},
				})
				return err
			},
			debeContener: []string{"DISTINCT ON (period, operator_code)", "AND period IN"},
			noDebeTener:  []string{"AND operator_code IN"},
		},
		{
			nombre: "CurrentCharges ambos filtros",
			leer: func(r repositories.LiquidationsStrRepository) error {
				_, err := r.CurrentCharges(context.Background(), repositories.StrChargeFilter{
					Periods:       []string{"2026-05"},
					OperatorCodes: []string{"CHEC", "EPM"},
				})
				return err
			},
			debeContener: []string{"AND period IN", "AND operator_code IN"},
		},
		{
			nombre: "TotalsByPeriod",
			leer: func(r repositories.LiquidationsStrRepository) error {
				_, err := r.TotalsByPeriod(context.Background(), []string{"2026-05"})
				return err
			},
			debeContener: []string{
				"DISTINCT ON (period, operator_code)",
				"SUM(amount_payable)",
				"GROUP BY period",
			},
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			repo, _, mockCP, sqlEjecutado := mocksSTRConCaptura(t)

			mockCP.ExpectQuery(`.*`).WillReturnRows(sqlmock.NewRows([]string{"period"}))

			if err := caso.leer(repo); err != nil {
				t.Fatalf("lectura: %v", err)
			}

			consultas := sqlEjecutado()
			if len(consultas) != 1 {
				t.Fatalf("se ejecutaron %d consultas, se esperaba 1: %v", len(consultas), consultas)
			}
			query := consultas[0]

			for _, fragmento := range caso.debeContener {
				if !strings.Contains(query, fragmento) {
					t.Errorf("al SQL le falta %q:\n%s", fragmento, query)
				}
			}
			for _, fragmento := range caso.noDebeTener {
				if strings.Contains(query, fragmento) {
					t.Errorf("el SQL trae %q y no debería:\n%s", fragmento, query)
				}
			}
		})
	}
}

// GORM no expande slices en `= ANY(?)`: produce ANY('CMMD','CSID',...) y Postgres
// devuelve "syntax error at or near ,". Ya pasó, en cuatro consultas a la vez, y el
// síntoma fue traicionero —el preview respondía 200 con los montos correctos y los
// nombres vacíos—. Este test fija el `IN (?)` para que no vuelva.
func TestUnit_FiltrosUsanInYNoAny(t *testing.T) {
	repo, _, mockCP, sqlEjecutado := mocksSTRConCaptura(t)

	mockCP.ExpectQuery(`.*`).WillReturnRows(sqlmock.NewRows([]string{"period"}))

	_, err := repo.CurrentCharges(context.Background(), repositories.StrChargeFilter{
		Periods:       []string{"2026-05", "2026-04"},
		OperatorCodes: []string{"CHEC", "EPM"},
	})
	if err != nil {
		t.Fatalf("CurrentCharges: %v", err)
	}

	query := sqlEjecutado()[0]
	if strings.Contains(query, "ANY(") || strings.Contains(query, "ANY (") {
		t.Errorf("volvió el ANY(?) que GORM no expande:\n%s", query)
	}

	// Los dos slices tienen que quedar expandidos a un placeholder por valor: 4 en
	// total. Si GORM no expandiera, habría 2.
	if placeholders := strings.Count(query, "$"); placeholders != 4 {
		t.Errorf("hay %d placeholders, se esperaban 4 (2 períodos + 2 operadores):\n%s", placeholders, query)
	}
}

// ── Armado de resultados ─────────────────────────────────────────────────────

func TestUnit_CurrentCharges_MapeaFilas(t *testing.T) {
	repo, _, mockCP := mocksSTR(t)

	mockCP.ExpectQuery(`DISTINCT ON`).
		WillReturnRows(sqlmock.NewRows([]string{"load_id", "period", "operator_code", "operator_name", "amount_payable"}).
			AddRow("load-1", "2026-05", "CHEC", "CENTRAL HIDROELECTRICA DE CALDAS S.A. E.S.P.", 70_807_698.0).
			AddRow("load-1", "2026-05", "AIRE", "AIR-E S.A.S. E.S.P.", 142_265_108.0))

	res, err := repo.CurrentCharges(context.Background(), repositories.StrChargeFilter{})
	if err != nil {
		t.Fatalf("CurrentCharges: %v", err)
	}

	if len(res) != 2 {
		t.Fatalf("devolvió %d filas, se esperaban 2: %+v", len(res), res)
	}
	if res[0].OperatorCode != "CHEC" || res[0].AmountPayable != 70_807_698 {
		t.Errorf("primera fila mal mapeada: %+v", res[0])
	}
	if res[1].OperatorName != "AIR-E S.A.S. E.S.P." {
		t.Errorf("nombre mal mapeado: %+v", res[1])
	}
	if err := mockCP.ExpectationsWereMet(); err != nil {
		t.Errorf("calculator-prices: %v", err)
	}
}

func TestUnit_CurrentCharges_PropagaElError(t *testing.T) {
	repo, _, mockCP := mocksSTR(t)

	mockCP.ExpectQuery(`DISTINCT ON`).WillReturnError(errors.New("conexión caída"))

	_, err := repo.CurrentCharges(context.Background(), repositories.StrChargeFilter{})
	if err == nil {
		t.Fatal("no propagó el error de la base")
	}
	if !strings.Contains(err.Error(), "conexión caída") {
		t.Errorf("perdió la causa original: %v", err)
	}
}

// Un período sin datos NO aparece en el mapa. Es la forma de distinguir "cargado
// con total cero" de "sin cargar", que es lo que el front pinta distinto.
func TestUnit_TotalsByPeriod_PeriodoSinDatosNoApareceEnElMapa(t *testing.T) {
	repo, _, mockCP := mocksSTR(t)

	mockCP.ExpectQuery(`GROUP BY period`).
		WillReturnRows(sqlmock.NewRows([]string{"period", "total"}).
			AddRow("2026-05", 1_460_833_304.0))

	totales, err := repo.TotalsByPeriod(context.Background(), []string{"2026-05", "2026-04"})
	if err != nil {
		t.Fatalf("TotalsByPeriod: %v", err)
	}

	if totales["2026-05"] != 1_460_833_304 {
		t.Errorf("total de 2026-05 = %v", totales["2026-05"])
	}
	if _, hay := totales["2026-04"]; hay {
		t.Error("2026-04 no tenía datos y quedó en el mapa: se confunde con un total de cero")
	}
	if err := mockCP.ExpectationsWereMet(); err != nil {
		t.Errorf("calculator-prices: %v", err)
	}
}

func TestUnit_TotalsByPeriod_SinPeriodosNoConsulta(t *testing.T) {
	repo, mockFC, mockCP := mocksSTR(t)

	totales, err := repo.TotalsByPeriod(context.Background(), nil)
	if err != nil {
		t.Fatalf("TotalsByPeriod: %v", err)
	}
	if len(totales) != 0 {
		t.Errorf("devolvió %d totales sin pedir períodos", len(totales))
	}

	if err := mockCP.ExpectationsWereMet(); err != nil {
		t.Errorf("calculator-prices: %v", err)
	}
	if err := mockFC.ExpectationsWereMet(); err != nil {
		t.Errorf("file-compiler: %v", err)
	}
}

func TestUnit_TotalsByPeriod_PropagaElError(t *testing.T) {
	repo, _, mockCP := mocksSTR(t)

	mockCP.ExpectQuery(`GROUP BY period`).WillReturnError(errors.New("timeout"))

	if _, err := repo.TotalsByPeriod(context.Background(), []string{"2026-05"}); err == nil {
		t.Fatal("no propagó el error de la base")
	}
}

func TestUnit_PeriodsWithCharges_MasRecientePrimero(t *testing.T) {
	repo, _, mockCP := mocksSTR(t)

	mockCP.ExpectQuery(`ORDER BY period DESC`).
		WillReturnRows(sqlmock.NewRows([]string{"period"}).
			AddRow("2026-05").
			AddRow("2026-04"))

	periodos, err := repo.PeriodsWithCharges(context.Background())
	if err != nil {
		t.Fatalf("PeriodsWithCharges: %v", err)
	}

	if len(periodos) != 2 || periodos[0] != "2026-05" || periodos[1] != "2026-04" {
		t.Errorf("períodos = %v, se esperaba [2026-05 2026-04]", periodos)
	}
	if err := mockCP.ExpectationsWereMet(); err != nil {
		t.Errorf("calculator-prices: %v", err)
	}
}

// ── Historial de cargues ─────────────────────────────────────────────────────

// El historial NO sale de calculator-prices sino de file-compiler: ahí está el
// insumo con sus metadatos, y ahí es donde un cargue existe.
func TestUnit_Loads_LeeDeFileCompiler(t *testing.T) {
	repo, mockFC, mockCP, sqlEjecutado := mocksSTRConCaptura(t)

	mockFC.ExpectQuery(`.*`).
		WillReturnRows(sqlmock.NewRows([]string{"load_id", "period", "created_at", "created_by", "created_by_id", "source_files", "operators"}).
			AddRow("carga-2", "2026-07", time.Now(), "Erika", "uid-1", "a.xlsx, b.xlsx", 23).
			AddRow("carga-1", "2026-06", time.Now().Add(-time.Hour), "", "", "", 23))

	cargues, err := repo.Loads(context.Background(), nil)
	if err != nil {
		t.Fatalf("Loads: %v", err)
	}

	if len(cargues) != 2 {
		t.Fatalf("devolvió %d cargues: %+v", len(cargues), cargues)
	}
	if cargues[0].CreatedBy != "Erika" || cargues[0].Operators != 23 {
		t.Errorf("primer cargue mal mapeado: %+v", cargues[0])
	}
	// Un cargue viejo, anterior a los metadatos, viaja con los campos vacíos.
	if cargues[1].CreatedBy != "" || cargues[1].Operators != 23 {
		t.Errorf("cargue sin metadatos mal mapeado: %+v", cargues[1])
	}

	query := sqlEjecutado()[0]
	if !strings.Contains(query, "liquidations_str_inputs") {
		t.Errorf("no lee de la tabla de insumos:\n%s", query)
	}
	if !strings.Contains(query, "GROUP BY load_id, period") {
		t.Errorf("no agrupa por cargue:\n%s", query)
	}
	// Más reciente primero: es lo que espera el historial.
	if !strings.Contains(query, "ORDER BY MIN(created_at) DESC") {
		t.Errorf("no ordena por fecha descendente:\n%s", query)
	}

	if err := mockCP.ExpectationsWereMet(); err != nil {
		t.Errorf("calculator-prices recibió tráfico que no le corresponde: %v", err)
	}
}

func TestUnit_Loads_SinPeriodosNoFiltra(t *testing.T) {
	repo, mockFC, _, sqlEjecutado := mocksSTRConCaptura(t)

	mockFC.ExpectQuery(`.*`).WillReturnRows(sqlmock.NewRows([]string{"load_id"}))

	if _, err := repo.Loads(context.Background(), nil); err != nil {
		t.Fatalf("Loads: %v", err)
	}

	if query := sqlEjecutado()[0]; strings.Contains(query, "WHERE") {
		t.Errorf("filtró sin que le pidieran períodos:\n%s", query)
	}
}

func TestUnit_Loads_ConPeriodosUsaIn(t *testing.T) {
	repo, mockFC, _, sqlEjecutado := mocksSTRConCaptura(t)

	mockFC.ExpectQuery(`.*`).WillReturnRows(sqlmock.NewRows([]string{"load_id"}))

	if _, err := repo.Loads(context.Background(), []string{"2026-07", "2026-06"}); err != nil {
		t.Fatalf("Loads: %v", err)
	}

	query := sqlEjecutado()[0]
	if !strings.Contains(query, "WHERE period IN (") {
		t.Errorf("no filtró por período:\n%s", query)
	}
	if strings.Contains(query, "ANY(") {
		t.Errorf("volvió el ANY(?) que GORM no expande:\n%s", query)
	}
	if n := strings.Count(query, "$"); n != 2 {
		t.Errorf("hay %d placeholders, se esperaban 2:\n%s", n, query)
	}
}
