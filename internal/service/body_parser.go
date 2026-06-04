package service

import (
	"bytes"
	"math"
)

var (
	keyTransaction     = []byte(`"transaction":`)
	keyCustomer        = []byte(`"customer":`)
	keyMerchant        = []byte(`"merchant":`)
	keyTerminal        = []byte(`"terminal":`)
	keyLastTransaction = []byte(`"last_transaction":`)
	keyAmount          = []byte(`"amount":`)
	keyInstallments    = []byte(`"installments":`)
	keyRequestedAt     = []byte(`"requested_at":`)
	keyAvgAmount       = []byte(`"avg_amount":`)
	keyTxCount24h      = []byte(`"tx_count_24h":`)
	keyKnownMerchants  = []byte(`"known_merchants":`)
	keyID              = []byte(`"id":`)
	keyMCC             = []byte(`"mcc":`)
	keyIsOnline        = []byte(`"is_online":`)
	keyCardPresent     = []byte(`"card_present":`)
	keyKmFromHome      = []byte(`"km_from_home":`)
	keyTimestamp       = []byte(`"timestamp":`)
	keyKmFromCurrent   = []byte(`"km_from_current":`)
	literalNull        = []byte(`null`)
	literalTrue        = []byte(`true`)
	literalFalse       = []byte(`false`)
)

type parsedTransaction struct {
	requestedAt     []byte
	lastTimestamp   []byte
	mcc             []byte
	amount          float64
	customerAvg     float64
	kmLast          float64
	kmFromHome      float64
	merchantAvg     float64
	installments    int
	txCount24h      int
	hasLast         bool
	isOnline        bool
	cardPresent     bool
	merchantUnknown bool
}

func (s *FraudScorer) ScoreBody(body []byte) (int, bool) {
	var req parsedTransaction
	if !parseTransactionBody(body, &req) {
		return 0, false
	}
	score := scoreParsedZ(&req)
	if score < decisionZThreshold {
		return 0, true
	}
	if score > decisionZThreshold+hybridDecisionMargin {
		return 5, true
	}
	return s.index.FraudCount(vectorizeParsed(&req)), true
}

func scoreParsedZ(req *parsedTransaction) float64 {
	requestedDay := daysFromIsoBytes(req.requestedAt)
	requestedMinuteOfDay := isoMinuteOfDayBytes(req.requestedAt)
	amount := modelFloat(req.amount)
	customerAvg := modelFloat(req.customerAvg)
	if customerAvg <= 0 {
		customerAvg = 0.01
	}

	kmLast := 0.0
	minutes := -1.0
	if req.hasLast {
		kmLast = modelFloat(req.kmLast)
		minutes = float64((requestedDay-daysFromIsoBytes(req.lastTimestamp))*1440+requestedMinuteOfDay-isoMinuteOfDayBytes(req.lastTimestamp)) * invMaxMinutes
	}

	kmFromHome := modelFloat(req.kmFromHome)
	merchantAvg := modelFloat(req.merchantAvg)

	z := weights[0]
	z += weights[1] * clamp(amount*invMaxAmount)
	z += weights[2] * clamp(float64(req.installments)*invMaxInstallments)
	z += weights[3] * clamp((amount/customerAvg)*invAmountVsAvgRatio)
	z += weights[4] * (float64(requestedMinuteOfDay/60) * invMaxHour)
	z += weights[5] * (float64(mondayBasedDowFromDay(requestedDay)) * invMaxDow)
	if req.hasLast {
		z += weights[6] * clamp(minutes)
		z += weights[7] * clamp(kmLast*invMaxKm)
	} else {
		z -= weights[6]
		z -= weights[7]
	}
	z += weights[8] * clamp(kmFromHome*invMaxKm)
	z += weights[9] * clamp(float64(req.txCount24h)*invMaxTxCount24h)
	if req.isOnline {
		z += weights[10]
	}
	if req.cardPresent {
		z += weights[11]
	}
	if req.merchantUnknown {
		z += weights[12]
	}
	z += weights[13] * mccRiskValueBytes(req.mcc)
	z += weights[14] * clamp(merchantAvg*invMaxMerchantAvgAmount)
	if req.hasLast {
		z += weights[15]
	}
	z += weights[16] * (math.Log1p(amount/customerAvg) * invLog101)
	z += weights[17] * (math.Log1p(kmFromHome) * invLog1001)
	z += weights[18] * (math.Log1p(kmLast) * invLog1001)

	return z
}

func vectorizeParsed(req *parsedTransaction) [14]int16 {
	requestedDay := daysFromIsoBytes(req.requestedAt)
	requestedMinuteOfDay := isoMinuteOfDayBytes(req.requestedAt)
	amount := req.amount
	customerAvg := req.customerAvg
	if customerAvg <= 0 {
		customerAvg = 0.01
	}

	kmLast := 0.0
	minutes := -1.0
	if req.hasLast {
		kmLast = req.kmLast
		minutes = float64((requestedDay-daysFromIsoBytes(req.lastTimestamp))*1440+requestedMinuteOfDay-isoMinuteOfDayBytes(req.lastTimestamp)) * invMaxMinutes
	}

	var vector [14]int16
	vector[0] = quantize(amount * invMaxAmount)
	vector[1] = quantize(float64(req.installments) * invMaxInstallments)
	vector[2] = quantize((amount / customerAvg) * invAmountVsAvgRatio)
	vector[3] = quantize(float64(requestedMinuteOfDay/60) * invMaxHour)
	vector[4] = quantize(float64(mondayBasedDowFromDay(requestedDay)) * invMaxDow)
	if req.hasLast {
		vector[5] = quantize(minutes)
		vector[6] = quantize(kmLast * invMaxKm)
	} else {
		vector[5] = -int16(vectorScale)
		vector[6] = -int16(vectorScale)
	}
	vector[7] = quantize(req.kmFromHome * invMaxKm)
	vector[8] = quantize(float64(req.txCount24h) * invMaxTxCount24h)
	if req.isOnline {
		vector[9] = int16(vectorScale)
	}
	if req.cardPresent {
		vector[10] = int16(vectorScale)
	}
	if req.merchantUnknown {
		vector[11] = int16(vectorScale)
	}
	vector[12] = quantize(mccRiskValueBytes(req.mcc))
	vector[13] = quantize(req.merchantAvg * invMaxMerchantAvgAmount)

	return vector
}

func parseTransactionBody(body []byte, req *parsedTransaction) bool {
	cursor, ok := indexAfter(body, keyTransaction, 0)
	if !ok {
		return false
	}
	if req.amount, ok = numberAfterCursor(body, keyAmount, &cursor); !ok {
		return false
	}
	if req.installments, ok = intAfterCursor(body, keyInstallments, &cursor); !ok {
		return false
	}
	if req.requestedAt, ok = stringAfterCursor(body, keyRequestedAt, &cursor); !ok {
		return false
	}

	if cursor, ok = indexAfter(body, keyCustomer, cursor); !ok {
		return false
	}
	if req.customerAvg, ok = numberAfterCursor(body, keyAvgAmount, &cursor); !ok {
		return false
	}
	if req.txCount24h, ok = intAfterCursor(body, keyTxCount24h, &cursor); !ok {
		return false
	}
	knownMerchants, ok := arrayAfterCursor(body, keyKnownMerchants, &cursor)
	if !ok {
		return false
	}

	if cursor, ok = indexAfter(body, keyMerchant, cursor); !ok {
		return false
	}
	merchantID, ok := stringAfterCursor(body, keyID, &cursor)
	if !ok {
		return false
	}
	if req.mcc, ok = stringAfterCursor(body, keyMCC, &cursor); !ok {
		return false
	}
	if req.merchantAvg, ok = numberAfterCursor(body, keyAvgAmount, &cursor); !ok {
		return false
	}
	req.merchantUnknown = !stringArrayContains(knownMerchants, merchantID)

	if cursor, ok = indexAfter(body, keyTerminal, cursor); !ok {
		return false
	}
	if req.isOnline, ok = boolAfterCursor(body, keyIsOnline, &cursor); !ok {
		return false
	}
	if req.cardPresent, ok = boolAfterCursor(body, keyCardPresent, &cursor); !ok {
		return false
	}
	if req.kmFromHome, ok = numberAfterCursor(body, keyKmFromHome, &cursor); !ok {
		return false
	}

	cursor, ok = indexAfter(body, keyLastTransaction, cursor)
	if !ok {
		return false
	}
	if len(body)-cursor >= len(literalNull) && bytes.Equal(body[cursor:cursor+len(literalNull)], literalNull) {
		req.hasLast = false
		return true
	}
	req.hasLast = true
	if req.lastTimestamp, ok = stringAfterCursor(body, keyTimestamp, &cursor); !ok {
		return false
	}
	if req.kmLast, ok = numberAfterCursor(body, keyKmFromCurrent, &cursor); !ok {
		return false
	}

	return true
}

func indexAfter(body, key []byte, cursor int) (int, bool) {
	if cursor < 0 || cursor >= len(body) {
		return 0, false
	}
	idx := bytes.Index(body[cursor:], key)
	if idx < 0 {
		return 0, false
	}
	return skipSpaces(body, cursor+idx+len(key)), true
}

func numberAfterCursor(body, key []byte, cursor *int) (float64, bool) {
	i, ok := indexAfter(body, key, *cursor)
	if !ok {
		return 0, false
	}
	*cursor = i
	return parseNumber(body, i)
}

func intAfterCursor(body, key []byte, cursor *int) (int, bool) {
	i, ok := indexAfter(body, key, *cursor)
	if !ok {
		return 0, false
	}
	*cursor = i
	return parseInt(body, i)
}

func boolAfterCursor(body, key []byte, cursor *int) (bool, bool) {
	i, ok := indexAfter(body, key, *cursor)
	if !ok {
		return false, false
	}
	*cursor = i
	if len(body)-i >= len(literalTrue) && bytes.Equal(body[i:i+len(literalTrue)], literalTrue) {
		return true, true
	}
	if len(body)-i >= len(literalFalse) && bytes.Equal(body[i:i+len(literalFalse)], literalFalse) {
		return false, true
	}
	return false, false
}

func stringAfterCursor(body, key []byte, cursor *int) ([]byte, bool) {
	i, ok := indexAfter(body, key, *cursor)
	if !ok || i >= len(body) || body[i] != '"' {
		return nil, false
	}
	start := i + 1
	for i = start; i < len(body); i++ {
		if body[i] == '"' {
			*cursor = i + 1
			return body[start:i], true
		}
	}
	return nil, false
}

func arrayAfterCursor(body, key []byte, cursor *int) ([]byte, bool) {
	i, ok := indexAfter(body, key, *cursor)
	if !ok || i >= len(body) || body[i] != '[' {
		return nil, false
	}
	start := i + 1
	for i = start; i < len(body); i++ {
		if body[i] == ']' {
			*cursor = i + 1
			return body[start:i], true
		}
	}
	return nil, false
}

func parseNumber(body []byte, i int) (float64, bool) {
	if i >= len(body) {
		return 0, false
	}
	sign := 1.0
	if body[i] == '-' {
		sign = -1.0
		i++
	}
	value := 0.0
	digits := 0
	for i < len(body) && body[i] >= '0' && body[i] <= '9' {
		value = value*10 + float64(body[i]-'0')
		i++
		digits++
	}
	if i < len(body) && body[i] == '.' {
		i++
		scale := 0.1
		for i < len(body) && body[i] >= '0' && body[i] <= '9' {
			value += float64(body[i]-'0') * scale
			scale *= 0.1
			i++
			digits++
		}
	}
	if digits == 0 {
		return 0, false
	}
	return sign * value, true
}

func parseInt(body []byte, i int) (int, bool) {
	if i >= len(body) {
		return 0, false
	}
	value := 0
	digits := 0
	for i < len(body) && body[i] >= '0' && body[i] <= '9' {
		value = value*10 + int(body[i]-'0')
		i++
		digits++
	}
	return value, digits > 0
}

func stringArrayContains(body, target []byte) bool {
	for i := 0; i < len(body); i++ {
		if body[i] != '"' {
			continue
		}
		start := i + 1
		for i = start; i < len(body); i++ {
			if body[i] == '"' {
				if bytes.Equal(body[start:i], target) {
					return true
				}
				break
			}
		}
	}
	return false
}

func mccRiskValueBytes(mcc []byte) float64 {
	if len(mcc) != 4 {
		return 0.50
	}
	switch {
	case mcc[0] == '5' && mcc[1] == '4' && mcc[2] == '1' && mcc[3] == '1':
		return 0.15
	case mcc[0] == '5' && mcc[1] == '8' && mcc[2] == '1' && mcc[3] == '2':
		return 0.30
	case mcc[0] == '5' && mcc[1] == '9' && mcc[2] == '1' && mcc[3] == '2':
		return 0.20
	case mcc[0] == '5' && mcc[1] == '9' && mcc[2] == '4' && mcc[3] == '4':
		return 0.45
	case mcc[0] == '7' && mcc[1] == '8' && mcc[2] == '0' && mcc[3] == '1':
		return 0.80
	case mcc[0] == '7' && mcc[1] == '8' && mcc[2] == '0' && mcc[3] == '2':
		return 0.75
	case mcc[0] == '7' && mcc[1] == '9' && mcc[2] == '9' && mcc[3] == '5':
		return 0.85
	case mcc[0] == '4' && mcc[1] == '5' && mcc[2] == '1' && mcc[3] == '1':
		return 0.35
	case mcc[0] == '5' && mcc[1] == '3' && mcc[2] == '1' && mcc[3] == '1':
		return 0.25
	case mcc[0] == '5' && mcc[1] == '9' && mcc[2] == '9' && mcc[3] == '9':
		return 0.50
	default:
		return 0.50
	}
}

func skipSpaces(body []byte, i int) int {
	for i < len(body) {
		switch body[i] {
		case ' ', '\n', '\r', '\t':
			i++
		default:
			return i
		}
	}
	return i
}

func modelFloat(value float64) float64 {
	return float64(float32(value))
}
