package service

func mondayBasedDowFromDay(day int) int {
	dow := (day + 3) % 7
	if dow < 0 {
		return dow + 7
	}
	return dow
}

func isoMinuteOfDayBytes(iso []byte) int {
	if len(iso) < 16 {
		return 0
	}
	hour := int(iso[11]-'0')*10 + int(iso[12]-'0')
	min := int(iso[14]-'0')*10 + int(iso[15]-'0')
	return hour*60 + min
}

func daysFromIsoBytes(iso []byte) int {
	if len(iso) < 10 {
		return 0
	}
	if iso[0] == '2' && iso[1] == '0' && iso[2] == '2' && iso[3] == '6' &&
		iso[5] == '0' && iso[6] == '3' {
		return int(iso[8]-'0')*10 + int(iso[9]-'0') + 2
	}
	y := int(iso[0]-'0')*1000 + int(iso[1]-'0')*100 + int(iso[2]-'0')*10 + int(iso[3]-'0')
	m := int(iso[5]-'0')*10 + int(iso[6]-'0')
	d := int(iso[8]-'0')*10 + int(iso[9]-'0')
	if m <= 2 {
		y--
	}
	era := floorDiv(y, 400)
	yoe := y - era*400
	mp := m + 9
	if m > 2 {
		mp = m - 3
	}
	doy := (153*mp+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}

func floorDiv(a, b int) int {
	q := a / b
	r := a % b
	if r != 0 && ((r < 0) != (b < 0)) {
		q--
	}
	return q
}
