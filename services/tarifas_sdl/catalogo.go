package tarifas_sdl

import "sort"

// Códigos del catálogo, ordenados una sola vez al cargar el paquete.
//
// El orden de iteración de un mapa en Go es aleatorio. En Cargos STR eso hizo
// que un encabezado con dos códigos se atribuyera a un operador distinto en cada
// corrida, y el bug no lo detectaba ningún test. Acá el orden fija además el
// orden de las filas que se guardan y se muestran.
var codigosOrdenados = func() []string {
	codes := make([]string, 0, len(orTipo))
	for code := range orTipo {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}()
