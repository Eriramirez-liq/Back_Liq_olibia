package cargos_str_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bia-bills/services/cargos_str"
)

// Test dorado del parser de Cargos STR.
//
// Reproduce los números que validó la versión TypeScript contra los mismos
// archivos reales. Es la única forma de saber que el port a Go no introdujo una
// regresión silenciosa: la lógica tiene varias heurísticas (detección del
// encabezado, búsqueda de la fila BIAC-BIAE, homologación de columnas) y un error
// ahí no rompe nada, solo devuelve un número distinto.
//
//	CHEC = 70.812.140 (factura) − 906 − 3.536 (ajustes) = 70.807.698
//	lote = 1.460.833.304
//
// Los archivos viven en archivos_ejemplo/STR/ y NO están versionados (son insumos
// reales). Sin ellos el test se saltea en vez de fallar, así que en CI no corre.

const (
	periodo   = "2026-05"
	anio, mes = 2026, 5

	checFactura  = 70_812_140.0
	checAjustes  = -906.0 - 3_536.0
	checAPagar   = 70_807_698.0
	totalLote    = 1_460_833_304.0
	operadores   = 23
	aireFactura  = 142_265_108.0
)

func cargarArchivos(t *testing.T) []cargos_str.UploadedFile {
	t.Helper()

	dir := filepath.Join("..", "..", "archivos_ejemplo", "STR")
	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("sin archivos de ejemplo en %s: %v", dir, err)
	}

	var archivos []cargos_str.UploadedFile
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".xlsx") {
			continue
		}
		contenido, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("no se pudo leer %s: %v", e.Name(), err)
		}
		archivos = append(archivos, cargos_str.UploadedFile{Name: e.Name(), Content: contenido})
	}

	if len(archivos) == 0 {
		t.Skip("no hay .xlsx en archivos_ejemplo/STR")
	}

	return archivos
}

func buscar(rows []cargos_str.StrRow, code string) (cargos_str.StrRow, bool) {
	for _, r := range rows {
		if r.OperatorCode == code {
			return r, true
		}
	}
	return cargos_str.StrRow{}, false
}

func TestParseStrInputs_ArchivosReales(t *testing.T) {
	archivos := cargarArchivos(t)
	res := cargos_str.ParseStrInputs(archivos, anio, mes)

	if len(res.CriticalErrors) > 0 {
		t.Fatalf("errores críticos inesperados: %v", res.CriticalErrors)
	}

	t.Run("una fila por operador, no por archivo", func(t *testing.T) {
		if len(res.Rows) != operadores {
			t.Errorf("filas = %d, se esperaban %d", len(res.Rows), operadores)
		}
	})

	t.Run("el período sale del filtro, no del nombre del archivo", func(t *testing.T) {
		// El lote trae refacturas de 2025-NOV y 2026-MAR; todas quedan en 2026-05.
		for _, r := range res.Rows {
			if r.Period != periodo {
				t.Fatalf("%s tiene período %q, se esperaba %q", r.OperatorCode, r.Period, periodo)
			}
		}
	})

	t.Run("CHEC reproduce el valor a pagar validado", func(t *testing.T) {
		chec, ok := buscar(res.Rows, "CHEC")
		if !ok {
			t.Fatal("CHEC no está en el resultado")
		}
		if chec.InvoiceAmount != checFactura {
			t.Errorf("factura = %.2f, se esperaba %.2f", chec.InvoiceAmount, checFactura)
		}
		if chec.Reinvoice1Amount == nil || chec.Reinvoice2Amount == nil {
			t.Fatal("faltan los dos ajustes del lote")
		}
		if suma := *chec.Reinvoice1Amount + *chec.Reinvoice2Amount; suma != checAjustes {
			t.Errorf("ajustes = %.2f, se esperaba %.2f", suma, checAjustes)
		}
		if chec.AmountPayable != checAPagar {
			t.Errorf("a pagar = %.2f, se esperaba %.2f", chec.AmountPayable, checAPagar)
		}
	})

	t.Run("distingue archivo ausente (nil) de valor cero", func(t *testing.T) {
		// El lote trae 2 refacturas: la tercera columna queda sin dato en todos.
		for _, r := range res.Rows {
			if r.Reinvoice3Amount != nil {
				t.Fatalf("%s tiene ajuste 3 = %.2f y ese archivo no vino", r.OperatorCode, *r.Reinvoice3Amount)
			}
			if r.Reinvoice1Amount == nil {
				t.Fatalf("%s no tiene ajuste 1 y ese archivo sí vino", r.OperatorCode)
			}
		}

		// AIRE aparece en los ajustes con valor cero, no ausente. Y suma sus dos
		// columnas del Excel (CSID + CSSD) en un solo operador.
		aire, ok := buscar(res.Rows, "AIRE")
		if !ok {
			t.Fatal("AIRE no está en el resultado")
		}
		if suma := *aire.Reinvoice1Amount + *aire.Reinvoice2Amount; suma != 0 {
			t.Errorf("ajustes de AIRE = %.2f, se esperaba 0", suma)
		}
		if aire.InvoiceAmount != aireFactura {
			t.Errorf("factura de AIRE = %.2f, se esperaba %.2f (CSID + CSSD)", aire.InvoiceAmount, aireFactura)
		}
	})

	t.Run("el total del lote cuadra con la suma de los operadores", func(t *testing.T) {
		var total float64
		for _, r := range res.Rows {
			total += r.AmountPayable

			// Y cada fila cuadra consigo misma.
			suma := r.InvoiceAmount
			for _, ajuste := range []*float64{r.Reinvoice1Amount, r.Reinvoice2Amount, r.Reinvoice3Amount} {
				if ajuste != nil {
					suma += *ajuste
				}
			}
			if suma != r.AmountPayable {
				t.Errorf("%s: factura+ajustes = %.2f pero a pagar = %.2f", r.OperatorCode, suma, r.AmountPayable)
			}
		}

		if total != totalLote {
			t.Errorf("total del lote = %.2f, se esperaba %.2f", total, totalLote)
		}
	})
}

func TestParseStrInputs_ReglasDelLote(t *testing.T) {
	falso := func(nombre string) cargos_str.UploadedFile {
		return cargos_str.UploadedFile{Name: nombre, Content: []byte{}}
	}

	t.Run("corta la carga si llegan más ajustes que columnas", func(t *testing.T) {
		res := cargos_str.ParseStrInputs([]cargos_str.UploadedFile{
			falso("BalanceSTRTipoFactu2026-MAY.xlsx"),
			falso("BalanceSTRTipoReFactu2025-NOV-1.xlsx"),
			falso("BalanceSTRTipoReFactu2025-DIC-1.xlsx"),
			falso("BalanceSTRTipoReFactu2026-ENE-1.xlsx"),
			falso("BalanceSTRTipoReFactu2026-MAR-1.xlsx"),
		}, anio, mes)

		// Falla explícita: nunca descartar un ajuste en silencio.
		if len(res.Rows) != 0 {
			t.Errorf("devolvió %d filas, no debería procesar nada", len(res.Rows))
		}
		if len(res.CriticalErrors) != 1 {
			t.Fatalf("errores críticos = %d, se esperaba 1: %v", len(res.CriticalErrors), res.CriticalErrors)
		}
		if !strings.Contains(res.CriticalErrors[0], "BalanceSTRTipoReFactu2026-MAR-1.xlsx") {
			t.Errorf("el error no nombra los archivos: %q", res.CriticalErrors[0])
		}
	})

	t.Run("avisa si no viene ningún archivo", func(t *testing.T) {
		res := cargos_str.ParseStrInputs(nil, anio, mes)
		if len(res.CriticalErrors) != 1 {
			t.Errorf("errores críticos = %d, se esperaba 1", len(res.CriticalErrors))
		}
	})
}
