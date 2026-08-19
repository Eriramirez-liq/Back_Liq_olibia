// Command liquidations-dev levanta SOLO el módulo de Liquidaciones, para probarlo
// local sin arrastrar todo bia-bills.
//
// ⚠️ NO SE TRASVASA. En bia-bills el entrypoint es su `main.go` de la raíz, que
// arma el router completo del servicio; ahí el módulo se registra con una línea
// (RegisterLiquidations). Este archivo vive en cmd/ —ruta que su repo no tiene—
// justamente para no pisarle nada y para que quede claro que es un arnés de
// desarrollo, no parte del servicio.
//
// Uso:
//
//	liq_db_host=... liq_db_user=... liq_db_password=... \
//	  go run ./cmd/liquidations-dev
//
// Las rutas quedan igual que en producción: /ms-bill/liquidations/...
//
// ── Y todo lo demás se reenvía a dev ────────────────────────────────────────
// El front manda TODO su tráfico de /ms-bill a una sola URL, así que apuntarlo
// acá le cortaba también los permisos y el selector de equipo quedaba con todo
// bloqueado. Por eso este arnés hace de intermediario: atiende lo de
// Liquidaciones y reenvía el resto al gateway de desarrollo.
//
// Así el front funciona completo en local —permisos, Facturación, todo— y solo
// las rutas del módulo que se está desarrollando salen de acá.
package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"bia-bills/router"

	"github.com/gin-gonic/gin"
)

// Gateway de desarrollo al que se reenvía lo que este arnés no atiende.
const upstreamPorDefecto = "https://olibia.dev.bia.app"

func main() {
	// Variable propia y no PORT: el .env del repo trae PORT=5001 de la etapa Flask
	// y arrancó ahí sin que nadie se lo pidiera. Un nombre específico evita esa
	// clase de sorpresa.
	puerto := os.Getenv("LIQ_DEV_PORT")
	if puerto == "" {
		puerto = "8081" // 8080 lo usa bia-bills local; 4000 el backend TypeScript
	}

	gin.SetMode(gin.ReleaseMode)
	motor := gin.New()
	motor.Use(gin.Logger(), gin.Recovery())

	// Mismo prefijo que el servicio real, para que las URLs de la prueba sean las
	// mismas que va a ver el front.
	apiPrefix := motor.Group("/ms-bill")
	router.RegisterLiquidations(apiPrefix)

	// Todo lo que no sea de Liquidaciones va al gateway de desarrollo.
	//
	// Se puede apagar con LIQ_DEV_UPSTREAM=off si se quiere el arnés aislado.
	upstream := os.Getenv("LIQ_DEV_UPSTREAM")
	if upstream == "" {
		upstream = upstreamPorDefecto
	}
	if strings.EqualFold(upstream, "off") {
		log.Printf("liquidations-dev: sin reenvío. Todo lo que no sea de Liquidaciones va a dar 404.")
	} else {
		reenvio, err := reenvioA(upstream)
		if err != nil {
			log.Fatalf("liquidations-dev: upstream inválido %q: %v", upstream, err)
		}
		motor.NoRoute(reenvio)
		log.Printf("liquidations-dev: lo que no sea de Liquidaciones se reenvía a %s", upstream)
	}

	log.Printf("liquidations-dev escuchando en :%s — probá /ms-bill/liquidations/health", puerto)
	if err := motor.Run(":" + puerto); err != nil {
		log.Fatal(err)
	}
}

// reenvioA arma el handler que manda la request al gateway de desarrollo.
//
// Se reescribe el Host porque el gateway rutea por ese header: sin eso responde
// 404 aunque la ruta exista. Los demás headers pasan tal cual, incluido el
// Authorization que el proxy del front inyecta desde la cookie.
func reenvioA(destino string) (gin.HandlerFunc, error) {
	objetivo, err := url.Parse(destino)
	if err != nil {
		return nil, err
	}
	if objetivo.Scheme == "" || objetivo.Host == "" {
		return nil, http.ErrMissingFile
	}

	proxy := httputil.NewSingleHostReverseProxy(objetivo)

	base := proxy.Director
	proxy.Director = func(r *http.Request) {
		base(r)
		r.Host = objetivo.Host
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("liquidations-dev: falló el reenvío de %s %s: %v", r.Method, r.URL.Path, err)
		w.WriteHeader(http.StatusBadGateway)
	}

	return func(c *gin.Context) {
		proxy.ServeHTTP(c.Writer, c.Request)
	}, nil
}
