package domain

// Кто платит комиссию.
const (
	PayerBuyer  = "buyer"
	PayerSeller = "seller"
)

// Commission считает комиссию и итоговые суммы.
// pct — сотые доли процента (750 = 7,5%). Округление комиссии — до ближайшей
// минимальной единицы, половина вверх.
func Commission(amount, pct int64, payer string) (commission, buyerPays, sellerReceives int64) {
	commission = (amount*pct + 5000) / 10000
	if payer == PayerBuyer {
		return commission, amount + commission, amount
	}
	return commission, amount, amount - commission
}
