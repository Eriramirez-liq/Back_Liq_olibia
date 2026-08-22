package tc1_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"bia-bills/models"
	"bia-bills/repositories"
	"bia-bills/services/tc1"
)

// Tests del servicio de TC1.
//
// ⚠️ TRASVASE: archivo del paquete tc1. Ver docs/backend/migracion-a-go.md.
//
// El servicio no parsea —eso pasa en el navegador— así que lo que hay que probar
// es lo otro: qué rechaza antes de guardar, y que lo que guarda quede rotulado
// con el período y el operador elegidos y no con lo que diga el archivo.

type repoFake struct {
	guardadas []models.LiquidationsTc1Input
	cargues   []repositories.Tc1Load
	err       error
}

func (r *repoFake) InsertInputs(_ context.Context, rows []models.LiquidationsTc1Input) error {
	if r.err != nil {
		return r.err
	}
	r.guardadas = append(r.guardadas, rows...)
	return nil
}

func (r *repoFake) CurrentInputs(context.Context, repositories.Tc1Filter) ([]models.LiquidationsTc1Input, error) {
	return r.guardadas, r.err
}
func (r *repoFake) Loads(context.Context, []string) ([]repositories.Tc1Load, error) {
	return r.cargues, r.err
}
func (r *repoFake) Periods(context.Context) ([]string, error) { return nil, r.err }

func filaValida(frontera string) tc1.Row {
	return tc1.Row{
		Niu:                  "1940639",
		CodFronteraComercial: frontera,
		NivelDeTension:       "2",
		IDComercializador:    "62371",
		Latitud:              "35003989",
	}
}

func TestServicio_GuardaYRotulaConLoElegidoEnNuevaCarga(t *testing.T) {
	repo := &repoFake{}
	servicio := tc1.NewTc1Service(repo, nil)

	loadID, err := servicio.Confirm(context.Background(), "2026-02", "cens",
		[]tc1.Row{filaValida("Frt32152"), filaValida("Frt11726")},
		tc1.LoadMeta{CreatedBy: "Erika", CreatedByID: "uid-1", SourceFile: "TC1_FEB.csv"})
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if loadID == "" {
		t.Error("no devolvió el id del cargue")
	}
	if len(repo.guardadas) != 2 {
		t.Fatalf("guardó %d filas, se esperaban 2", len(repo.guardadas))
	}

	for _, f := range repo.guardadas {
		// El período y el operador NO salen de la fila: los elige la persona en
		// Nueva carga y aplican a todo el archivo.
		if f.Period != "2026-02" {
			t.Errorf("período = %q, se esperaba 2026-02", f.Period)
		}
		// En mayúsculas, como el resto del módulo guarda los operadores.
		if f.OperatorCode != "CENS" {
			t.Errorf("operador = %q, se esperaba CENS", f.OperatorCode)
		}
		if f.LoadID != loadID {
			t.Errorf("la fila no quedó atada al cargue: %q vs %q", f.LoadID, loadID)
		}
		if f.CreatedBy != "Erika" || f.CreatedByID != "uid-1" || f.SourceFile != "TC1_FEB.csv" {
			t.Errorf("se perdió la traza del cargue: %+v", f)
		}
	}
	// El valor llega tal cual, sin convertir: la latitud viene en microgrados.
	if repo.guardadas[0].Latitud != "35003989" {
		t.Errorf("cambió el valor original: %q", repo.guardadas[0].Latitud)
	}
}

// El período con otra forma no llega a la base: allá hay un CHECK, pero el
// mensaje de Postgres no le sirve a nadie. El formato viejo "2-2026" del export
// a Metabase es el caso concreto que hay que atajar.
func TestServicio_PeriodoConMalaFormaSeRechaza(t *testing.T) {
	for _, periodo := range []string{"2-2026", "2026-2", "2026-13", "febrero", ""} {
		t.Run(periodo, func(t *testing.T) {
			repo := &repoFake{}
			_, err := tc1.NewTc1Service(repo, nil).Confirm(
				context.Background(), periodo, "CENS", []tc1.Row{filaValida("Frt1")}, tc1.LoadMeta{})

			if err == nil {
				t.Fatalf("aceptó el período %q", periodo)
			}
			if len(repo.guardadas) > 0 {
				t.Error("guardó pese al período inválido")
			}
		})
	}
}

// La frontera es la clave con la que después se cruza contra Facturación. Una
// repetida duplicaría el cruce; una vacía es una fila que nadie va a poder
// conciliar.
func TestServicio_FronteraRepetidaCortaYDiceCuales(t *testing.T) {
	repo := &repoFake{}

	_, err := tc1.NewTc1Service(repo, nil).Confirm(context.Background(), "2026-02", "CENS",
		[]tc1.Row{filaValida("Frt1"), filaValida("Frt2"), filaValida("Frt1")}, tc1.LoadMeta{})

	if err == nil {
		t.Fatal("aceptó una frontera repetida")
	}
	// El mensaje tiene que decir cuál y dónde, o no se sabe qué corregir en un
	// archivo de cientos de filas.
	for _, esperado := range []string{"Frt1", "1", "3"} {
		if !strings.Contains(err.Error(), esperado) {
			t.Errorf("el mensaje no menciona %q: %s", esperado, err)
		}
	}
	if len(repo.guardadas) > 0 {
		t.Error("guardó pese a la frontera repetida")
	}
}

func TestServicio_FronteraVaciaCortaEIndicaLaFila(t *testing.T) {
	repo := &repoFake{}

	_, err := tc1.NewTc1Service(repo, nil).Confirm(context.Background(), "2026-02", "CENS",
		[]tc1.Row{filaValida("Frt1"), filaValida("   ")}, tc1.LoadMeta{})

	if err == nil {
		t.Fatal("aceptó una frontera vacía")
	}
	if !strings.Contains(err.Error(), "fila 2") {
		t.Errorf("el mensaje no dice qué fila: %s", err)
	}
}

func TestServicio_SinFilasOSinOperadorSeRechaza(t *testing.T) {
	casos := map[string]struct {
		operador string
		filas    []tc1.Row
	}{
		"sin filas":    {"CENS", nil},
		"sin operador": {"  ", []tc1.Row{filaValida("Frt1")}},
	}

	for nombre, caso := range casos {
		t.Run(nombre, func(t *testing.T) {
			_, err := tc1.NewTc1Service(&repoFake{}, nil).Confirm(
				context.Background(), "2026-02", caso.operador, caso.filas, tc1.LoadMeta{})

			if err == nil {
				t.Errorf("aceptó un cargue %s", nombre)
			}
		})
	}
}

// Si la base falla, el cargue falla. Sin esto, la pantalla diría "cargado" y no
// habría nada guardado.
func TestServicio_ErrorDeLaBaseSePropaga(t *testing.T) {
	repo := &repoFake{err: errors.New("se cayó la conexión")}

	_, err := tc1.NewTc1Service(repo, nil).Confirm(context.Background(), "2026-02", "CENS",
		[]tc1.Row{filaValida("Frt1")}, tc1.LoadMeta{})

	if err == nil {
		t.Fatal("se tragó el error de la base")
	}
	if !strings.Contains(err.Error(), "se cayó la conexión") {
		t.Errorf("el error no conserva la causa: %s", err)
	}
}

// ── Estado del período: cuántos de los 21 cargaron ──────────────────────────

func servicioConCatalogo(repo *repoFake) tc1.Tc1Service {
	return tc1.NewTc1Service(repo, []string{"CENS", "CHEC", "EPM"})
}

func TestServicio_StatusSeparaCargadosDePendientes(t *testing.T) {
	repo := &repoFake{cargues: []repositories.Tc1Load{{OperatorCode: "CHEC"}}}

	estado, err := servicioConCatalogo(repo).Status(context.Background(), "2026-02")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if len(estado.Expected) != 3 {
		t.Errorf("se esperaban 3 operadores en el catálogo, hay %d", len(estado.Expected))
	}
	if len(estado.Loaded) != 1 || estado.Loaded[0] != "CHEC" {
		t.Errorf("cargados = %v, se esperaba [CHEC]", estado.Loaded)
	}
	// En el orden del catálogo, no en el de un mapa: una lista que cambia de
	// orden en cada consulta es incómoda de leer en pantalla.
	if len(estado.Pending) != 2 || estado.Pending[0] != "CENS" || estado.Pending[1] != "EPM" {
		t.Errorf("pendientes = %v, se esperaba [CENS EPM] en ese orden", estado.Pending)
	}
}

// Dos cargues del mismo operador siguen siendo UN operador: el modelo es
// append-only y recargar un archivo corregido es normal.
func TestServicio_StatusCuentaOperadoresNoCargues(t *testing.T) {
	repo := &repoFake{cargues: []repositories.Tc1Load{
		{OperatorCode: "CHEC"}, {OperatorCode: "CHEC"}, {OperatorCode: "CENS"},
	}}

	estado, err := servicioConCatalogo(repo).Status(context.Background(), "2026-02")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(estado.Loaded) != 2 {
		t.Errorf("cargados = %v, se esperaban 2 operadores y no 3 cargues", estado.Loaded)
	}
}

func TestServicio_StatusRechazaPeriodoConMalaForma(t *testing.T) {
	if _, err := servicioConCatalogo(&repoFake{}).Status(context.Background(), "2-2026"); err == nil {
		t.Error("aceptó un período con la forma vieja de Metabase")
	}
}

// Sin nada cargado, todos están pendientes. Es el caso del primer día del mes y
// tiene que decirlo, no devolver una lista vacía.
func TestServicio_StatusSinCarguesTodosPendientes(t *testing.T) {
	estado, err := servicioConCatalogo(&repoFake{}).Status(context.Background(), "2026-02")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(estado.Loaded) != 0 || len(estado.Pending) != 3 {
		t.Errorf("cargados=%v pendientes=%v", estado.Loaded, estado.Pending)
	}
}
