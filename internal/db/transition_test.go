package db

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"brokerbot/internal/domain"
)

// Участники тестовой сделки.
const (
	guarantorID = 1001
	sellerID    = 1002
	buyerID     = 1003
	moderID     = 1004
	strangerID  = 1005
)

func testDB(t *testing.T) *DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL не задан")
	}
	ctx := context.Background()
	if err := Migrate(ctx, url); err != nil {
		t.Fatalf("миграции: %v", err)
	}
	d, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("подключение: %v", err)
	}
	t.Cleanup(d.Close)

	if _, err := d.Exec(ctx, `TRUNCATE deals, users, deal_status_log, disputes,
		deal_files, message_log, username_history, conversation_state CASCADE`); err != nil {
		t.Fatalf("очистка: %v", err)
	}
	for id, role := range map[int64]string{
		guarantorID: domain.RoleGuarantor, sellerID: domain.RoleUser,
		buyerID: domain.RoleUser, moderID: domain.RoleModerator, strangerID: domain.RoleUser,
	} {
		if _, err := d.TouchUser(ctx, id, "", "u", ""); err != nil {
			t.Fatal(err)
		}
		if err := d.SetRole(ctx, id, role); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

// newDeal создаёт сделку с вступившими сторонами, как после подтверждения.
func newDeal(t *testing.T, d *DB, eventAt time.Time) *domain.Deal {
	t.Helper()
	ctx := context.Background()

	commission, buyerPays, sellerReceives := domain.Commission(1000000, 750, domain.PayerBuyer)
	deal, err := d.CreateDeal(ctx, &domain.Deal{
		ChatID: -100500, GuarantorID: guarantorID, EventName: "Концерт", EventAt: eventAt,
		TicketsCount: 2, Currency: domain.DefaultCurrency, Amount: 1000000,
		CommissionPct: 750, CommissionPayer: domain.PayerBuyer,
		CommissionAmount: commission, BuyerPays: buyerPays, SellerReceives: sellerReceives,
	})
	if err != nil {
		t.Fatalf("создание сделки: %v", err)
	}
	for role, id := range map[string]int64{domain.PayerSeller: sellerID, domain.PayerBuyer: buyerID} {
		ok, err := d.ClaimParty(ctx, deal.ID, role, id)
		if err != nil || !ok {
			t.Fatalf("занять роль %s: %v (ok=%v)", role, err, ok)
		}
	}
	deal.SellerID, deal.BuyerID = sellerID, buyerID
	return deal
}

func user(id int64, role string) *domain.Actor { return &domain.Actor{TgID: id, Role: role} }

// TestTransitionHappyPath проходит сделку от создания до закрытия и проверяет,
// что чужие переходы отбиваются.
func TestTransitionHappyPath(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	// Мероприятие в прошлом, чтобы проход можно было подтвердить.
	deal := newDeal(t, d, time.Now().Add(-time.Hour))

	step := func(to string, actor *domain.Actor) {
		t.Helper()
		if _, err := domain.Transition(ctx, d, deal.ID, to, actor, "", nil); err != nil {
			t.Fatalf("переход в %s: %v", to, err)
		}
	}
	denied := func(to string, actor *domain.Actor, want error) {
		t.Helper()
		if _, err := domain.Transition(ctx, d, deal.ID, to, actor, "", nil); !errors.Is(err, want) {
			t.Fatalf("переход в %s должен был дать %v, дал %v", to, want, err)
		}
	}

	denied(domain.StatusWaitingMembers, user(strangerID, domain.RoleUser), domain.ErrForbidden)
	step(domain.StatusWaitingMembers, user(guarantorID, domain.RoleGuarantor))

	// Прыжок через этапы недопустим.
	denied(domain.StatusPayoutPending, user(guarantorID, domain.RoleGuarantor), domain.ErrBadTransition)

	step(domain.StatusWaitingPayment, user(buyerID, domain.RoleUser))
	// Оплату фиксирует только гарант.
	denied(domain.StatusPaid, user(buyerID, domain.RoleUser), domain.ErrForbidden)
	step(domain.StatusPaid, user(guarantorID, domain.RoleGuarantor))

	step(domain.StatusWaitingTickets, nil) // планировщик
	// Билеты получил — только покупатель.
	denied(domain.StatusTicketsSent, user(sellerID, domain.RoleUser), domain.ErrForbidden)
	step(domain.StatusTicketsSent, user(buyerID, domain.RoleUser))

	step(domain.StatusWaitingEntry, nil)
	denied(domain.StatusEntryOK, user(guarantorID, domain.RoleGuarantor), domain.ErrForbidden)
	step(domain.StatusEntryOK, user(buyerID, domain.RoleUser))

	step(domain.StatusPayoutPending, nil)
	denied(domain.StatusClosed, user(buyerID, domain.RoleUser), domain.ErrForbidden)
	step(domain.StatusClosed, user(guarantorID, domain.RoleGuarantor))

	// Повторное нажатие кнопки ничего не ломает.
	denied(domain.StatusClosed, user(guarantorID, domain.RoleGuarantor), domain.ErrAlreadyInStatus)
	// Из терминального статуса выхода нет.
	denied(domain.StatusDispute, user(moderID, domain.RoleModerator), domain.ErrBadTransition)

	closed, err := d.GetDeal(ctx, deal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if closed.Status != domain.StatusClosed {
		t.Errorf("статус = %s", closed.Status)
	}
	// Комиссия 7,5% с 10 000,00 = 750,00; покупатель платит 10 750,00.
	if closed.CommissionAmount != 75000 || closed.BuyerPays != 1075000 {
		t.Errorf("суммы поехали: комиссия %d, к оплате %d",
			closed.CommissionAmount, closed.BuyerPays)
	}

	var logged int
	var closedAt *time.Time
	if err := d.QueryRow(ctx, `SELECT count(*) FROM deal_status_log WHERE deal_id = $1`,
		deal.ID).Scan(&logged); err != nil {
		t.Fatal(err)
	}
	if logged != 10 { // CREATED + 9 переходов
		t.Errorf("в логе статусов %d записей, ожидали 10", logged)
	}
	if err := d.QueryRow(ctx, `SELECT closed_at FROM deals WHERE id = $1`,
		deal.ID).Scan(&closedAt); err != nil {
		t.Fatal(err)
	}
	if closedAt == nil {
		t.Error("closed_at не заполнен")
	}
}

// TestTransitionExtraRollback: если побочное действие упало, статус не меняется.
func TestTransitionExtraRollback(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	deal := newDeal(t, d, time.Now().Add(time.Hour))

	boom := errors.New("не вышло")
	_, err := domain.Transition(ctx, d, deal.ID, domain.StatusWaitingMembers,
		user(guarantorID, domain.RoleGuarantor), "",
		func(context.Context, pgx.Tx, *domain.Deal) error { return boom })
	if !errors.Is(err, boom) {
		t.Fatalf("ожидали ошибку побочного действия, получили %v", err)
	}

	after, err := d.GetDeal(ctx, deal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != domain.StatusCreated {
		t.Errorf("статус изменился при откате: %s", after.Status)
	}
}

// TestDisputeFlow: спор открывается в той же транзакции, что и переход, и не дублируется.
func TestDisputeFlow(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	deal := newDeal(t, d, time.Now().Add(time.Hour))

	for _, to := range []string{domain.StatusWaitingMembers, domain.StatusWaitingPayment, domain.StatusPaid} {
		actor := user(guarantorID, domain.RoleGuarantor)
		if _, err := domain.Transition(ctx, d, deal.ID, to, actor, "", nil); err != nil {
			t.Fatalf("переход в %s: %v", to, err)
		}
	}

	opener := int64(buyerID)
	open := func(ctx context.Context, tx pgx.Tx, dl *domain.Deal) error {
		return OpenDisputeTx(ctx, tx, dl.ID, &opener, "не пустили")
	}
	if _, err := domain.Transition(ctx, d, deal.ID, domain.StatusDispute,
		user(buyerID, domain.RoleUser), "не пустили", open); err != nil {
		t.Fatalf("открытие спора: %v", err)
	}

	if has, err := d.HasDispute(ctx, deal.ID); err != nil || !has {
		t.Fatalf("спор не записан: %v", err)
	}
	// Второй открытый спор по той же сделке не создаётся.
	tx, err := d.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := OpenDisputeTx(ctx, tx, deal.ID, &opener, "ещё раз"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := d.QueryRow(ctx, `SELECT count(*) FROM disputes WHERE deal_id = $1`,
		deal.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("споров по сделке %d, ожидали 1", n)
	}

	// Решение модератора: выплата продавцу, затем закрытие как RESOLVED.
	if _, err := domain.Transition(ctx, d, deal.ID, domain.StatusPayoutPending,
		user(moderID, domain.RoleModerator), "решено в пользу продавца", nil); err != nil {
		t.Fatalf("решение спора: %v", err)
	}
	if _, err := domain.Transition(ctx, d, deal.ID, domain.StatusResolved,
		user(guarantorID, domain.RoleGuarantor), "выплачено", nil); err != nil {
		t.Fatalf("закрытие после спора: %v", err)
	}
}
