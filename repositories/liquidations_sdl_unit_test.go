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

// Tests del repositorio de Tarifas SDL con sqlmock, SIN base de datos.
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_`. Ver docs/backend/migracion-a-go.md.
//
// El mismo motivo que en Cargos STR: file-compiler y calculator-prices son RDS
// externos que el CI de bia-bills no alcanza, así que sin estos tests el paquete
// reporta 0% de cobertura allá.

func mocksSDL(t *testing.T) (repositories.LiquidationsSdlRepository, sqlmock.Sqlmock, sqlmock.Sqlmock, func() []string) {
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

	return repositories.NewLiquidationsSdlRepository(db), mockFC, mockCP,
		func() []string { return ejecutado }
}

func filaInsumoSdl() models.LiquidationsSdlInput {
	area := "CENTRO"
	dt1, dt2, dt3 := 317.1201141, 198.4777546, 88.97771469

	return models.LiquidationsSdlInput{
		ID: "in-1", LoadID: "load-1", Period: "2026-01", OperatorCode: "CENS",
		DistributionArea: &area,
		DT1Add:           &dt1, DT2Add: &dt2, DT3Add: &dt3,
		DT1: 287.37107192, DT2: 190.82309415, DT3: 110.50282951,
		CDI: 53.77485343, CDN4: 34.85239617,
		PR1: 0.138011536, PR2: 0.037078059, PR3: 0.038380691,
	}
}

func filaTarifaSdl() models.LiquidationsSdlRate {
	return models.LiquidationsSdlRate{
		ID: "ra-1", LoadID: "load-1", Period: "2026-01", OperatorCode: "CENS",
		ActiveLevel1Operator: 276.6876, ActiveLevel1Shared: 249.8001, ActiveLevel1User: 222.9127,
		ActiveLevel2User: 162.2833, ActiveLevel3User: 52.7343,
		ReactiveLevel1Operator: 317.1201, ReactiveLevel1Shared: 290.2327,
		ReactiveLevel1User: 263.3453, ReactiveLevel2User: 198.4778, ReactiveLevel3User: 88.9777,
	}
}

// ── A qué base va cada escritura ─────────────────────────────────────────────

// Los componentes van a file-compiler y las tarifas a calculator-prices.
// Cruzarlas dejaría los datos en la base equivocada sin que nada falle.
func TestUnitSDL_InsertInputs_VaAFileCompiler(t *testing.T) {
	repo, mockFC, mockCP, _ := mocksSDL(t)

	mockFC.ExpectBegin()
	mockFC.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "public"."liquidations_sdl_inputs"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("in-1"))
	mockFC.ExpectCommit()

	if err := repo.InsertInputs(context.Background(),
		[]models.LiquidationsSdlInput{filaInsumoSdl()}); err != nil {
		t.Fatalf("InsertInputs: %v", err)
	}

	if err := mockFC.ExpectationsWereMet(); err != nil {
		t.Errorf("file-compiler: %v", err)
	}
	if err := mockCP.ExpectationsWereMet(); err != nil {
		t.Errorf("calculator-prices recibió tráfico que no le corresponde: %v", err)
	}
}

func TestUnitSDL_InsertRates_VaACalculatorPrices(t *testing.T) {
	repo, mockFC, mockCP, _ := mocksSDL(t)

	mockCP.ExpectBegin()
	mockCP.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "public"."liquidations_sdl_rates"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("ra-1"))
	mockCP.ExpectCommit()

	if err := repo.InsertRates(context.Background(),
		[]models.LiquidationsSdlRate{filaTarifaSdl()}); err != nil {
		t.Fatalf("InsertRates: %v", err)
	}

	if err := mockCP.ExpectationsWereMet(); err != nil {
		t.Errorf("calculator-prices: %v", err)
	}
	if err := mockFC.ExpectationsWereMet(); err != nil {
		t.Errorf("file-compiler recibió tráfico que no le corresponde: %v", err)
	}
}

func TestUnitSDL_InsertSinFilasNoConsulta(t *testing.T) {
	repo, mockFC, mockCP, _ := mocksSDL(t)

	if err := repo.InsertInputs(context.Background(), nil); err != nil {
		t.Errorf("InsertInputs con nil: %v", err)
	}
	if err := repo.InsertRates(context.Background(), nil); err != nil {
		t.Errorf("InsertRates con nil: %v", err)
	}

	if err := mockFC.ExpectationsWereMet(); err != nil {
		t.Errorf("file-compiler: %v", err)
	}
	if err := mockCP.ExpectationsWereMet(); err != nil {
		t.Errorf("calculator-prices: %v", err)
	}
}

func TestUnitSDL_DeleteInputsByLoad(t *testing.T) {
	repo, mockFC, _, _ := mocksSDL(t)

	mockFC.ExpectBegin()
	mockFC.ExpectExec(`DELETE FROM "public"\."liquidations_sdl_inputs" WHERE load_id = \$1`).
		WithArgs("load-1").
		WillReturnResult(sqlmock.NewResult(0, 21))
	mockFC.ExpectCommit()

	if err := repo.DeleteInputsByLoad(context.Background(), "load-1"); err != nil {
		t.Fatalf("DeleteInputsByLoad: %v", err)
	}
	if err := mockFC.ExpectationsWereMet(); err != nil {
		t.Errorf("file-compiler: %v", err)
	}
}

// ── La regla de lo vigente ───────────────────────────────────────────────────

// Si una lectura perdiera el DISTINCT ON, mezclaría cargues viejos con nuevos y
// devolvería tarifas de dos meses distintos como si fueran del mismo. Nada
// fallaría, y las tarifas alimentan la facturación.
func TestUnitSDL_TodaLecturaAplicaLaReglaVigente(t *testing.T) {
	casos := []struct {
		nombre       string
		enFileComp   bool
		leer         func(repositories.LiquidationsSdlRepository) error
		debeContener []string
	}{
		{
			nombre: "CurrentRates",
			leer: func(r repositories.LiquidationsSdlRepository) error {
				_, err := r.CurrentRates(context.Background(), repositories.SdlRateFilter{})
				return err
			},
			debeContener: []string{
				"DISTINCT ON (period, operator_code)",
				"public.liquidations_sdl_rates",
				"ORDER BY period, operator_code, created_at DESC, id DESC",
			},
		},
		{
			nombre:     "CurrentInputs",
			enFileComp: true,
			leer: func(r repositories.LiquidationsSdlRepository) error {
				_, err := r.CurrentInputs(context.Background(), nil)
				return err
			},
			debeContener: []string{
				"DISTINCT ON (period, operator_code)",
				"public.liquidations_sdl_inputs",
			},
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			repo, mockFC, mockCP, sqlEjecutado := mocksSDL(t)

			mock := mockCP
			if caso.enFileComp {
				mock = mockFC
			}
			mock.ExpectQuery(`.*`).WillReturnRows(sqlmock.NewRows([]string{"period"}))

			if err := caso.leer(repo); err != nil {
				t.Fatalf("%s: %v", caso.nombre, err)
			}

			query := sqlEjecutado()[0]
			for _, fragmento := range caso.debeContener {
				if !strings.Contains(query, fragmento) {
					t.Errorf("al SQL le falta %q:\n%s", fragmento, query)
				}
			}
		})
	}
}

// GORM no expande slices en `= ANY(?)`. Ya rompió cuatro consultas en Cargos STR
// y el síntoma fue traicionero.
func TestUnitSDL_FiltrosUsanInYNoAny(t *testing.T) {
	repo, _, mockCP, sqlEjecutado := mocksSDL(t)

	mockCP.ExpectQuery(`.*`).WillReturnRows(sqlmock.NewRows([]string{"period"}))

	_, err := repo.CurrentRates(context.Background(), repositories.SdlRateFilter{
		Periods:       []string{"2026-01", "2025-12"},
		OperatorCodes: []string{"CENS", "CHEC"},
	})
	if err != nil {
		t.Fatalf("CurrentRates: %v", err)
	}

	query := sqlEjecutado()[0]
	if strings.Contains(query, "ANY(") || strings.Contains(query, "ANY (") {
		t.Errorf("volvió el ANY(?) que GORM no expande:\n%s", query)
	}
	if n := strings.Count(query, "$"); n != 4 {
		t.Errorf("hay %d placeholders, se esperaban 4:\n%s", n, query)
	}
}

// ── Armado de resultados ─────────────────────────────────────────────────────

func TestUnitSDL_CurrentRates_MapeaLasDiezColumnas(t *testing.T) {
	repo, _, mockCP, _ := mocksSDL(t)

	columnas := []string{
		"load_id", "period", "operator_code", "operator_name", "agent_code", "market",
		"active_level_1_operator", "active_level_1_shared", "active_level_1_user",
		"active_level_2_user", "active_level_3_user",
		"reactive_level_1_operator", "reactive_level_1_shared", "reactive_level_1_user",
		"reactive_level_2_user", "reactive_level_3_user",
	}

	mockCP.ExpectQuery(`DISTINCT ON`).
		WillReturnRows(sqlmock.NewRows(columnas).AddRow(
			"load-1", "2026-01", "CENS",
			"CENTRALES ELECTRICAS DEL NORTE DE SANTANDER S.A. E.S.P.", "CNSD", "NORTE DE SANTANDER",
			276.6876, 249.8001, 222.9127, 162.2833, 52.7343,
			317.1201, 290.2327, 263.3453, 198.4778, 88.9777))

	res, err := repo.CurrentRates(context.Background(), repositories.SdlRateFilter{})
	if err != nil {
		t.Fatalf("CurrentRates: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("devolvió %d filas", len(res))
	}

	fila := res[0]
	// El nombre y el mercado: sin ellos las dos filas de Celsia son
	// indistinguibles en pantalla.
	if fila.OperatorName == "" || fila.Market != "NORTE DE SANTANDER" {
		t.Errorf("identidad mal mapeada: nombre=%q agente=%q mercado=%q",
			fila.OperatorName, fila.AgentCode, fila.Market)
	}

	// Las diez, una por una: un mapeo cruzado entre activa y reactiva daría
	// números que se ven bien.
	esperado := map[string][2]float64{
		"nivel 1 operador":   {fila.ActiveLevel1Operator, 276.6876},
		"nivel 1 compartido": {fila.ActiveLevel1Shared, 249.8001},
		"nivel 1 usuario":    {fila.ActiveLevel1User, 222.9127},
		"nivel 2 usuario":    {fila.ActiveLevel2User, 162.2833},
		"nivel 3 usuario":    {fila.ActiveLevel3User, 52.7343},
		"react 1 operador":   {fila.ReactiveLevel1Operator, 317.1201},
		"react 1 compartido": {fila.ReactiveLevel1Shared, 290.2327},
		"react 1 usuario":    {fila.ReactiveLevel1User, 263.3453},
		"react 2 usuario":    {fila.ReactiveLevel2User, 198.4778},
		"react 3 usuario":    {fila.ReactiveLevel3User, 88.9777},
	}
	for nombre, par := range esperado {
		if par[0] != par[1] {
			t.Errorf("%s = %v, se esperaba %v", nombre, par[0], par[1])
		}
	}
}

func TestUnitSDL_CurrentInputs_ConservaElAreaNulaYLosPunteros(t *testing.T) {
	repo, mockFC, _, _ := mocksSDL(t)

	columnas := []string{
		"id", "load_id", "period", "operator_code", "operator_name", "agent_code", "market",
		"distribution_area",
		"dt1_add", "dt2_add", "dt3_add", "dt1", "dt2", "dt3", "cdi", "cdn4",
		"pr1", "pr2", "pr3", "source_files", "created_by", "created_by_id", "created_at",
	}

	mockFC.ExpectQuery(`DISTINCT ON`).
		WillReturnRows(sqlmock.NewRows(columnas).
			// Un operador tipo ADD: área y los tres cargos.
			AddRow("i1", "l1", "2026-01", "CENS", "CENS S.A.", "CNSD", "NORTE DE SANTANDER", "CENTRO",
				317.12, 198.47, 88.97, 287.37, 190.82, 110.50, 53.77, 34.85,
				0.138, 0.037, 0.038, "a.xlsx", "Erika", "uid", time.Now()).
			// Un operador tipo USO: sin área ni cargos del ADD.
			AddRow("i2", "l1", "2026-01", "EPM", "EPM E.S.P.", "EPMD", "ANTIOQUIA", nil,
				nil, nil, nil, 299.04, 200.0, 100.0, 63.65, 34.85,
				0.128, 0.030, 0.030, "b.xlsx", "Erika", "uid", time.Now()))

	res, err := repo.CurrentInputs(context.Background(), nil)
	if err != nil {
		t.Fatalf("CurrentInputs: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("devolvió %d filas", len(res))
	}

	if res[0].DistributionArea == nil || *res[0].DistributionArea != "CENTRO" {
		t.Errorf("CENS debería traer área CENTRO: %+v", res[0].DistributionArea)
	}
	if res[0].DT1Add == nil {
		t.Error("CENS debería traer los cargos del ADD")
	}

	// El nil tiene que llegar como nil y no como cero: un cero acá diría que el
	// cargo del ADD es cero, que es un dato distinto de "no aplica".
	if res[1].DistributionArea != nil {
		t.Errorf("EPM no debería traer área: %v", *res[1].DistributionArea)
	}
	if res[1].DT1Add != nil {
		t.Errorf("EPM no debería traer cargos del ADD: %v", *res[1].DT1Add)
	}
}

func TestUnitSDL_LecturasPropaganElError(t *testing.T) {
	casos := map[string]struct {
		enFileComp bool
		leer       func(repositories.LiquidationsSdlRepository) error
	}{
		"CurrentRates": {false, func(r repositories.LiquidationsSdlRepository) error {
			_, err := r.CurrentRates(context.Background(), repositories.SdlRateFilter{})
			return err
		}},
		"CurrentInputs": {true, func(r repositories.LiquidationsSdlRepository) error {
			_, err := r.CurrentInputs(context.Background(), nil)
			return err
		}},
		"PeriodsWithRates": {false, func(r repositories.LiquidationsSdlRepository) error {
			_, err := r.PeriodsWithRates(context.Background())
			return err
		}},
		"Loads": {true, func(r repositories.LiquidationsSdlRepository) error {
			_, err := r.Loads(context.Background(), nil)
			return err
		}},
	}

	for nombre, caso := range casos {
		t.Run(nombre, func(t *testing.T) {
			repo, mockFC, mockCP, _ := mocksSDL(t)

			mock := mockCP
			if caso.enFileComp {
				mock = mockFC
			}
			mock.ExpectQuery(`.*`).WillReturnError(errors.New("base caída"))

			err := caso.leer(repo)
			if err == nil {
				t.Fatal("no propagó el error de la base")
			}
			if !strings.Contains(err.Error(), "base caída") {
				t.Errorf("perdió la causa: %v", err)
			}
		})
	}
}

// ── Historial ────────────────────────────────────────────────────────────────

func TestUnitSDL_Loads_LeeDeFileCompilerYAgrupa(t *testing.T) {
	repo, mockFC, mockCP, sqlEjecutado := mocksSDL(t)

	mockFC.ExpectQuery(`.*`).
		WillReturnRows(sqlmock.NewRows([]string{
			"load_id", "period", "created_at", "created_by", "created_by_id", "source_files", "operators",
		}).AddRow("carga-2", "2026-01", time.Now(), "Erika", "uid-1", "a.xlsx", 21))

	cargues, err := repo.Loads(context.Background(), nil)
	if err != nil {
		t.Fatalf("Loads: %v", err)
	}
	if len(cargues) != 1 || cargues[0].Operators != 21 {
		t.Fatalf("cargues = %+v", cargues)
	}

	query := sqlEjecutado()[0]
	for _, fragmento := range []string{
		"public.liquidations_sdl_inputs",
		"GROUP BY load_id, period",
		"ORDER BY MIN(created_at) DESC",
	} {
		if !strings.Contains(query, fragmento) {
			t.Errorf("al SQL le falta %q:\n%s", fragmento, query)
		}
	}

	if err := mockCP.ExpectationsWereMet(); err != nil {
		t.Errorf("calculator-prices recibió tráfico que no le corresponde: %v", err)
	}
}

func TestUnitSDL_PeriodsWithRates_MasRecientePrimero(t *testing.T) {
	repo, _, mockCP, _ := mocksSDL(t)

	mockCP.ExpectQuery(`ORDER BY period DESC`).
		WillReturnRows(sqlmock.NewRows([]string{"period"}).
			AddRow("2026-01").AddRow("2025-12"))

	periodos, err := repo.PeriodsWithRates(context.Background())
	if err != nil {
		t.Fatalf("PeriodsWithRates: %v", err)
	}
	if len(periodos) != 2 || periodos[0] != "2026-01" {
		t.Errorf("períodos = %v", periodos)
	}
}
