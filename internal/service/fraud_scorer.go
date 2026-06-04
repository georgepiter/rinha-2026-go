package service

import (
	"math"
	"os"
)

const defaultIndexPath = "index.bin"

type FraudScorer struct {
	index *VectorIndex
}

func NewFraudScorer() (*FraudScorer, error) {
	path := os.Getenv("INDEX_PATH")
	if path == "" {
		path = defaultIndexPath
	}
	index, err := LoadVectorIndex(path)
	if err != nil {
		return nil, err
	}
	return &FraudScorer{index: index}, nil
}

const (
	decisionZThreshold      = -1.92774846938101
	hybridDecisionMargin    = 6.0
	vectorScale             = 10000.0
	invMaxAmount            = 1.0 / 10000.0
	invMaxInstallments      = 1.0 / 12.0
	invAmountVsAvgRatio     = 1.0 / 10.0
	invMaxMinutes           = 1.0 / 1440.0
	invMaxKm                = 1.0 / 1000.0
	invMaxTxCount24h        = 1.0 / 20.0
	invMaxMerchantAvgAmount = 1.0 / 10000.0
	invMaxHour              = 1.0 / 23.0
	invMaxDow               = 1.0 / 6.0
)

var weights = [...]float64{
	-8.523164709, 1.863921269, 3.094092674, 0.5529062283, -0.2422058805,
	-0.06415558462, -2.511498622, 0.7589419852, 1.524335692, 3.462601206,
	-0.01745284273, -0.208227002, 0.5689366744, 0.9938504886, -22.91874404,
	-0.1604114719, 2.358111607, 4.211645929, 3.049510263,
}

var (
	invLog101  = 1.0 / math.Log(101.0)
	invLog1001 = 1.0 / math.Log(1001.0)
)

func clamp(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

func quantize(x float64) int16 {
	return int16(math.Round(clamp(x) * vectorScale))
}
