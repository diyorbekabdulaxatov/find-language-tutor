package disputes

import (
	"context"

	"github.com/google/uuid"
)

// Refunder is the slice of the booking/payment flow this module needs when an
// operator resolves a dispute in the student's favour with `refund: true`. It
// refunds the booking's payment WITHOUT touching the booking's lifecycle — the
// lesson stays completed / confirmed, only the money moves back.
//
// Same rule as every other cross-module dependency here: the consuming module
// (this one) defines the port, *bookings.Service satisfies it, and cmd/api wires
// them together with Service.SetRefunder. A nil Refunder makes `refund: true` a
// logged no-op, so tests and a cmd/api without payments keep working.
type Refunder interface {
	// AdminRefund releases / refunds the booking's payment. A booking whose
	// intent was never authorized is a no-op void.
	AdminRefund(ctx context.Context, bookingID uuid.UUID) error
}
