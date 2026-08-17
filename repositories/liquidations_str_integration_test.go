package repositories_test

import (
	"context"
	"os"
	"testing"

	"bia-bills/models"
	"bia-bills/providers/postgres"
	"bia-bills/repositories"

	"github.com/google/uuid"
)

// Tests de repositorio contra las bases REALES.
//
// Van contra la base y no con sqlmock a propósito: sqlmock verifica que el SQL
// sea el que escribiste, no que Postgres lo acepte. El bug de `= ANY(?)` —GORM
// expande los slices como lista separada por comas y generaba SQL inválido— habría
// pasado un test con sqlmock sin problema.
//
// Se saltean sin credenciales, así que en CI no corren. La cobertura para su
// SonarQube va aparte, con sqlmock.
//
// Todo usa un período de prueba propio y limpia al terminar.

const periodoPrueba = "1900-01"

func repoDePrueba(t *testing.T) (repositories.LiquidationsStrRepository, func()) {
	t.Helper()

	if os.Getenv("liq_db_host") == "" {
		t.Skip("sin credenciales de las bases de BIA (liq_db_host vacío)")
	}

	db := postgres.NewLiquidationsDB()
	repo := repositories.NewLiquidationsStrRepository(db)

	limpiar := func() {
		ctx := context.Background()
		db.Connection(postgres.LiqDBFileCompiler).WithContext(ctx).
			Exec("DELETE FROM public.liquidations_str_inputs WHERE period = ?", periodoPrueba)
		db.Connection(postgres.LiqDBCalculatorPrices).WithContext(ctx).
			Exec("DELETE FROM public.liquidations_str_charges WHERE period = ?", periodoPrueba)
	}

	limpiar() // por si un test anterior murió a mitad
	return repo, limpiar
}

func insumo(loadID, operador string, factura float64, ajuste *float64) models.LiquidationsStrInput {
	return models.LiquidationsStrInput{
		LoadID:           loadID,
		Period:           periodoPrueba,
		OperatorCode:     operador,
		InvoiceAmount:    factura,
		Reinvoice1Amount: ajuste,
	}
}

func cargo(loadID, operador, nombre string, aPagar float64) models.LiquidationsStrCharge {
	return models.LiquidationsStrCharge{
		LoadID:        loadID,
		Period:        periodoPrueba,
		OperatorCode:  operador,
		OperatorName:  nombre,
		AmountPayable: aPagar,
	}
}

func TestRepositorio_EscribeYLee(t *testing.T) {
	repo, limpiar := repoDePrueba(t)
	defer limpiar()

	ctx := context.Background()
	load := uuid.NewString()
	ajuste := -100.0

	if err := repo.InsertInputs(ctx, []models.LiquidationsStrInput{
		insumo(load, "CHEC", 1000, &ajuste),
		insumo(load, "EPM", 2000, nil), // sin ajuste: la columna queda NULL
	}); err != nil {
		t.Fatalf("InsertInputs: %v", err)
	}

	if err := repo.InsertCharges(ctx, []models.LiquidationsStrCharge{
		cargo(load, "CHEC", "Central Hidroeléctrica de Caldas", 900),
		cargo(load, "EPM", "Empresas Públicas de Medellín", 2000),
	}); err != nil {
		t.Fatalf("InsertCharges: %v", err)
	}

	cargos, err := repo.CurrentCharges(ctx, repositories.StrChargeFilter{Periods: []string{periodoPrueba}})
	if err != nil {
		t.Fatalf("CurrentCharges: %v", err)
	}
	if len(cargos) != 2 {
		t.Fatalf("devolvió %d cargos, se esperaban 2", len(cargos))
	}

	// Vienen ordenados por nombre, no por código.
	if cargos[0].OperatorName > cargos[1].OperatorName {
		t.Error("no vienen ordenados por operator_name")
	}
}

// El caso que más me preocupa del modelo append-only: si una carga posterior trae
// MENOS operadores, los que faltan tienen que conservar el valor de la carga
// anterior. Es el comportamiento buscado, y si estuviera mal nadie lo notaría
// hasta que un operador desapareciera de la matriz.
func TestRepositorio_CargaConMenosOperadores(t *testing.T) {
	repo, limpiar := repoDePrueba(t)
	defer limpiar()

	ctx := context.Background()
	primera, segunda := uuid.NewString(), uuid.NewString()

	if err := repo.InsertCharges(ctx, []models.LiquidationsStrCharge{
		cargo(primera, "CHEC", "Chec", 1000),
		cargo(primera, "EPM", "EPM", 2000),
	}); err != nil {
		t.Fatalf("primera carga: %v", err)
	}

	// La segunda solo trae CHEC, con otro valor.
	if err := repo.InsertCharges(ctx, []models.LiquidationsStrCharge{
		cargo(segunda, "CHEC", "Chec", 9999),
	}); err != nil {
		t.Fatalf("segunda carga: %v", err)
	}

	cargos, err := repo.CurrentCharges(ctx, repositories.StrChargeFilter{Periods: []string{periodoPrueba}})
	if err != nil {
		t.Fatalf("CurrentCharges: %v", err)
	}

	porOperador := map[string]repositories.StrCharge{}
	for _, c := range cargos {
		porOperador[c.OperatorCode] = c
	}

	if len(cargos) != 2 {
		t.Fatalf("devolvió %d cargos, se esperaban 2 (CHEC nuevo + EPM heredado)", len(cargos))
	}
	if got := porOperador["CHEC"].AmountPayable; got != 9999 {
		t.Errorf("CHEC = %.0f, se esperaba 9999 (el de la carga nueva)", got)
	}
	if got := porOperador["CHEC"].LoadID; got != segunda {
		t.Errorf("CHEC quedó con el load_id viejo")
	}
	if got := porOperador["EPM"].AmountPayable; got != 2000 {
		t.Errorf("EPM = %.0f, se esperaba 2000 (conserva el de la carga anterior)", got)
	}
	if got := porOperador["EPM"].LoadID; got != primera {
		t.Errorf("EPM debería seguir con el load_id de la primera carga")
	}
}

func TestRepositorio_Filtros(t *testing.T) {
	repo, limpiar := repoDePrueba(t)
	defer limpiar()

	ctx := context.Background()
	load := uuid.NewString()

	if err := repo.InsertCharges(ctx, []models.LiquidationsStrCharge{
		cargo(load, "CHEC", "Chec", 1000),
		cargo(load, "EPM", "EPM", 2000),
		cargo(load, "AIRE", "Aire", 3000),
	}); err != nil {
		t.Fatalf("carga: %v", err)
	}

	casos := []struct {
		nombre   string
		filtro   repositories.StrChargeFilter
		esperado int
	}{
		{"por período", repositories.StrChargeFilter{Periods: []string{periodoPrueba}}, 3},
		{"por un operador", repositories.StrChargeFilter{
			Periods: []string{periodoPrueba}, OperatorCodes: []string{"CHEC"}}, 1},
		{"por varios operadores", repositories.StrChargeFilter{
			Periods: []string{periodoPrueba}, OperatorCodes: []string{"CHEC", "EPM"}}, 2},
		{"período sin datos", repositories.StrChargeFilter{Periods: []string{"1899-01"}}, 0},
		{"operador que no existe", repositories.StrChargeFilter{
			Periods: []string{periodoPrueba}, OperatorCodes: []string{"NOEXISTE"}}, 0},
		{"varios períodos", repositories.StrChargeFilter{
			Periods: []string{periodoPrueba, "1899-01"}}, 3},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			cargos, err := repo.CurrentCharges(ctx, caso.filtro)
			if err != nil {
				t.Fatalf("CurrentCharges: %v", err)
			}
			if len(cargos) != caso.esperado {
				t.Errorf("devolvió %d, se esperaban %d", len(cargos), caso.esperado)
			}
		})
	}
}

func TestRepositorio_TotalesYPeriodos(t *testing.T) {
	repo, limpiar := repoDePrueba(t)
	defer limpiar()

	ctx := context.Background()
	primera, segunda := uuid.NewString(), uuid.NewString()

	if err := repo.InsertCharges(ctx, []models.LiquidationsStrCharge{
		cargo(primera, "CHEC", "Chec", 1000),
		cargo(primera, "EPM", "EPM", 2000),
	}); err != nil {
		t.Fatalf("primera carga: %v", err)
	}
	// Recarga del mismo período con otros valores.
	if err := repo.InsertCharges(ctx, []models.LiquidationsStrCharge{
		cargo(segunda, "CHEC", "Chec", 500),
		cargo(segunda, "EPM", "EPM", 500),
	}); err != nil {
		t.Fatalf("segunda carga: %v", err)
	}

	totales, err := repo.TotalsByPeriod(ctx, []string{periodoPrueba, "1899-01"})
	if err != nil {
		t.Fatalf("TotalsByPeriod: %v", err)
	}

	// 1000 si suma solo lo vigente; 4000 si sumara las dos cargas.
	if got := totales[periodoPrueba]; got != 1000 {
		t.Errorf("total = %.0f, se esperaba 1000 (solo la carga vigente)", got)
	}
	// Los períodos sin datos NO aparecen: es lo que permite distinguir "cero" de
	// "no cargado".
	if _, existe := totales["1899-01"]; existe {
		t.Error("un período sin datos no debería aparecer en el mapa")
	}

	// Mapa vacío sin períodos, sin tocar la base.
	vacio, err := repo.TotalsByPeriod(ctx, nil)
	if err != nil || len(vacio) != 0 {
		t.Errorf("TotalsByPeriod(nil) = %v, %v", vacio, err)
	}

	periodos, err := repo.PeriodsWithCharges(ctx)
	if err != nil {
		t.Fatalf("PeriodsWithCharges: %v", err)
	}
	var encontrado bool
	for _, p := range periodos {
		if p == periodoPrueba {
			encontrado = true
		}
	}
	if !encontrado {
		t.Errorf("PeriodsWithCharges no incluye %s: %v", periodoPrueba, periodos)
	}
}

func TestRepositorio_BorraSoloLaCargaIndicada(t *testing.T) {
	repo, limpiar := repoDePrueba(t)
	defer limpiar()

	ctx := context.Background()
	aBorrar, aConservar := uuid.NewString(), uuid.NewString()

	if err := repo.InsertInputs(ctx, []models.LiquidationsStrInput{
		insumo(aBorrar, "CHEC", 1000, nil),
		insumo(aConservar, "EPM", 2000, nil),
	}); err != nil {
		t.Fatalf("InsertInputs: %v", err)
	}

	if err := repo.DeleteInputsByLoad(ctx, aBorrar); err != nil {
		t.Fatalf("DeleteInputsByLoad: %v", err)
	}

	// Se verifica por el lado del resultado: si el borrado se hubiera llevado la
	// otra carga, el rollback del Confirm destruiría datos buenos.
	db := postgres.NewLiquidationsDB()
	var quedan int64
	if err := db.Connection(postgres.LiqDBFileCompiler).WithContext(ctx).
		Raw(`SELECT count(*) FROM public.liquidations_str_inputs WHERE period = ? AND load_id = ?`,
			periodoPrueba, aConservar).Scan(&quedan).Error; err != nil {
		t.Fatalf("consulta de verificación: %v", err)
	}
	if quedan != 1 {
		t.Errorf("la otra carga quedó con %d filas, se esperaba 1", quedan)
	}

	var borradas int64
	if err := db.Connection(postgres.LiqDBFileCompiler).WithContext(ctx).
		Raw(`SELECT count(*) FROM public.liquidations_str_inputs WHERE load_id = ?`, aBorrar).
		Scan(&borradas).Error; err != nil {
		t.Fatalf("consulta de verificación: %v", err)
	}
	if borradas != 0 {
		t.Errorf("la carga borrada dejó %d filas", borradas)
	}
}

func TestRepositorio_InsertVacioNoFalla(t *testing.T) {
	repo, limpiar := repoDePrueba(t)
	defer limpiar()

	ctx := context.Background()
	if err := repo.InsertInputs(ctx, nil); err != nil {
		t.Errorf("InsertInputs(nil): %v", err)
	}
	if err := repo.InsertCharges(ctx, nil); err != nil {
		t.Errorf("InsertCharges(nil): %v", err)
	}
}

func TestRepositorio_NombresDeAgentes(t *testing.T) {
	if os.Getenv("liq_db_host") == "" {
		t.Skip("sin credenciales de las bases de BIA")
	}

	ctx := context.Background()
	repo := repositories.NewLiquidationsAgentsRepository(postgres.NewLiquidationsDB())

	// El mismo mapa que usa el parser, reducido a dos operadores para el test.
	nombres, err := repo.NamesByOperator(ctx, map[string]string{
		"CHCD": "CHEC",
		"CSID": "AIRE",
		"CSSD": "AIRE",
	})
	if err != nil {
		t.Fatalf("NamesByOperator: %v", err)
	}

	if got := nombres["CHEC"]; got != "CENTRAL HIDROELECTRICA DE CALDAS S.A. E.S.P." {
		t.Errorf("CHEC = %q", got)
	}
	// AIRE llega por dos códigos; el nombre sale del que es OPERADOR DE RED (CSSD),
	// no del que figura como intervenido (CSID).
	if got := nombres["AIRE"]; got != "AIR- E S.A.S. E.S.P." {
		t.Errorf("AIRE = %q, se esperaba el registro vigente y no el intervenido", got)
	}

	// Un código que no existe simplemente no aparece.
	nombres, err = repo.NamesByOperator(ctx, map[string]string{"XXXX": "INVENTADO"})
	if err != nil {
		t.Fatalf("NamesByOperator con código inexistente: %v", err)
	}
	if len(nombres) != 0 {
		t.Errorf("devolvió %v para un código que no existe", nombres)
	}
}
