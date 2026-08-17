package models_test

import (
	"testing"

	"bia-bills/models"
)

// ⚠️ TRASVASE: archivo con prefijo `liquidations_`. Ver docs/backend/migracion-a-go.md.
//
// Un solo test, pero cubre el dato más frágil del módulo: si TableName cambiara,
// GORM escribiría en otra tabla —o crearía una— sin que nada falle. Y las dos
// tablas se llaman casi igual (inputs / charges) y viven en bases distintas, así
// que confundirlas es fácil y el error no se ve hasta buscar los datos.
func TestTableNames(t *testing.T) {
	if got := (models.LiquidationsStrInput{}).TableName(); got != "public.liquidations_str_inputs" {
		t.Errorf("tabla de insumos = %q", got)
	}
	if got := (models.LiquidationsStrCharge{}).TableName(); got != "public.liquidations_str_charges" {
		t.Errorf("tabla de resultados = %q", got)
	}
}

// Los ajustes son punteros a propósito: NULL es "ese archivo de refactura no vino
// en el lote" y 0 es "vino y el operador tenía cero". Si alguien los cambiara a
// float64, los dos casos quedarían indistinguibles y no habría forma de saber si
// falta un archivo.
func TestAjustesSonPunterosParaDistinguirNullDeCero(t *testing.T) {
	cero := 0.0
	fila := models.LiquidationsStrInput{Reinvoice1Amount: &cero}

	if fila.Reinvoice1Amount == nil || *fila.Reinvoice1Amount != 0 {
		t.Error("un ajuste en cero tiene que poder guardarse como cero")
	}
	if fila.Reinvoice2Amount != nil {
		t.Error("un ajuste que no vino tiene que quedar en nil, no en cero")
	}
}
