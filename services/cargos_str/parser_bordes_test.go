package cargos_str_test

import (
	"strings"
	"testing"

	"bia-bills/services/cargos_str"

	"github.com/xuri/excelize/v2"
)

// Casos borde del parser.
//
// Los fixtures reales cubren el camino bueno; esto cubre lo que pasa cuando el
// archivo no es lo esperado. Acá sí se construyen archivos sintéticos, pero NO
// para validar montos —eso es trabajo del test dorado— sino para ejercitar rutas
// que los archivos reales no recorren.

func TestParser_ArchivoQueNoEsExcel(t *testing.T) {
	res := cargos_str.ParseStrInputs([]cargos_str.UploadedFile{
		{Name: "BalanceSTRTipoFactu2026-MAY.xlsx", Content: []byte("esto no es un xlsx")},
	}, 2026, 5)

	// Avisa y sigue, no explota: un archivo corrupto no debe tumbar el request.
	if len(res.CriticalErrors) > 0 {
		t.Errorf("no debería ser error crítico: %v", res.CriticalErrors)
	}
	if len(res.Rows) != 0 {
		t.Errorf("devolvió %d filas de un archivo ilegible", len(res.Rows))
	}

	var aviso bool
	for _, w := range res.Warnings {
		if strings.Contains(w, "no se pudo leer como Excel") {
			aviso = true
		}
	}
	if !aviso {
		t.Errorf("no avisó que el archivo es ilegible: %v", res.Warnings)
	}
}

func TestParser_NombreSinTipoReconocible(t *testing.T) {
	res := cargos_str.ParseStrInputs([]cargos_str.UploadedFile{
		{Name: "reporte_cualquiera.xlsx", Content: []byte("x")},
	}, 2026, 5)

	var aviso bool
	for _, w := range res.Warnings {
		if strings.Contains(w, "omitido") {
			aviso = true
		}
	}
	if !aviso {
		t.Errorf("no avisó que omitió el archivo: %v", res.Warnings)
	}
}

func TestParser_SoloAjustesSinFactura(t *testing.T) {
	res := cargos_str.ParseStrInputs([]cargos_str.UploadedFile{
		{Name: "BalanceSTRTipoReFactu2026-MAR-1.xlsx", Content: []byte("x")},
	}, 2026, 5)

	var aviso bool
	for _, w := range res.Warnings {
		if strings.Contains(w, "no trae archivo de factura") {
			aviso = true
		}
	}
	if !aviso {
		t.Errorf("no avisó que falta la factura: %v", res.Warnings)
	}
}

// excelSintetico arma un xlsx en memoria con una fila de encabezados y una fila
// BIAC-BIAE, para ejercitar rutas que los archivos reales no recorren.
func excelSintetico(t *testing.T, hoja string, encabezados []string, valores []string) []byte {
	t.Helper()

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	indice, err := f.NewSheet(hoja)
	if err != nil {
		t.Fatalf("NewSheet: %v", err)
	}
	f.SetActiveSheet(indice)

	// Fila 1: encabezados. Fila 2: la de BIAC-BIAE con los valores.
	for i, valor := range encabezados {
		celda, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(hoja, celda, valor); err != nil {
			t.Fatalf("SetCellValue: %v", err)
		}
	}
	for i, valor := range valores {
		celda, _ := excelize.CoordinatesToCellName(i+1, 2)
		if err := f.SetCellValue(hoja, celda, valor); err != nil {
			t.Fatalf("SetCellValue: %v", err)
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer: %v", err)
	}

	return buf.Bytes()
}

// Un encabezado con DOS códigos en la misma celda tiene que resolverse siempre
// igual. Antes de la corrección el parser recorría el mapa de homologación
// directamente, y el orden de iteración de un mapa en Go es aleatorio: el monto
// quedaba atribuido a un operador distinto en cada ejecución.
func TestParser_EncabezadoConDosCodigosEsDeterministico(t *testing.T) {
	contenido := excelSintetico(t, "BalSTR01",
		[]string{"BIAC - BIAE", "CMMD CHCD"}, // celda con dos códigos: AFINIA y CHEC
		[]string{"BIAC - BIAE", "5000"})

	archivo := []cargos_str.UploadedFile{
		{Name: "BalanceSTRTipoFactu2026-MAY.xlsx", Content: contenido},
	}

	primero := cargos_str.ParseStrInputs(archivo, 2026, 5)
	if len(primero.Rows) != 1 {
		t.Fatalf("devolvió %d filas: %+v (avisos: %v)", len(primero.Rows), primero.Rows, primero.Warnings)
	}
	ganador := primero.Rows[0].OperatorCode

	// Con orden alfabético de códigos, CHCD va antes que CMMD, así que gana CHEC.
	if ganador != "CHEC" {
		t.Errorf("ganó %q; con los códigos ordenados debería ganar CHEC (CHCD < CMMD)", ganador)
	}

	// Y sobre todo: el mismo resultado en cada corrida.
	for i := 0; i < 30; i++ {
		res := cargos_str.ParseStrInputs(archivo, 2026, 5)
		if len(res.Rows) != 1 || res.Rows[0].OperatorCode != ganador {
			t.Fatalf("corrida %d dio un resultado distinto: %+v", i, res.Rows)
		}
	}
}

func TestParser_HojaSinFilaBiac(t *testing.T) {
	contenido := excelSintetico(t, "BalSTR01",
		[]string{"otra cosa", "CHCD"},
		[]string{"nada", "5000"}) // no hay ninguna celda con BIAC y BIAE

	res := cargos_str.ParseStrInputs([]cargos_str.UploadedFile{
		{Name: "BalanceSTRTipoFactu2026-MAY.xlsx", Content: contenido},
	}, 2026, 5)

	if len(res.Rows) != 0 {
		t.Errorf("devolvió %d filas sin encontrar la fila BIAC-BIAE", len(res.Rows))
	}

	var aviso bool
	for _, w := range res.Warnings {
		if strings.Contains(w, "BIAC - BIAE") {
			aviso = true
		}
	}
	if !aviso {
		t.Errorf("no avisó que falta la fila BIAC-BIAE: %v", res.Warnings)
	}
}

// Convención de números: la COMA es separador de miles y el PUNTO es decimal.
//
// Es la convención de los valores crudos de Excel, que es lo que se lee con
// RawCellValue — ahí un monto viene como "70812140" o "70812140.5", nunca con
// separadores de miles. Idéntica a la del parser TypeScript, verificado valor por
// valor.
//
// Ojo con la consecuencia: un texto en formato colombiano como "1.000" se lee
// como 1, no como mil. Con valores crudos eso no pasa, pero si alguna vez se
// dejara de usar RawCellValue habría que revisar esto antes que nada.
func TestParser_ValoresDeCelda(t *testing.T) {
	casos := []struct {
		nombre   string
		celda    string
		esperado float64
	}{
		{"entero", "5000", 5000},
		{"negativo", "-906", -906},
		{"coma como separador de miles", "70,812,140", 70_812_140},
		{"punto como decimal", "1.5", 1.5},
		{"símbolo de moneda y espacios se descartan", "$ 4442  ", 4442},
		{"formato colombiano se lee como decimal", "1.000", 1}, // documenta el límite

		// Notación científica: excelize la devuelve así para valores muy chicos o
		// muy grandes. Antes se descartaba en silencio —al quitar la "e" quedaba
		// "1.5-05", que no parsea— y la celda se trataba como vacía. Un ajuste de
		// refactura chico dejaba de restarse sin que nada fallara.
		{"científica positiva", "1.5e+06", 1_500_000},
		{"científica negativa", "-5.0623e-05", -0.000050623},
		{"científica con E mayúscula", "2.5E+03", 2500},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			contenido := excelSintetico(t, "BalSTR01",
				[]string{"BIAC - BIAE", "CHCD"},
				[]string{"BIAC - BIAE", caso.celda})

			res := cargos_str.ParseStrInputs([]cargos_str.UploadedFile{
				{Name: "BalanceSTRTipoFactu2026-MAY.xlsx", Content: contenido},
			}, 2026, 5)

			if len(res.Rows) != 1 {
				t.Fatalf("devolvió %d filas (avisos: %v)", len(res.Rows), res.Warnings)
			}
			if got := res.Rows[0].InvoiceAmount; got != caso.esperado {
				t.Errorf("celda %q → %.2f, se esperaba %.2f", caso.celda, got, caso.esperado)
			}
		})
	}
}

func TestParser_CeldaVaciaNoCuentaComoCero(t *testing.T) {
	contenido := excelSintetico(t, "BalSTR01",
		[]string{"BIAC - BIAE", "CHCD", "EPMD"},
		[]string{"BIAC - BIAE", "", "300"}) // CHEC sin valor, EPM con 300

	res := cargos_str.ParseStrInputs([]cargos_str.UploadedFile{
		{Name: "BalanceSTRTipoFactu2026-MAY.xlsx", Content: contenido},
	}, 2026, 5)

	// Una celda vacía no genera fila para ese operador: no es lo mismo que un cero.
	for _, r := range res.Rows {
		if r.OperatorCode == "CHEC" {
			t.Errorf("CHEC apareció con %.2f y su celda estaba vacía", r.InvoiceAmount)
		}
	}

	var epm bool
	for _, r := range res.Rows {
		if r.OperatorCode == "EPM" {
			epm = true
			if r.InvoiceAmount != 300 {
				t.Errorf("EPM = %.2f, se esperaba 300", r.InvoiceAmount)
			}
		}
	}
	if !epm {
		t.Error("EPM no apareció y tenía valor")
	}
}
