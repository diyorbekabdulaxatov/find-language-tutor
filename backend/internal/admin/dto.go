package admin

import (
	"time"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/teachers"
)

// Wire DTOs — the source of truth for the JSON shape, in sync with openapi.yaml
// (snake_case, RFC3339 UTC, money in integer minor units).

// --- metrics ---

type metricsDTO struct {
	UsersTotal       int64  `json:"users_total"`
	TeachersTotal    int64  `json:"teachers_total"`
	TeachersPending  int64  `json:"teachers_pending"`
	BookingsTotal    int64  `json:"bookings_total"`
	BookingsThisWeek int64  `json:"bookings_this_week"`
	GMVMinor         int64  `json:"gmv_minor"`
	GMVCurrency      string `json:"gmv_currency"`
}

func toMetricsDTO(m Metrics) metricsDTO {
	return metricsDTO{
		UsersTotal:       m.UsersTotal,
		TeachersTotal:    m.TeachersTotal,
		TeachersPending:  m.TeachersPending,
		BookingsTotal:    m.BookingsTotal,
		BookingsThisWeek: m.BookingsThisWeek,
		GMVMinor:         m.GMVMinor,
		GMVCurrency:      m.GMVCurrency,
	}
}

// --- users ---

type userRowDTO struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"display_name"`
	CreatedAt    time.Time `json:"created_at"`
	IsTeacher    bool      `json:"is_teacher"`
	BookingCount int64     `json:"booking_count"`
}

type usersPageDTO struct {
	Users []userRowDTO `json:"users"`
	Total int          `json:"total"`
}

func toUsersPageDTO(p UsersPage) usersPageDTO {
	rows := make([]userRowDTO, len(p.Users))
	for i, u := range p.Users {
		rows[i] = userRowDTO{
			ID:           u.ID.String(),
			Email:        u.Email,
			DisplayName:  u.DisplayName,
			CreatedAt:    u.CreatedAt.UTC(),
			IsTeacher:    u.IsTeacher,
			BookingCount: u.BookingCount,
		}
	}
	return usersPageDTO{Users: rows, Total: p.Total}
}

type userCoreDTO struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

type roleRefDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type userTeacherProfileDTO struct {
	Slug     string `json:"slug"`
	Status   string `json:"status"`
	Verified bool   `json:"verified"`
}

type moneyDTO struct {
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
}

type userBookingDTO struct {
	ID             string    `json:"id"`
	Status         string    `json:"status"`
	StartAt        time.Time `json:"start_at"`
	RoleInBooking  string    `json:"role_in_booking"`
	OtherPartyName string    `json:"other_party_name"`
	Price          moneyDTO  `json:"price"`
}

type paymentsSummaryDTO struct {
	AuthorizedMinor int64  `json:"authorized_minor"`
	CapturedMinor   int64  `json:"captured_minor"`
	RefundedMinor   int64  `json:"refunded_minor"`
	Currency        string `json:"currency"`
}

type userDetailDTO struct {
	User            userCoreDTO            `json:"user"`
	Roles           []roleRefDTO           `json:"roles"`
	TeacherProfile  *userTeacherProfileDTO `json:"teacher_profile"`
	Bookings        []userBookingDTO       `json:"bookings"`
	PaymentsSummary paymentsSummaryDTO     `json:"payments_summary"`
}

func toUserDetailDTO(d UserDetail) userDetailDTO {
	out := userDetailDTO{
		User: userCoreDTO{
			ID:          d.User.ID.String(),
			Email:       d.User.Email,
			DisplayName: d.User.DisplayName,
			CreatedAt:   d.User.CreatedAt.UTC(),
		},
		Roles:    make([]roleRefDTO, len(d.Roles)),
		Bookings: make([]userBookingDTO, len(d.Bookings)),
		PaymentsSummary: paymentsSummaryDTO{
			AuthorizedMinor: d.Payments.AuthorizedMinor,
			CapturedMinor:   d.Payments.CapturedMinor,
			RefundedMinor:   d.Payments.RefundedMinor,
			Currency:        d.Payments.Currency,
		},
	}
	for i, rr := range d.Roles {
		out.Roles[i] = roleRefDTO{ID: rr.ID.String(), Name: rr.Name}
	}
	if d.TeacherProfile != nil {
		out.TeacherProfile = &userTeacherProfileDTO{
			Slug:     d.TeacherProfile.Slug,
			Status:   d.TeacherProfile.Status,
			Verified: d.TeacherProfile.Verified,
		}
	}
	for i, b := range d.Bookings {
		out.Bookings[i] = userBookingDTO{
			ID:             b.ID.String(),
			Status:         b.Status,
			StartAt:        b.StartAt.UTC(),
			RoleInBooking:  string(b.RoleInBooking),
			OtherPartyName: b.OtherPartyName,
			Price:          moneyDTO{AmountMinor: b.PriceMinor, Currency: b.Currency},
		}
	}
	return out
}

// --- teacher moderation ---

type ownerRefDTO struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type ownerDTO struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type teacherRowDTO struct {
	Slug        string      `json:"slug"`
	DisplayName string      `json:"display_name"`
	Status      string      `json:"status"`
	Verified    bool        `json:"verified"`
	Headline    string      `json:"headline"`
	CountryName string      `json:"country_name"`
	CreatedAt   time.Time   `json:"created_at"`
	Owner       ownerRefDTO `json:"owner"`
}

type teachersPageDTO struct {
	Teachers []teacherRowDTO `json:"teachers"`
	Total    int             `json:"total"`
}

func toTeachersPageDTO(p TeachersPage) teachersPageDTO {
	rows := make([]teacherRowDTO, len(p.Teachers))
	for i, t := range p.Teachers {
		row := teacherRowDTO{
			Slug:        t.Slug,
			DisplayName: t.DisplayName,
			Status:      t.Status,
			Verified:    t.Verified,
			Headline:    t.Headline,
			CountryName: t.CountryName,
			CreatedAt:   t.CreatedAt.UTC(),
		}
		if t.Owner.ID != uuid.Nil {
			row.Owner = ownerRefDTO{ID: t.Owner.ID.String(), Email: t.Owner.Email}
		}
		rows[i] = row
	}
	return teachersPageDTO{Teachers: rows, Total: p.Total}
}

// teacherDetailDTO is the full teachers-module profile plus the moderation
// slice and the owner. The embedded teachers.Profile already carries `status`,
// `verified` and `moderation_note`.
type teacherDetailDTO struct {
	teachers.Profile
	Owner ownerDTO `json:"owner"`
}

func toTeacherDetailDTO(d TeacherDetail) teacherDetailDTO {
	out := teacherDetailDTO{Profile: teachers.BuildProfile(d.Profile)}
	// The moderation row is the authoritative source for these three (it is the
	// row the write just touched), so the response always reflects the action
	// that was just applied.
	out.Status = d.Moderation.Status
	out.Verified = d.Moderation.Verified
	out.ModerationNote = d.Moderation.ModerationNote
	if d.Moderation.Owner.ID != uuid.Nil {
		out.Owner = ownerDTO{
			ID:          d.Moderation.Owner.ID.String(),
			Email:       d.Moderation.Owner.Email,
			DisplayName: d.Moderation.Owner.DisplayName,
		}
	}
	return out
}
