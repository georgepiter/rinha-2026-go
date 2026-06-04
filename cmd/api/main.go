package main

import (
	"log"
	"rinha-2026/internal/handler"
	"rinha-2026/internal/service"
	"runtime"
	"runtime/debug"

	"github.com/valyala/fasthttp"
)

func main() {
	debug.SetMemoryLimit(150 * 1024 * 1024)
	debug.SetGCPercent(-1)
	runtime.GOMAXPROCS(1)

	scorer, err := service.NewFraudScorer()
	if err != nil {
		log.Fatal(err)
	}
	fraudHandler := handler.NewFraudHandler(scorer)

	log.Println("Server starting on port 9999")
	server := &fasthttp.Server{
		Handler: func(ctx *fasthttp.RequestCtx) {
			path := ctx.Path()
			if isFraudScorePath(path) {
				fraudHandler.FraudScore(ctx)
				return
			}
			if isReadyPath(path) {
				handler.Ready(ctx)
				return
			}
			ctx.SetStatusCode(fasthttp.StatusNotFound)
		},
		NoDefaultServerHeader:         true,
		NoDefaultDate:                 true,
		NoDefaultContentType:          true,
		MaxRequestBodySize:            2048,
		ReadBufferSize:                1024,
		WriteBufferSize:               512,
		MaxRequestsPerConn:            0,
		MaxIdleWorkerDuration:         0,
		MaxKeepaliveDuration:          0,
		DisableHeaderNamesNormalizing: true,
	}
	if err := server.ListenAndServe(":9999"); err != nil {
		log.Fatal(err)
	}
}

func isFraudScorePath(path []byte) bool {
	return len(path) == 12 &&
		path[0] == '/' &&
		path[1] == 'f' &&
		path[2] == 'r' &&
		path[3] == 'a' &&
		path[4] == 'u' &&
		path[5] == 'd' &&
		path[6] == '-' &&
		path[7] == 's' &&
		path[8] == 'c' &&
		path[9] == 'o' &&
		path[10] == 'r' &&
		path[11] == 'e'
}

func isReadyPath(path []byte) bool {
	return len(path) == 6 &&
		path[0] == '/' &&
		path[1] == 'r' &&
		path[2] == 'e' &&
		path[3] == 'a' &&
		path[4] == 'd' &&
		path[5] == 'y'
}
