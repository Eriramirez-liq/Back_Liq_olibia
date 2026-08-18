package cargos_str_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"bia-bills/models"
	"bia-bills/repositories"
	"bia-bills/services/cargos_str"
)

// Tests del servicio con dobles a mano.
//
// Interesa sobre todo el Confirm: escribe en DOS bases sin transacción que las
// abarque, así que el comportamiento ante un fallo parcial es lo que hay que
// tener clavado. Un insumo sin resultado deja la matriz mostrando de menos sin
// que nadie se entere.
//
// (Para la CI de bia-bills, `make mock` genera los mocks con mockery a partir de
// las interfaces; estos dobles son para poder testear sin la herramienta.)

type strRepoFake struct {
	insumosInsertados []models.LiquidationsStrInput
	cargosInsertados  []models.LiquidationsStrCharge
	borradoDeCarga    string

	errAlInsertarInsumos error
	errAlInsertarCargos  error
	errAlBorrar          error
}

func (f *strRepoFake) InsertInputs(_ context.Context, rows []models.LiquidationsStrInput) error {
	if f.errAlInsertarInsumos != nil {
		return f.errAlInsertarInsumos
	}
	f.insumosInsertados = append(f.insumosInsertados, rows...)
	return nil
}

func (f *strRepoFake) InsertCharges(_ context.Context, rows []models.LiquidationsStrCharge) error {
	if f.errAlInsertarCargos != nil {
		return f.errAlInsertarCargos
	}
	f.cargosInsertados = append(f.cargosInsertados, rows...)
	return nil
}

func (f *strRepoFake) DeleteInputsByLoad(_ context.Context, loadID string) error {
	f.borradoDeCarga = loadID
	return f.errAlBorrar
}

func (f *strRepoFake) CurrentCharges(context.Context, repositories.StrChargeFilter) ([]repositories.StrCharge, error) {
	return nil, nil
}
func (f *strRepoFake) TotalsByPeriod(context.Context, []string) (map[string]float64, error) {
	return nil, nil
}
func (f *strRepoFake) PeriodsWithCharges(context.Context) ([]string, error) { return nil, nil }
func (f *strRepoFake) Loads(context.Context, []string) ([]repositories.StrLoad, error) {
	return nil, nil
}

type agentsRepoFake struct {
	nombres map[string]string
	err     error
}

func (f agentsRepoFake) NamesByOperator(context.Context, map[string]string) (map[string]string, error) {
	return f.nombres, f.err
}

// Cargos STR no la usa —resuelve por operador, no por agente— pero la interfaz
// del repositorio la incluye porque Tarifas SDL sí la necesita.
func (f agentsRepoFake) NamesByAgentCode(context.Context, []string) (map[string]string, error) {
	return nil, nil
}

func metaDePrueba() cargos_str.LoadMeta {
	return cargos_str.LoadMeta{
		CreatedBy:   "Erika Ramírez",
		CreatedByID: "u-123",
		SourceFiles: []string{"BalanceSTRTipoFactu2026-MAY.xlsx"},
	}
}

func filasDePrueba() []cargos_str.StrRow {
	ajuste := -4442.0
	return []cargos_str.StrRow{{
		OperatorCode:     "CHEC",
		Period:           "2026-05",
		InvoiceAmount:    70_812_140,
		Reinvoice1Amount: &ajuste,
		AmountPayable:    70_807_698,
	}}
}

func TestConfirm(t *testing.T) {
	nombres := map[string]string{"CHEC": "CENTRAL HIDROELECTRICA DE CALDAS S.A. E.S.P."}

	t.Run("guarda en las dos bases con el mismo load_id", func(t *testing.T) {
		strRepo := &strRepoFake{}
		service := cargos_str.NewCargosStrService(strRepo, agentsRepoFake{nombres: nombres})

		loadID, err := service.Confirm(context.Background(), filasDePrueba(), metaDePrueba())
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if loadID == "" {
			t.Fatal("no devolvió load_id")
		}
		if len(strRepo.insumosInsertados) != 1 || len(strRepo.cargosInsertados) != 1 {
			t.Fatalf("insumos=%d cargos=%d, se esperaba 1 y 1",
				len(strRepo.insumosInsertados), len(strRepo.cargosInsertados))
		}

		// El mismo id en las dos bases es lo que permite trazar un valor a pagar
		// hasta el insumo que lo produjo.
		if strRepo.insumosInsertados[0].LoadID != loadID || strRepo.cargosInsertados[0].LoadID != loadID {
			t.Error("el load_id no coincide entre las dos bases")
		}

		// El nombre se resuelve en el servidor, no se toma de lo que mandó el
		// navegador: es NOT NULL y es lo que va a ver Finanzas.
		if got := strRepo.cargosInsertados[0].OperatorName; got != nombres["CHEC"] {
			t.Errorf("operator_name = %q, se esperaba %q", got, nombres["CHEC"])
		}
	})

	t.Run("si falla la segunda base, borra el insumo de esa carga", func(t *testing.T) {
		strRepo := &strRepoFake{errAlInsertarCargos: errors.New("caída")}
		service := cargos_str.NewCargosStrService(strRepo, agentsRepoFake{nombres: nombres})

		_, err := service.Confirm(context.Background(), filasDePrueba(), metaDePrueba())
		if err == nil {
			t.Fatal("se esperaba error")
		}

		// Sin este borrado quedaría un insumo sin resultado: la matriz mostraría
		// de menos y nadie se enteraría.
		if strRepo.borradoDeCarga == "" {
			t.Error("no hizo el rollback del insumo")
		}
		if strRepo.insumosInsertados[0].LoadID != strRepo.borradoDeCarga {
			t.Error("borró una carga distinta de la que había insertado")
		}
	})

	t.Run("no guarda nada si un operador no tiene nombre en el catálogo", func(t *testing.T) {
		strRepo := &strRepoFake{}
		service := cargos_str.NewCargosStrService(strRepo, agentsRepoFake{nombres: map[string]string{}})

		_, err := service.Confirm(context.Background(), filasDePrueba(), metaDePrueba())
		if err == nil {
			t.Fatal("se esperaba error")
		}
		if !strings.Contains(err.Error(), "CHEC") {
			t.Errorf("el error no dice qué operador falta: %v", err)
		}
		if len(strRepo.insumosInsertados) != 0 {
			t.Error("escribió el insumo aunque faltaba un nombre")
		}
	})

	t.Run("rechaza un lote vacío", func(t *testing.T) {
		strRepo := &strRepoFake{}
		service := cargos_str.NewCargosStrService(strRepo, agentsRepoFake{nombres: nombres})

		if _, err := service.Confirm(context.Background(), nil, metaDePrueba()); err == nil {
			t.Fatal("se esperaba error con un lote vacío")
		}
	})
}

func TestPreview_SigueSinElCatalogoDeAgentes(t *testing.T) {
	service := cargos_str.NewCargosStrService(
		&strRepoFake{},
		agentsRepoFake{err: errors.New("catálogo caído")},
	)

	res, err := service.Preview(context.Background(), cargarArchivos(t), anio, mes)
	if err != nil {
		t.Fatalf("el preview no debería fallar por el catálogo: %v", err)
	}
	if len(res.Rows) != operadores {
		t.Fatalf("filas = %d, se esperaban %d", len(res.Rows), operadores)
	}

	// Lo que el usuario valida en pantalla son los montos: bloquear el preview
	// porque no se pudo resolver un nombre sería peor que mostrarlo con aviso.
	chec, ok := buscar(res.Rows, "CHEC")
	if !ok {
		t.Fatal("CHEC no está en el resultado")
	}
	if chec.AmountPayable != checAPagar {
		t.Errorf("a pagar = %.2f, se esperaba %.2f", chec.AmountPayable, checAPagar)
	}

	var avisa bool
	for _, w := range res.Warnings {
		if strings.Contains(w, "catálogo de agentes") {
			avisa = true
		}
	}
	if !avisa {
		t.Error("no avisó que no pudo resolver los nombres")
	}
}
