package cargos_str_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"bia-bills/repositories"
	"bia-bills/services/cargos_str"
)

// Lecturas del servicio y resolución de nombres en el preview.
//
// Los métodos de lectura son delegaciones al repositorio, pero no por eso son
// gratis: cada uno abre un span y pasa el contexto, y un cruce entre ellos
// —Periods llamando a TotalsByPeriod, por ejemplo— devolvería datos plausibles
// del tipo equivocado. Acá se fija que cada uno llame a lo que le corresponde y
// devuelva lo que recibe sin tocarlo.

// lecturasFake registra qué método del repositorio se llamó y con qué.
type lecturasFake struct {
	strRepoFake

	filtroRecibido   repositories.StrChargeFilter
	periodosRecibido []string

	cargos       []repositories.StrCharge
	totales      map[string]float64
	periodos     []string
	errDeLectura error

	llamadas []string
}

func (f *lecturasFake) CurrentCharges(_ context.Context, filter repositories.StrChargeFilter) ([]repositories.StrCharge, error) {
	f.llamadas = append(f.llamadas, "CurrentCharges")
	f.filtroRecibido = filter
	return f.cargos, f.errDeLectura
}

func (f *lecturasFake) TotalsByPeriod(_ context.Context, periods []string) (map[string]float64, error) {
	f.llamadas = append(f.llamadas, "TotalsByPeriod")
	f.periodosRecibido = periods
	return f.totales, f.errDeLectura
}

func (f *lecturasFake) PeriodsWithCharges(context.Context) ([]string, error) {
	f.llamadas = append(f.llamadas, "PeriodsWithCharges")
	return f.periodos, f.errDeLectura
}

func TestServicio_CurrentChargesPasaElFiltroYDevuelveLoLeido(t *testing.T) {
	repo := &lecturasFake{
		cargos: []repositories.StrCharge{
			{Period: "2026-05", OperatorCode: "CHEC", OperatorName: "CENTRAL HIDROELECTRICA DE CALDAS S.A. E.S.P.", AmountPayable: 70_807_698},
		},
	}
	servicio := cargos_str.NewCargosStrService(repo, agentsRepoFake{})

	filtro := repositories.StrChargeFilter{
		Periods:       []string{"2026-05"},
		OperatorCodes: []string{"CHEC"},
	}

	res, err := servicio.CurrentCharges(context.Background(), filtro)
	if err != nil {
		t.Fatalf("CurrentCharges: %v", err)
	}

	if len(repo.llamadas) != 1 || repo.llamadas[0] != "CurrentCharges" {
		t.Errorf("llamó a %v", repo.llamadas)
	}
	if len(repo.filtroRecibido.Periods) != 1 || repo.filtroRecibido.Periods[0] != "2026-05" {
		t.Errorf("el filtro llegó alterado: %+v", repo.filtroRecibido)
	}
	if len(res) != 1 || res[0].AmountPayable != 70_807_698 {
		t.Errorf("resultado = %+v", res)
	}
}

func TestServicio_TotalsByPeriodDelegaYNoTocaLosTotales(t *testing.T) {
	repo := &lecturasFake{totales: map[string]float64{"2026-05": 1_460_833_304}}
	servicio := cargos_str.NewCargosStrService(repo, agentsRepoFake{})

	totales, err := servicio.TotalsByPeriod(context.Background(), []string{"2026-05", "2026-04"})
	if err != nil {
		t.Fatalf("TotalsByPeriod: %v", err)
	}

	if len(repo.llamadas) != 1 || repo.llamadas[0] != "TotalsByPeriod" {
		t.Errorf("llamó a %v", repo.llamadas)
	}
	if len(repo.periodosRecibido) != 2 {
		t.Errorf("los períodos llegaron alterados: %v", repo.periodosRecibido)
	}
	if totales["2026-05"] != 1_460_833_304 {
		t.Errorf("total = %v", totales["2026-05"])
	}
	// El período sin datos no debe aparecer: el front distingue "sin cargar" de
	// "cargado en cero".
	if _, hay := totales["2026-04"]; hay {
		t.Error("apareció un período que el repositorio no devolvió")
	}
}

func TestServicio_PeriodsDelegaEnPeriodsWithCharges(t *testing.T) {
	repo := &lecturasFake{periodos: []string{"2026-05", "2026-04"}}
	servicio := cargos_str.NewCargosStrService(repo, agentsRepoFake{})

	periodos, err := servicio.Periods(context.Background())
	if err != nil {
		t.Fatalf("Periods: %v", err)
	}

	if len(repo.llamadas) != 1 || repo.llamadas[0] != "PeriodsWithCharges" {
		t.Errorf("llamó a %v", repo.llamadas)
	}
	if len(periodos) != 2 || periodos[0] != "2026-05" {
		t.Errorf("períodos = %v", periodos)
	}
}

// ── Resolución de nombres en el preview ──────────────────────────────────────
//
// Sobre los archivos reales de testdata/, los mismos del test dorado. Lo que se
// verifica acá no son los montos —eso ya está cubierto— sino qué hace el servicio
// con el catálogo de agentes en cada escenario.

func TestPreview_ResuelveLosNombresDelCatalogo(t *testing.T) {
	archivos := cargarArchivos(t)
	nombres := map[string]string{
		"CHEC": "CENTRAL HIDROELECTRICA DE CALDAS S.A. E.S.P.",
		"AIRE": "AIR-E S.A.S. E.S.P.",
	}

	servicio := cargos_str.NewCargosStrService(&strRepoFake{}, agentsRepoFake{nombres: nombres})

	res, err := servicio.Preview(context.Background(), archivos, 2026, 5)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	chec, ok := buscar(res.Rows, "CHEC")
	if !ok {
		t.Fatal("CHEC no está en el resultado")
	}
	if chec.OperatorName != nombres["CHEC"] {
		t.Errorf("nombre de CHEC = %q", chec.OperatorName)
	}

	aire, ok := buscar(res.Rows, "AIRE")
	if !ok {
		t.Fatal("AIRE no está en el resultado")
	}
	if aire.OperatorName != nombres["AIRE"] {
		t.Errorf("nombre de AIRE = %q", aire.OperatorName)
	}
}

// A los operadores que el catálogo no resuelve se les muestra el código como
// nombre, y el aviso los lista. Es lo correcto para el preview: el usuario ve el
// monto igual y entiende qué le falta. Confirm, en cambio, sí bloquea —ahí el
// nombre se guarda y es NOT NULL—.
func TestPreview_SinNombreUsaElCodigoYAvisaCuales(t *testing.T) {
	archivos := cargarArchivos(t)

	// Catálogo parcial: solo CHEC. Todos los demás operadores de los archivos
	// reales quedan sin nombre.
	servicio := cargos_str.NewCargosStrService(&strRepoFake{}, agentsRepoFake{
		nombres: map[string]string{"CHEC": "CENTRAL HIDROELECTRICA DE CALDAS S.A. E.S.P."},
	})

	res, err := servicio.Preview(context.Background(), archivos, 2026, 5)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	aire, ok := buscar(res.Rows, "AIRE")
	if !ok {
		t.Fatal("AIRE no está en el resultado")
	}
	if aire.OperatorName != "AIRE" {
		t.Errorf("sin nombre en el catálogo debería mostrar el código, mostró %q", aire.OperatorName)
	}
	// Y el monto tiene que seguir intacto: la falta de nombre no toca las cifras.
	if aire.InvoiceAmount == 0 {
		t.Error("se perdió el monto de AIRE al no resolver el nombre")
	}

	var aviso string
	for _, w := range res.Warnings {
		if strings.Contains(w, "Sin nombre en el catálogo") {
			aviso = w
		}
	}
	if aviso == "" {
		t.Fatalf("no avisó qué operadores quedaron sin nombre: %v", res.Warnings)
	}
	if !strings.Contains(aviso, "AIRE") {
		t.Errorf("el aviso no lista a AIRE: %q", aviso)
	}
	if strings.Contains(aviso, "CHEC") {
		t.Errorf("el aviso lista a CHEC, que sí tenía nombre: %q", aviso)
	}
}

// Con errores críticos el preview corta antes de consultar el catálogo: no tiene
// sentido pegarle a la base para un lote que ya se sabe inválido.
func TestPreview_ConErroresCriticosNoConsultaElCatalogo(t *testing.T) {
	// Cuatro archivos de refactura: uno más que las tres columnas de ajuste, que
	// es un error crítico del parser.
	archivos := []cargos_str.UploadedFile{
		{Name: "BalanceSTRTipoFactu2026-MAY.xlsx", Content: []byte("x")},
		{Name: "BalanceSTRTipoReFactu2026-ABR-1.xlsx", Content: []byte("x")},
		{Name: "BalanceSTRTipoReFactu2026-MAR-1.xlsx", Content: []byte("x")},
		{Name: "BalanceSTRTipoReFactu2026-FEB-1.xlsx", Content: []byte("x")},
		{Name: "BalanceSTRTipoReFactu2026-ENE-1.xlsx", Content: []byte("x")},
	}

	catalogo := &agentsRepoEspia{}
	servicio := cargos_str.NewCargosStrService(&strRepoFake{}, catalogo)

	res, err := servicio.Preview(context.Background(), archivos, 2026, 5)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	if len(res.CriticalErrors) == 0 {
		t.Fatalf("se esperaba error crítico con 4 refacturas; avisos: %v", res.Warnings)
	}
	if catalogo.consultado {
		t.Error("consultó el catálogo de agentes para un lote que ya era inválido")
	}
}

// agentsRepoEspia solo registra si lo llamaron.
type agentsRepoEspia struct {
	consultado bool
}

func (f *agentsRepoEspia) NamesByOperator(context.Context, map[string]string) (map[string]string, error) {
	f.consultado = true
	return map[string]string{}, nil
}

func (f *agentsRepoEspia) NamesByAgentCode(context.Context, []string) (map[string]string, error) {
	return nil, nil
}

func TestServicio_LasLecturasPropaganElError(t *testing.T) {
	casos := map[string]func(cargos_str.CargosStrService) error{
		"CurrentCharges": func(s cargos_str.CargosStrService) error {
			_, err := s.CurrentCharges(context.Background(), repositories.StrChargeFilter{})
			return err
		},
		"TotalsByPeriod": func(s cargos_str.CargosStrService) error {
			_, err := s.TotalsByPeriod(context.Background(), []string{"2026-05"})
			return err
		},
		"Periods": func(s cargos_str.CargosStrService) error {
			_, err := s.Periods(context.Background())
			return err
		},
	}

	for nombre, leer := range casos {
		t.Run(nombre, func(t *testing.T) {
			repo := &lecturasFake{errDeLectura: errors.New("base caída")}
			servicio := cargos_str.NewCargosStrService(repo, agentsRepoFake{})

			err := leer(servicio)
			if err == nil {
				t.Fatal("no propagó el error de la base")
			}
			if !strings.Contains(err.Error(), "base caída") {
				t.Errorf("perdió la causa: %v", err)
			}
		})
	}
}
