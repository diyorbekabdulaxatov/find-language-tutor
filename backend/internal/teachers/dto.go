package teachers

// Wire DTOs. These are the source of truth for the JSON shape and must stay in
// sync with openapi.yaml (snake_case, money as {amount_minor, currency}).

type moneyDTO struct {
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
}

type languageDTO struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Level string `json:"level"`
}

type experienceDTO struct {
	Title  string `json:"title"`
	Org    string `json:"org"`
	Period string `json:"period"`
}

// summaryDTO is the card view used in search results.
type summaryDTO struct {
	ID                string        `json:"id"`
	Slug              string        `json:"slug"`
	DisplayName       string        `json:"display_name"`
	Headline          string        `json:"headline"`
	Kind              string        `json:"kind"`
	CountryCode       string        `json:"country_code"`
	CountryName       string        `json:"country_name"`
	City              string        `json:"city"`
	Timezone          string        `json:"timezone"`
	Teaches           []languageDTO `json:"teaches"`
	AlsoSpeaks        []languageDTO `json:"also_speaks"`
	PricePerHour      moneyDTO      `json:"price_per_hour"`
	Rating            float64       `json:"rating"`
	ReviewCount       int           `json:"review_count"`
	LessonsCompleted  int           `json:"lessons_completed"`
	StudentCount      int           `json:"student_count"`
	Focus             []string      `json:"focus"`
	ResponseTimeHours int           `json:"response_time_hours"`
	AcceptingStudents bool          `json:"accepting_students"`
	AvatarURL         string        `json:"avatar_url"`
	VideoThumbnailURL string        `json:"video_thumbnail_url"`
}

// profileDTO is the full profile view. It embeds summaryDTO, so its JSON is the
// summary fields plus the long-form ones.
type profileDTO struct {
	summaryDTO
	IntroVideoURL string          `json:"intro_video_url"`
	About         string          `json:"about"`
	TeachingStyle string          `json:"teaching_style"`
	Experience    []experienceDTO `json:"experience"`
	TrialPrice    *moneyDTO       `json:"trial_price,omitempty"`
}

type languageFacetDTO struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type facetsDTO struct {
	Languages []languageFacetDTO `json:"languages"`
}

type listResponseDTO struct {
	Teachers []summaryDTO `json:"teachers"`
	Total    int          `json:"total"`
	Facets   facetsDTO    `json:"facets"`
}

// --- mapping ---

func money(m Money) moneyDTO {
	return moneyDTO{AmountMinor: m.AmountMinor, Currency: string(m.Currency)}
}

func languages(ls []Language) []languageDTO {
	out := make([]languageDTO, len(ls))
	for i, l := range ls {
		out[i] = languageDTO{Code: l.Code, Name: l.Name, Level: string(l.Level)}
	}
	return out
}

func toSummary(t Teacher) summaryDTO {
	focus := t.Focus
	if focus == nil {
		focus = []string{}
	}
	return summaryDTO{
		ID:                t.ID.String(),
		Slug:              t.Slug,
		DisplayName:       t.DisplayName,
		Headline:          t.Headline,
		Kind:              string(t.Kind),
		CountryCode:       t.CountryCode,
		CountryName:       t.CountryName,
		City:              t.City,
		Timezone:          t.Timezone,
		Teaches:           languages(t.Teaches),
		AlsoSpeaks:        languages(t.AlsoSpeaks),
		PricePerHour:      money(t.PricePerHour),
		Rating:            t.Rating,
		ReviewCount:       t.ReviewCount,
		LessonsCompleted:  t.LessonsCompleted,
		StudentCount:      t.StudentCount,
		Focus:             focus,
		ResponseTimeHours: t.ResponseTimeHours,
		AcceptingStudents: t.AcceptingStudents,
		AvatarURL:         t.AvatarURL,
		VideoThumbnailURL: t.VideoThumbnailURL,
	}
}

func toProfile(t Teacher) profileDTO {
	exp := make([]experienceDTO, len(t.Experience))
	for i, e := range t.Experience {
		exp[i] = experienceDTO{Title: e.Title, Org: e.Org, Period: e.Period}
	}

	p := profileDTO{
		summaryDTO:    toSummary(t),
		IntroVideoURL: t.IntroVideoURL,
		About:         t.About,
		TeachingStyle: t.TeachingStyle,
		Experience:    exp,
	}
	if t.TrialPrice != nil {
		m := money(*t.TrialPrice)
		p.TrialPrice = &m
	}
	return p
}

func toListResponse(res ListResult) listResponseDTO {
	teachers := make([]summaryDTO, len(res.Teachers))
	for i, t := range res.Teachers {
		teachers[i] = toSummary(t)
	}

	facets := make([]languageFacetDTO, len(res.Facets.Languages))
	for i, f := range res.Facets.Languages {
		facets[i] = languageFacetDTO{Code: f.Code, Name: f.Name, Count: f.Count}
	}

	return listResponseDTO{
		Teachers: teachers,
		Total:    res.Total,
		Facets:   facetsDTO{Languages: facets},
	}
}
