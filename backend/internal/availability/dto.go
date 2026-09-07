package availability

// Wire DTOs. These are the source of truth for the JSON shape and must stay in
// sync with openapi.yaml (snake_case). Times are minutes from 00:00 UTC.

type slotDTO struct {
	Weekday     int `json:"weekday"`      // 0 = Sunday .. 6 = Saturday
	StartMinute int `json:"start_minute"` // minutes from 00:00 UTC, multiple of granularity_minutes
	EndMinute   int `json:"end_minute"`   // exclusive; > start_minute
}

type availabilityDTO struct {
	TeacherSlug        string    `json:"teacher_slug"`
	Timezone           string    `json:"timezone"`
	GranularityMinutes int       `json:"granularity_minutes"`
	Slots              []slotDTO `json:"slots"`
}

type replaceRequestDTO struct {
	Slots []slotDTO `json:"slots"`
}

// --- mapping ---

func toAvailabilityDTO(wa WeeklyAvailability) availabilityDTO {
	slots := make([]slotDTO, len(wa.Slots))
	for i, s := range wa.Slots {
		slots[i] = slotDTO{
			Weekday:     int(s.Weekday),
			StartMinute: s.StartMinute,
			EndMinute:   s.EndMinute,
		}
	}
	return availabilityDTO{
		TeacherSlug:        wa.TeacherSlug,
		Timezone:           wa.Timezone,
		GranularityMinutes: SlotGranularityMinutes,
		Slots:              slots,
	}
}

func fromSlotDTOs(in []slotDTO) []Slot {
	out := make([]Slot, len(in))
	for i, s := range in {
		out[i] = Slot{
			Weekday:     Weekday(s.Weekday),
			StartMinute: s.StartMinute,
			EndMinute:   s.EndMinute,
		}
	}
	return out
}
