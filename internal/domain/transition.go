package domain

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ActorKind — в каком качестве пользователь двигает сделку.
type ActorKind string

const (
	ActorSystem    ActorKind = "system" // планировщик
	ActorGuarantor ActorKind = "guarantor"
	ActorSeller    ActorKind = "seller"
	ActorBuyer     ActorKind = "buyer"
	ActorModerator ActorKind = "moderator"
)

// transitions — единственная таблица допустимых переходов: откуда → куда → кто может.
var transitions = map[string]map[string][]ActorKind{
	StatusCreated: {
		StatusWaitingMembers: {ActorGuarantor},
		StatusCancelled:      {ActorGuarantor, ActorSystem},
	},
	StatusWaitingMembers: {
		StatusWaitingPayment: {ActorSeller, ActorBuyer, ActorGuarantor},
		StatusCancelled:      {ActorGuarantor, ActorSystem},
	},
	StatusWaitingPayment: {
		StatusPaid:      {ActorGuarantor},
		StatusCancelled: {ActorGuarantor, ActorSystem},
		// реквизиты продавца совпали со скам-листом — сделка замораживается
		StatusDispute: {ActorModerator, ActorSystem},
	},
	StatusPaid: {
		StatusWaitingTickets: {ActorSystem, ActorGuarantor},
		StatusDispute:        {ActorSeller, ActorBuyer, ActorGuarantor, ActorModerator, ActorSystem},
	},
	StatusWaitingTickets: {
		StatusTicketsSent: {ActorBuyer},
		StatusDispute:     {ActorSeller, ActorBuyer, ActorGuarantor, ActorModerator, ActorSystem},
	},
	StatusTicketsSent: {
		StatusWaitingEntry: {ActorSystem, ActorGuarantor},
		StatusDispute:      {ActorSeller, ActorBuyer, ActorGuarantor, ActorModerator, ActorSystem},
	},
	StatusWaitingEntry: {
		StatusEntryOK: {ActorBuyer},
		StatusDispute: {ActorSeller, ActorBuyer, ActorGuarantor, ActorModerator, ActorSystem},
	},
	StatusEntryOK: {
		StatusPayoutPending: {ActorSystem, ActorGuarantor},
		StatusDispute:       {ActorSeller, ActorBuyer, ActorGuarantor, ActorModerator, ActorSystem},
	},
	StatusPayoutPending: {
		StatusClosed:        {ActorGuarantor},
		StatusResolved:      {ActorGuarantor}, // выплата по решению спора
		StatusRefundPending: {ActorGuarantor}, // частичное решение: сначала выплата, потом возврат
		StatusDispute:       {ActorModerator},
	},
	StatusDispute: {
		StatusPayoutPending: {ActorModerator},
		StatusRefundPending: {ActorModerator},
		StatusResolved:      {ActorModerator},
	},
	StatusRefundPending: {
		StatusResolved: {ActorGuarantor},
	},
}

// Actor — кто выполняет переход. nil означает планировщик.
type Actor struct {
	TgID int64
	Role string
}

var (
	ErrBadTransition   = errors.New("такой переход статуса недопустим")
	ErrForbidden       = errors.New("нет прав на этот переход")
	ErrAlreadyInStatus = errors.New("сделка уже в этом статусе")
	ErrDealNotFound    = errors.New("сделка не найдена")
)

// Beginner — всё, что умеет открыть транзакцию (пул pgx).
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Transition — единственный способ сменить статус сделки. Блокирует строку
// сделки, проверяет допустимость перехода и права актора, выполняет extra в той
// же транзакции и пишет deal_status_log.
func Transition(ctx context.Context, pool Beginner, dealID int64, to string, actor *Actor,
	comment string, extra func(context.Context, pgx.Tx, *Deal) error) (*Deal, error) {

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	deal, err := lockDeal(ctx, tx, dealID)
	if err != nil {
		return nil, err
	}
	if deal.Status == to {
		return deal, ErrAlreadyInStatus
	}
	allowed, ok := transitions[deal.Status][to]
	if !ok {
		return deal, fmt.Errorf("%w: %s → %s", ErrBadTransition, deal.Status, to)
	}
	if !hasKind(allowed, actorKinds(deal, actor)) {
		return deal, ErrForbidden
	}

	if extra != nil {
		if err := extra(ctx, tx, deal); err != nil {
			return deal, err
		}
	}

	var actorID *int64
	if actor != nil {
		actorID = &actor.TgID
	}
	if _, err := tx.Exec(ctx, `
		UPDATE deals SET status = $2, updated_at = now(),
			closed_at = CASE WHEN $3 THEN now() ELSE closed_at END
		WHERE id = $1`, dealID, to, Terminal(to)); err != nil {
		return deal, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO deal_status_log (deal_id, from_status, to_status, actor_id, comment)
		VALUES ($1, $2, $3, $4, nullif($5, ''))`,
		dealID, deal.Status, to, actorID, comment); err != nil {
		return deal, err
	}

	if err := tx.Commit(ctx); err != nil {
		return deal, err
	}
	deal.Status = to
	return deal, nil
}

// actorKinds — все качества, в которых актор может действовать по этой сделке.
func actorKinds(deal *Deal, actor *Actor) []ActorKind {
	if actor == nil {
		return []ActorKind{ActorSystem}
	}
	var kinds []ActorKind
	if actor.TgID == deal.GuarantorID {
		kinds = append(kinds, ActorGuarantor)
	}
	if actor.TgID != 0 && actor.TgID == deal.SellerID {
		kinds = append(kinds, ActorSeller)
	}
	if actor.TgID != 0 && actor.TgID == deal.BuyerID {
		kinds = append(kinds, ActorBuyer)
	}
	if AtLeast(actor.Role, RoleModerator) {
		kinds = append(kinds, ActorModerator)
	}
	return kinds
}

func hasKind(allowed, have []ActorKind) bool {
	for _, a := range allowed {
		for _, h := range have {
			if a == h {
				return true
			}
		}
	}
	return false
}
