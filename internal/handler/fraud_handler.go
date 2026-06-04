package handler

import (
	"rinha-2026/internal/service"

	"github.com/valyala/fasthttp"
)

var (
	scoreBodies = [6][]byte{
		[]byte(`{"approved":true,"fraud_score":0}`),
		[]byte(`{"approved":true,"fraud_score":0.2}`),
		[]byte(`{"approved":true,"fraud_score":0.4}`),
		[]byte(`{"approved":false,"fraud_score":0.6}`),
		[]byte(`{"approved":false,"fraud_score":0.8}`),
		[]byte(`{"approved":false,"fraud_score":1}`),
	}
	jsonType = []byte(`application/json`)
)

type FraudHandler struct {
	scorer *service.FraudScorer
}

func NewFraudHandler(scorer *service.FraudScorer) *FraudHandler {
	return &FraudHandler{scorer: scorer}
}

func Ready(ctx *fasthttp.RequestCtx) {
	ctx.SetStatusCode(fasthttp.StatusOK)
}

func (h *FraudHandler) FraudScore(ctx *fasthttp.RequestCtx) {
	fraudCount, ok := h.scorer.ScoreBody(ctx.PostBody())
	if !ok {
		ctx.Error("invalid request", fasthttp.StatusBadRequest)
		return
	}

	writeFraudResponse(ctx, fraudCount)
}

func writeFraudResponse(ctx *fasthttp.RequestCtx, fraudCount int) {
	ctx.SetContentTypeBytes(jsonType)
	ctx.SetStatusCode(fasthttp.StatusOK)
	if fraudCount < 0 {
		fraudCount = 0
	} else if fraudCount > 5 {
		fraudCount = 5
	}
	ctx.Response.SetBodyRaw(scoreBodies[fraudCount])
}
