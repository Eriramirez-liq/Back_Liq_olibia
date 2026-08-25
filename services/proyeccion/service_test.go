package proyeccion_test

import (
	"context"
	"errors"
	"testing"

	"bia-bills/models"
	"bia-bills/repositories"
	"bia-bills/services/proyeccion"
)

// Tests del servicio de Proyección Cargos OR.
//
// ⚠️ TRASVASE: archivo del paquete proyeccion. Ver docs/backend/migracion-a-go.md.
//
// Lo que hay que fijar acá es el PROMEDIO por nivel de tensión: es el número que
// se muestra en pantalla y el que alimentaría la valorización cuando Facturación
// se migre. Si cambia, la proyección cambia sin que nadie lo pida.

// ── Dobles de los dos repositorios ──────────────────────────────────────────

type sdlFake struct {
	tarifas  []repositories.SdlRate
	periodos []string
	err      error
}

func (f sdlFake) CurrentRates(context.Context, repositories.SdlRateFilter) ([]repositories.SdlRate, error) {
	return f.tarifas, f.err
}
func (f sdlFake) PeriodsWithRates(context.Context) ([]string, error) { return f.periodos, f.err }

func (sdlFake) InsertInputs(context.Context, []models.LiquidationsSdlInput) error { return nil }
func (sdlFake) InsertRates(context.Context, []models.LiquidationsSdlRate) error   { return nil }
func (sdlFake) DeleteInputsByLoad(context.Context, string) error                  { return nil }
func (sdlFake) CurrentInputs(context.Context, []string) ([]models.LiquidationsSdlInput, error) {
	return nil, nil
}
func (sdlFake) Loads(context.Context, []string) ([]repositories.SdlLoad, error) { return nil, nil }

type strFake struct {
	totales  map[string]float64
	periodos []string
	err      error
}

func (f strFake) TotalsByPeriod(context.Context, []string) (map[string]float64, error) {
	return f.totales, f.err
}
func (f strFake) PeriodsWithCharges(context.Context) ([]string, error) { return f.periodos, f.err }

func (strFake) InsertInputs(context.Context, []models.LiquidationsStrInput) error   { return nil }
func (strFake) InsertCharges(context.Context, []models.LiquidationsStrCharge) error { return nil }
func (strFake) DeleteInputsByLoad(context.Context, string) error                    { return nil }
func (strFake) CurrentCharges(context.Context, repositories.StrChargeFilter) ([]repositories.StrCharge, error) {
	return nil, nil
}
func (strFake) Loads(context.Context, []string) ([]repositories.StrLoad, error) { return nil, nil }

// tarifa arma una fila con valores distintos por columna, para que un promedio
// mal armado no pase inadvertido.
func tarifa(periodo, operador string, base float64) repositories.SdlRate {
	return repositories.SdlRate{
		Period: periodo, OperatorCode: operador,
		ActiveLevel1Operator: base, ActiveLevel1Shared: base + 1, ActiveLevel1User: base + 2,
		ActiveLevel2User: base + 10, ActiveLevel3User: base + 20,
		ReactiveLevel1Operator: base + 100, ReactiveLevel1Shared: base + 101,
		ReactiveLevel1User: base + 102,
		ReactiveLevel2User: base + 110, ReactiveLevel3User: base + 120,
	}
}

// El nivel 1 promedia sus TRES propiedades; los niveles 2 y 3 la única que
// tienen. Es lo que hacía la versión TypeScript sobre la tabla en formato largo,
// donde cada propiedad era una fila.
func TestPrices_ElNivel1PromediaSusTresPropiedades(t *testing.T) {
	servicio := proyeccion.NewProyeccionService(
		sdlFake{tarifas: []repositories.SdlRate{tarifa("2026-01", "CENS", 100)}},
		strFake{},
	)

	meses, err := servicio.Prices(context.Background(), []string{"2026-01"}, 0)
	if err != nil {
		t.Fatalf("Prices: %v", err)
	}
	if len(meses) != 1 {
		t.Fatalf("devolvió %d meses", len(meses))
	}

	// (100 + 101 + 102) / 3 = 101
	if *meses[0].ActivaNT1 != 101 {
		t.Errorf("activa NT1 = %v, se esperaba 101 (promedio de las tres propiedades)", *meses[0].ActivaNT1)
	}
	// Los niveles 2 y 3 tienen una sola: el promedio es el valor.
	if *meses[0].ActivaNT2 != 110 || *meses[0].ActivaNT3 != 120 {
		t.Errorf("activa NT2/NT3 = %v/%v, se esperaba 110/120", *meses[0].ActivaNT2, *meses[0].ActivaNT3)
	}
	// (200 + 201 + 202) / 3 = 201
	if *meses[0].ReactivaNT1 != 201 {
		t.Errorf("reactiva NT1 = %v, se esperaba 201", *meses[0].ReactivaNT1)
	}
}

// El promedio cruza TODOS los operadores del mes, no uno.
func TestPrices_PromediaEntreOperadores(t *testing.T) {
	servicio := proyeccion.NewProyeccionService(
		sdlFake{tarifas: []repositories.SdlRate{
			tarifa("2026-01", "CENS", 100), // NT2 = 110
			tarifa("2026-01", "CHEC", 200), // NT2 = 210
		}},
		strFake{},
	)

	meses, _ := servicio.Prices(context.Background(), []string{"2026-01"}, 0)

	if *meses[0].ActivaNT2 != 160 { // (110 + 210) / 2
		t.Errorf("activa NT2 = %v, se esperaba 160", *meses[0].ActivaNT2)
	}
	if meses[0].Operators != 2 {
		t.Errorf("operadores = %d, se esperaban 2", meses[0].Operators)
	}
}

// nil y no cero. Cero se lee como "la tarifa es cero" o "no se pagó nada", que es
// una afirmación distinta de "todavía no hay dato".
func TestPrices_SinDatosDevuelveNilYNoCero(t *testing.T) {
	servicio := proyeccion.NewProyeccionService(
		sdlFake{tarifas: []repositories.SdlRate{tarifa("2026-01", "CENS", 100)}},
		strFake{totales: map[string]float64{}},
	)

	meses, _ := servicio.Prices(context.Background(), []string{"2026-01", "2026-02"}, 0)
	if len(meses) != 2 {
		t.Fatalf("devolvió %d meses", len(meses))
	}

	// Del más viejo al más nuevo: la matriz se lee de izquierda a derecha.
	if meses[0].Period != "2026-01" {
		t.Errorf("el primero es %q, se esperaba 2026-01", meses[0].Period)
	}
	// 2026-01 tiene tarifas pero no cargos STR: una cosa sí y la otra no.
	if meses[0].ActivaNT1 == nil {
		t.Error("2026-01 tiene tarifas y llegó sin precios")
	}
	if meses[0].StrTotalCop != nil {
		t.Error("2026-01 no tiene cargos STR y llegó con total")
	}
	// 2026-02 no tiene ni tarifas ni cargos.
	if meses[1].ActivaNT1 != nil || meses[1].StrTotalCop != nil {
		t.Errorf("un mes sin datos no debería traer valores: %+v", meses[1])
	}
}

func TestPrices_TraeElValorStrDelMes(t *testing.T) {
	servicio := proyeccion.NewProyeccionService(
		sdlFake{},
		strFake{totales: map[string]float64{"2026-07": 1403159917}},
	)

	meses, _ := servicio.Prices(context.Background(), []string{"2026-07"}, 0)

	if meses[0].StrTotalCop == nil || *meses[0].StrTotalCop != 1403159917 {
		t.Errorf("no trajo el valor STR: %+v", meses[0])
	}
}

// Sin períodos pedidos, los meses son la UNIÓN de los que tienen tarifas y los que
// tienen cargos. Antes la lista salía de Facturación y sin ella la matriz quedaba
// vacía aunque hubiera datos cargados: es el bug que este servicio viene a cerrar.
func TestPrices_SinPeriodosUneLosDosOrigenes(t *testing.T) {
	servicio := proyeccion.NewProyeccionService(
		sdlFake{periodos: []string{"2026-02", "2026-01"}},
		strFake{periodos: []string{"2026-03", "2026-01"}},
	)

	meses, err := servicio.Prices(context.Background(), nil, 0)
	if err != nil {
		t.Fatalf("Prices: %v", err)
	}

	// Tres distintos: 2026-01 está en los dos y no se duplica.
	if len(meses) != 3 {
		t.Fatalf("devolvió %d meses, se esperaban 3: %+v", len(meses), meses)
	}
	esperado := []string{"2026-01", "2026-02", "2026-03"}
	for i, p := range esperado {
		if meses[i].Period != p {
			t.Errorf("posición %d = %q, se esperaba %q", i, meses[i].Period, p)
		}
	}
}

func TestPrices_SinNadaCargadoDevuelveListaVacia(t *testing.T) {
	meses, err := proyeccion.NewProyeccionService(sdlFake{}, strFake{}).
		Prices(context.Background(), nil, 0)
	if err != nil {
		t.Fatalf("Prices: %v", err)
	}
	if len(meses) != 0 {
		t.Errorf("devolvió %d meses sin datos", len(meses))
	}
}

// Un error de la base no se puede tragar: la pantalla mostraría una matriz vacía
// como si no hubiera datos.
func TestPrices_PropagaElError(t *testing.T) {
	fallo := errors.New("se cayó la conexión")

	casos := map[string]proyeccion.ProyeccionService{
		"tarifas": proyeccion.NewProyeccionService(sdlFake{err: fallo}, strFake{}),
		"cargos":  proyeccion.NewProyeccionService(sdlFake{}, strFake{err: fallo}),
	}

	for nombre, servicio := range casos {
		t.Run(nombre, func(t *testing.T) {
			if _, err := servicio.Prices(context.Background(), []string{"2026-01"}, 0); err == nil {
				t.Errorf("se tragó el error de %s", nombre)
			}
		})
	}
}

// ── Los meses proyectados ───────────────────────────────────────────────────

// Van DESPUÉS del último real y en orden, para que la matriz se lea de izquierda
// a derecha: los reales primero y los proyectados al final.
func TestPrices_ProyectaDespuesDelUltimoReal(t *testing.T) {
	servicio := proyeccion.NewProyeccionService(
		sdlFake{tarifas: []repositories.SdlRate{
			tarifa("2026-06", "CENS", 100),
			tarifa("2026-07", "CENS", 200),
		}},
		strFake{},
	)

	meses, err := servicio.Prices(context.Background(), []string{"2026-06", "2026-07"}, 3)
	if err != nil {
		t.Fatalf("Prices: %v", err)
	}
	if len(meses) != 5 {
		t.Fatalf("devolvió %d meses, se esperaban 2 reales + 3 proyectados", len(meses))
	}

	esperado := []struct {
		periodo    string
		proyectado bool
	}{
		{"2026-06", false}, {"2026-07", false},
		{"2026-08", true}, {"2026-09", true}, {"2026-10", true},
	}
	for i, e := range esperado {
		if meses[i].Period != e.periodo || meses[i].Projected != e.proyectado {
			t.Errorf("posición %d = %q proyectado=%v, se esperaba %q proyectado=%v",
				i, meses[i].Period, meses[i].Projected, e.periodo, e.proyectado)
		}
	}
}

// El precio proyectado es el promedio de los reales con precio.
func TestPrices_ElProyectadoPromediaLosReales(t *testing.T) {
	servicio := proyeccion.NewProyeccionService(
		sdlFake{tarifas: []repositories.SdlRate{
			tarifa("2026-06", "CENS", 100), // NT2 = 110
			tarifa("2026-07", "CENS", 200), // NT2 = 210
		}},
		strFake{},
	)

	meses, _ := servicio.Prices(context.Background(), []string{"2026-06", "2026-07"}, 1)

	if *meses[2].ActivaNT2 != 160 { // (110 + 210) / 2
		t.Errorf("activa NT2 proyectada = %v, se esperaba 160", *meses[2].ActivaNT2)
	}
}

// El valor STR no se proyecta: es plata que ya se liquidó o todavía no existe.
// Inventar un total daría un número que parece real y no lo es.
func TestPrices_NoProyectaElValorStr(t *testing.T) {
	servicio := proyeccion.NewProyeccionService(
		sdlFake{tarifas: []repositories.SdlRate{tarifa("2026-07", "CENS", 100)}},
		strFake{totales: map[string]float64{"2026-07": 1403159917}},
	)

	meses, _ := servicio.Prices(context.Background(), []string{"2026-07"}, 2)

	if meses[0].StrTotalCop == nil {
		t.Error("el mes real perdió su valor STR")
	}
	for _, m := range meses[1:] {
		if m.StrTotalCop != nil {
			t.Errorf("%s es proyectado y trae valor STR: %v", m.Period, *m.StrTotalCop)
		}
	}
}

// Cruza el año: diciembre proyecta enero del siguiente.
func TestPrices_ProyeccionCruzaElAnio(t *testing.T) {
	servicio := proyeccion.NewProyeccionService(
		sdlFake{tarifas: []repositories.SdlRate{tarifa("2026-12", "CENS", 100)}},
		strFake{},
	)

	meses, _ := servicio.Prices(context.Background(), []string{"2026-12"}, 2)

	if meses[1].Period != "2027-01" || meses[2].Period != "2027-02" {
		t.Errorf("no cruzó el año: %q, %q", meses[1].Period, meses[2].Period)
	}
}

// Sin nada real no hay de dónde promediar: no se proyecta.
func TestPrices_SinRealesNoProyecta(t *testing.T) {
	meses, _ := proyeccion.NewProyeccionService(sdlFake{}, strFake{}).
		Prices(context.Background(), []string{"2026-07"}, 3)

	for _, m := range meses {
		if m.Projected {
			t.Errorf("proyectó %s sin ningún mes real con precios", m.Period)
		}
	}
}
