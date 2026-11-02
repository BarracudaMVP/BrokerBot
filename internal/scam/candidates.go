package scam

import "strings"

// Candidate — что в тексте похоже на реквизит и как это искать в скам-листе.
type Candidate struct {
	Type    string
	Value   string // уже нормализованное/хешированное значение
	Display string
}

// Длины цифровых последовательностей, по которым опознаём реквизиты.
const (
	minPhoneDigits = 10
	maxPhoneDigits = 15
	minCardDigits  = 13
	maxCardDigits  = 19
)

// separators — символы, которыми люди разбивают номера.
const separators = "  -+().\t"

// Candidates вытаскивает из текста всё, что похоже на карту или телефон.
// Одна и та же последовательность может дать и карту, и телефон (длины пересекаются) —
// это нормально: лучше проверить дважды, чем пропустить скамера.
func Candidates(secret []byte, text string) []Candidate {
	var out []Candidate
	for _, run := range numberRuns(text) {
		n := len(run)
		if n >= minPhoneDigits && n <= maxPhoneDigits {
			out = append(out, Candidate{Type: TypePhone, Value: NormalizePhone(run), Display: run})
		}
		if n >= minCardDigits && n <= maxCardDigits {
			hash, display := HashCard(secret, run)
			out = append(out, Candidate{Type: TypeCard, Value: hash, Display: display})
		}
	}
	return out
}

// numberRuns возвращает кандидатов в номера: каждую цифровую последовательность и
// склейку последовательностей, разделённых только разделителями.
// «4276 1600 1234 5678» и «+7 912 345-67-89» склеиваются, а «2 карты 4276…» — нет,
// потому что между цифрами стоят буквы.
func numberRuns(text string) []string {
	var out []string
	var run strings.Builder
	var group []string

	closeRun := func() {
		if run.Len() > 0 {
			group = append(group, run.String())
			run.Reset()
		}
	}
	flush := func() {
		closeRun()
		out = append(out, group...)
		if len(group) > 1 {
			out = append(out, strings.Join(group, ""))
		}
		group = nil
	}

	for _, r := range text {
		switch {
		case r >= '0' && r <= '9':
			run.WriteByte(byte(r))
		case strings.ContainsRune(separators, r):
			closeRun()
		default:
			flush()
		}
	}
	flush()
	return out
}
