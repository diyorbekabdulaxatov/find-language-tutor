package main

import "strings"

// demoEmail derives the demo login for a teacher from their display name:
// "Nodira Karimova" -> "nodira@example.com". First names in the seed set are
// unique and ASCII, so this stays collision-free.
func demoEmail(displayName string) string {
	first := displayName
	if i := strings.IndexAny(displayName, " -"); i > 0 {
		first = displayName[:i]
	}
	return strings.ToLower(first) + "@example.com"
}

// seedTeachers mirrors frontend/src/features/teachers/mock-data.ts so the
// frontend gets identical data when it switches from its mock module to the API.

type seedLang struct {
	Code, Name, Level string
}

type seedExp struct {
	Title, Org, Period string
}

type seedTeacher struct {
	Slug, DisplayName, Headline, Kind        string
	CountryCode, CountryName, City, Timezone string
	PriceMinor                               int64
	TrialMinor                               *int64 // nil = no trial
	Rating                                   float32
	ReviewCount, LessonsCompleted            int32
	StudentCount, ResponseTimeHours          int32
	Accepting                                bool
	AvatarURL, VideoThumbnailURL             string
	IntroVideoURL, About, TeachingStyle      string
	Teaches, AlsoSpeaks                      []seedLang
	Focus                                    []string
	Experience                               []seedExp
	// Moderation (phase B). Status defaults to "approved" when empty so every
	// existing seed row stays live; Verified drives the badge.
	Status   string
	Verified bool
}

func minor(v int64) *int64 { return &v }

// seedSlot is one weekly availability span, in minutes from 00:00 UTC.
// weekday: 0 = Sunday .. 6 = Saturday.
type seedSlot struct {
	Weekday    int
	Start, End int
}

// at builds a slot from clock hours in UTC, e.g. at(6, 4, 0, 9, 0) == Saturday
// 04:00–09:00 UTC. Tashkent is UTC+5, so that is 09:00–14:00 local.
func at(weekday, startH, startM, endH, endM int) seedSlot {
	return seedSlot{Weekday: weekday, Start: startH*60 + startM, End: endH*60 + endM}
}

// week repeats the same UTC window across several weekdays.
func week(days []int, startH, startM, endH, endM int) []seedSlot {
	out := make([]seedSlot, 0, len(days))
	for _, d := range days {
		out = append(out, at(d, startH, startM, endH, endM))
	}
	return out
}

func slots(groups ...[]seedSlot) []seedSlot {
	var out []seedSlot
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// seedAvailability is keyed by teacher slug. Windows are in UTC; most teachers
// are in Asia/Tashkent (UTC+5). Deleting a teacher cascades to these rows, so
// re-running the seed stays idempotent.
var seedAvailability = map[string][]seedSlot{
	// Tashkent full-timer: weekday mornings + evenings, Sat morning.
	"nodira-karimova": slots(
		week([]int{1, 2, 3, 4, 5}, 4, 0, 8, 0), // 09:00–13:00 local
		week([]int{1, 3}, 12, 0, 15, 0),        // 17:00–20:00 local
		week([]int{6}, 5, 0, 9, 0),             // Sat 10:00–14:00 local
	),
	// Works a day job — evenings and weekend.
	"sardor-yusupov": slots(
		week([]int{1, 2, 4}, 14, 0, 17, 0), // 19:00–22:00 local
		week([]int{0, 6}, 6, 0, 10, 0),     // weekend 11:00–15:00 local
	),
	"elena-kim": slots(
		week([]int{1, 2, 3, 4, 5}, 5, 30, 11, 0),
	),
	// Samarkand (Asia/Samarkand, also UTC+5).
	"dilnoza-abdullayeva": slots(
		week([]int{1, 2, 3, 4, 5}, 8, 0, 12, 0), // after-school 13:00–17:00 local
		week([]int{6}, 5, 0, 8, 0),
	),
	"jasur-rakhimov": slots(
		week([]int{2, 4}, 13, 0, 17, 0),
		week([]int{6}, 7, 0, 11, 0),
	),
	"aziza-tosheva": slots(
		week([]int{1, 3, 5}, 6, 0, 10, 0),
	),
	// Seoul (Asia/Seoul, UTC+9).
	"kim-min-jun": slots(
		week([]int{1, 2, 3, 4}, 1, 0, 5, 0), // 10:00–14:00 Seoul
		week([]int{6}, 0, 0, 3, 0),
	),
	// Istanbul (Europe/Istanbul, UTC+3).
	"mehmet-demir": slots(
		week([]int{2, 4}, 15, 0, 18, 0), // 18:00–21:00 Istanbul
		week([]int{0}, 9, 0, 12, 0),
	),
	"kamola-sattorova": slots(
		week([]int{1, 2, 3, 4, 5}, 4, 0, 7, 0),
		week([]int{1, 3, 5}, 11, 0, 13, 0),
	),
	// Andijan (UTC+5), history teacher — evenings + Sunday.
	"bekzod-ergashev": slots(
		week([]int{1, 2, 3, 4, 5}, 13, 0, 16, 0),
		week([]int{0}, 6, 0, 10, 0),
	),
	// Pending teacher (Tashkent, UTC+5): a filled availability that only becomes
	// bookable once an admin approves the profile.
	"malika-abdurakhmonova": slots(
		week([]int{1, 2, 3, 4, 5}, 12, 0, 15, 0), // 17:00–20:00 local
	),
}

// seedReview is one sample review. Student is the demo account for another
// seeded teacher, addressed by first name (-> "<name>@example.com"). booking_id
// is always NULL for these — they stand in for the historical review history
// behind a teacher's hand-set rating / review_count ("showing 4 of 214"), so
// the seed deliberately does NOT recompute the aggregate from them.
type seedReview struct {
	StudentFirstName string
	Rating           int
	Comment          string
}

// seedReviews is keyed by teacher slug. 3–4 rows each so
// GET /v1/teachers/{slug}/reviews returns content on a fresh database.
var seedReviews = map[string][]seedReview{
	"nodira-karimova": {
		{"sardor", 5, "Went from 6.0 to 7.5 in speaking in two months. The weekly targets kept me honest."},
		{"elena", 5, "Extremely well prepared every lesson. The recordings of my answers were eye-opening."},
		{"jasur", 4, "Tough but fair feedback on my writing. Homework every time, which I needed."},
		{"kamola", 5, "Calm and encouraging even when I froze up. Got the band I needed for my visa."},
	},
	"sardor-yusupov": {
		{"dilnoza", 5, "Finally comfortable on work calls in English. We just talk the whole hour."},
		{"bekzod", 4, "Relaxed sessions, practical corrections. Wish he had more evening slots."},
		{"aziza", 5, "Great for interview prep — he pushes you to actually explain things."},
	},
	"elena-kim": {
		{"nodira", 5, "My daughter's grammar finally clicked. Clear explanations, patient pace."},
		{"mehmet", 5, "Thorough and structured. The grammar references after each topic are gold."},
		{"kamola", 4, "Very solid teacher for university prep. Lessons are calm and focused."},
	},
	"dilnoza-abdullayeva": {
		{"sardor", 5, "My son actually asks when the next lesson is. Games and stories really work."},
		{"elena", 5, "Kind and consistent. The after-lesson notes for parents are a nice touch."},
		{"jasur", 4, "Good with reluctant teens — brought in football and YouTube to hook him."},
	},
	"jasur-rakhimov": {
		{"dilnoza", 4, "Cleared up tenses and articles that cost me marks for years. Affordable too."},
		{"bekzod", 5, "Uses my own writing for error correction, which made it stick."},
		{"nodira", 4, "Reliable exam-prep tutor. Checks homework at the start of every lesson."},
	},
	"aziza-tosheva": {
		{"elena", 5, "Passed my Goethe B1 first try. The colour-coded grammar patterns are brilliant."},
		{"sardor", 5, "Prepared me for real situations — renting a flat, the Bürgeramt, interviews."},
		{"kamola", 5, "Systematic and clear. Best German teacher I've had, online or offline."},
	},
	"kim-min-jun": {
		{"nodira", 5, "Started from Hangul and never picked up bad habits. Slides and flashcards every time."},
		{"aziza", 4, "Speaks mostly Korean early on — hard at first, worth it fast."},
		{"mehmet", 5, "Great for TOPIK prep and for understanding K-content without subtitles."},
	},
	"mehmet-demir": {
		{"bekzod", 5, "Casual and fun. Uzbek and Turkish being close made the first lesson click."},
		{"dilnoza", 4, "Nice relaxed conversation practice. He sends a word list after each session."},
		{"jasur", 5, "He points out the false friends between our languages — very helpful."},
	},
	"kamola-sattorova": {
		{"elena", 5, "Survival Uzbek for the bazaar and taxis from lesson one. Warm and practical."},
		{"aziza", 5, "Lots of role-play from real situations. My neighbours noticed the difference."},
		{"sardor", 4, "Teaches the spoken Tashkent variety first, which is exactly what I needed."},
	},
	"bekzod-ergashev": {
		{"kamola", 5, "Reconnected with the Uzbek I grew up hearing. Never once made me feel behind."},
		{"nodira", 5, "Gentle, patient, lots of repetition. My speaking came back faster than expected."},
		{"mehmet", 4, "Good heritage-speaker lessons. We mostly talk about family and food."},
	},
}

var seedTeachers = []seedTeacher{
	{
		Slug: "nodira-karimova", DisplayName: "Nodira Karimova",
		Headline: "IELTS students who freeze up in the speaking test — that's my specialty",
		Kind:     "professional", Verified: true,
		CountryCode: "UZ", CountryName: "Uzbekistan", City: "Tashkent", Timezone: "Asia/Tashkent",
		PriceMinor: 9_000_000, TrialMinor: minor(3_000_000),
		Rating: 4.9, ReviewCount: 214, LessonsCompleted: 3800, StudentCount: 260, ResponseTimeHours: 1, Accepting: true,
		AvatarURL:         "https://i.pravatar.cc/240?img=45",
		VideoThumbnailURL: "https://picsum.photos/seed/nodira/640/360",
		IntroVideoURL:     "https://example.com/video/nodira",
		About:             "I've taught English in Tashkent for nine years, the last five focused almost entirely on IELTS. More than 200 of my students have reached the band they needed for a scholarship or a visa.\n\nMost people come to me because their speaking score won't move. We work on fluency first — getting you talking without the long pauses — then accuracy. You get a written summary and a target for the week after every lesson.",
		TeachingStyle:     "Structured but conversational. Each lesson has a clear goal tied to one of the four IELTS skills. I record your speaking answers so you can hear the difference over time. Homework is short and every lesson.",
		Teaches:           []seedLang{{"en", "English", "c2"}},
		AlsoSpeaks:        []seedLang{{"uz", "Uzbek", "native"}, {"ru", "Russian", "c1"}},
		Focus:             []string{"IELTS", "Speaking", "Academic writing"},
		Experience: []seedExp{
			{"IELTS instructor", "Cambridge Language Centre, Tashkent", "2019 – present"},
			{"English teacher", "Presidential School, Tashkent", "2015 – 2019"},
		},
	},
	{
		Slug: "sardor-yusupov", DisplayName: "Sardor Yusupov",
		Headline:    "Talk about your job, your week, your plans — in English, from the first lesson",
		Kind:        "community",
		CountryCode: "UZ", CountryName: "Uzbekistan", City: "Tashkent", Timezone: "Asia/Tashkent",
		PriceMinor: 5_500_000, TrialMinor: minor(2_000_000),
		Rating: 4.7, ReviewCount: 96, LessonsCompleted: 1720, StudentCount: 140, ResponseTimeHours: 3, Accepting: true,
		AvatarURL:         "https://i.pravatar.cc/240?img=33",
		VideoThumbnailURL: "https://picsum.photos/seed/sardor/640/360",
		IntroVideoURL:     "https://example.com/video/sardor",
		About:             "I work in IT project management and use English every day with clients abroad. I started tutoring because I remember how useless it felt to know grammar rules but not be able to hold a call.\n\nThese are speaking-focused sessions for people who already have some English and need to actually use it — at work, in interviews, while travelling.",
		TeachingStyle:     "We talk the whole hour. I take notes on the mistakes that get in the way of being understood and we review them at the end. I'll push you to explain things, not just answer yes or no.",
		Teaches:           []seedLang{{"en", "English", "c1"}},
		AlsoSpeaks:        []seedLang{{"uz", "Uzbek", "native"}, {"ru", "Russian", "c2"}},
		Focus:             []string{"Conversational", "Business English", "Interview prep"},
		Experience: []seedExp{
			{"IT project manager", "EPAM Uzbekistan", "2018 – present"},
			{"Conversation tutor", "Independent", "2021 – present"},
		},
	},
	{
		Slug: "elena-kim", DisplayName: "Elena Kim",
		Headline: "Russian for school and university, explained the way it finally clicks",
		Kind:     "professional", Verified: true,
		CountryCode: "UZ", CountryName: "Uzbekistan", City: "Tashkent", Timezone: "Asia/Tashkent",
		PriceMinor: 7_000_000, TrialMinor: minor(2_500_000),
		Rating: 4.9, ReviewCount: 158, LessonsCompleted: 4300, StudentCount: 190, ResponseTimeHours: 2, Accepting: true,
		AvatarURL:         "https://i.pravatar.cc/240?img=47",
		VideoThumbnailURL: "https://picsum.photos/seed/elena/640/360",
		IntroVideoURL:     "https://example.com/video/elena",
		About:             "I taught Russian language and literature in a Tashkent school for fifteen years before moving to tutoring full time.\n\nMy students are mostly teenagers preparing for university entrance and adults who need clearer, more correct Russian for work. We build from wherever you are — no lesson assumes you remember something we haven't covered.",
		TeachingStyle:     "Calm and thorough. I explain a rule, we practise it in speech and in writing, and I don't move on until it's solid. I send a short grammar reference after each topic so you can review offline.",
		Teaches:           []seedLang{{"ru", "Russian", "native"}},
		AlsoSpeaks:        []seedLang{{"uz", "Uzbek", "c1"}, {"en", "English", "b2"}},
		Focus:             []string{"University prep", "Grammar", "Writing"},
		Experience: []seedExp{
			{"Russian language teacher", "School No. 110, Tashkent", "2007 – 2022"},
			{"Private tutor", "Independent", "2022 – present"},
		},
	},
	{
		Slug: "dilnoza-abdullayeva", DisplayName: "Dilnoza Abdullayeva",
		Headline:    "Patient English lessons for kids who'd rather be doing anything else",
		Kind:        "professional",
		CountryCode: "UZ", CountryName: "Uzbekistan", City: "Samarkand", Timezone: "Asia/Samarkand",
		PriceMinor: 6_500_000, TrialMinor: minor(2_000_000),
		Rating: 4.8, ReviewCount: 122, LessonsCompleted: 2600, StudentCount: 110, ResponseTimeHours: 4, Accepting: true,
		AvatarURL:         "https://i.pravatar.cc/240?img=31",
		VideoThumbnailURL: "https://picsum.photos/seed/dilnoza/640/360",
		IntroVideoURL:     "https://example.com/video/dilnoza",
		About:             "I specialise in learners aged 7 to 15. I have a teaching degree and a CELT-P certificate for teaching young learners.\n\nLessons use games, stories, and a lot of pictures. Parents get a short message after each lesson so you know what we did and how it went.",
		TeachingStyle:     "Playful and consistent. Short activities, frequent changes of pace, praise for effort. For teens I bring in music, football, and YouTube so the English feels worth learning.",
		Teaches:           []seedLang{{"en", "English", "c1"}},
		AlsoSpeaks:        []seedLang{{"uz", "Uzbek", "native"}, {"ru", "Russian", "b2"}},
		Focus:             []string{"Kids & teens", "Beginners", "Cambridge YLE"},
		Experience: []seedExp{
			{"Primary English teacher", "Fidokor School, Samarkand", "2016 – present"},
			{"CELT-P (young learners)", "British Council", "2018"},
		},
	},
	{
		Slug: "jasur-rakhimov", DisplayName: "Jasur Rakhimov",
		Headline:    "We fix the grammar mistakes that quietly cost you marks on the exam",
		Kind:        "community",
		CountryCode: "UZ", CountryName: "Uzbekistan", City: "Namangan", Timezone: "Asia/Tashkent",
		PriceMinor: 4_500_000, TrialMinor: nil,
		Rating: 4.6, ReviewCount: 71, LessonsCompleted: 1180, StudentCount: 95, ResponseTimeHours: 8, Accepting: true,
		AvatarURL:         "https://i.pravatar.cc/240?img=52",
		VideoThumbnailURL: "https://picsum.photos/seed/jasur/640/360",
		IntroVideoURL:     "https://example.com/video/jasur",
		About:             "I'm a final-year linguistics student and I've been tutoring school and university students for four years.\n\nMost of my students are getting ready for a placement test or a national exam and keep losing points on the same few things — tenses, articles, prepositions. We drill those until they're automatic.",
		TeachingStyle:     "Explanation, then a lot of practice. I use error correction from your own writing rather than generic worksheets. Homework every lesson, checked at the start of the next one.",
		Teaches:           []seedLang{{"en", "English", "c1"}},
		AlsoSpeaks:        []seedLang{{"uz", "Uzbek", "native"}, {"ru", "Russian", "b1"}},
		Focus:             []string{"Grammar", "Exam prep", "CEFR A2–B1"},
		Experience: []seedExp{
			{"Linguistics student", "Namangan State University", "2021 – present"},
			{"English tutor", "Independent", "2020 – present"},
		},
	},
	{
		Slug: "aziza-tosheva", DisplayName: "Aziza Tosheva",
		Headline:    "German is logical once someone shows you the pattern — I'll show you",
		Kind:        "professional",
		CountryCode: "UZ", CountryName: "Uzbekistan", City: "Tashkent", Timezone: "Asia/Tashkent",
		PriceMinor: 8_500_000, TrialMinor: minor(3_000_000),
		Rating: 4.9, ReviewCount: 84, LessonsCompleted: 1600, StudentCount: 90, ResponseTimeHours: 5, Accepting: false,
		AvatarURL:         "https://i.pravatar.cc/240?img=44",
		VideoThumbnailURL: "https://picsum.photos/seed/aziza/640/360",
		IntroVideoURL:     "https://example.com/video/aziza",
		About:             "I studied for a master's degree in Cologne and now teach German to people planning the same move — for an Ausbildung, a university place, or the Blue Card.\n\nWe follow a clear A1-to-B2 path and prepare specifically for the Goethe or telc exam you need.",
		TeachingStyle:     "Systematic. I teach grammar in patterns and colours, not long lists. Lots of speaking practice about real situations: renting a flat, registering at the Bürgeramt, a job interview.",
		Teaches:           []seedLang{{"de", "German", "c1"}},
		AlsoSpeaks:        []seedLang{{"uz", "Uzbek", "native"}, {"ru", "Russian", "c1"}, {"en", "English", "b2"}},
		Focus:             []string{"Goethe exam", "A1–B2", "Studying in Germany"},
		Experience: []seedExp{
			{"M.A., Media Studies", "University of Cologne", "2016 – 2018"},
			{"German instructor", "DeutschKlub Tashkent", "2019 – present"},
		},
	},
	{
		Slug: "kim-min-jun", DisplayName: "Kim Min-jun",
		Headline: "From the K-drama phrases you already know to real conversation — and TOPIK if you want it",
		Kind:     "professional", Verified: true,
		CountryCode: "KR", CountryName: "South Korea", City: "Seoul", Timezone: "Asia/Seoul",
		PriceMinor: 11_000_000, TrialMinor: minor(4_000_000),
		Rating: 4.8, ReviewCount: 133, LessonsCompleted: 2900, StudentCount: 180, ResponseTimeHours: 6, Accepting: true,
		AvatarURL:         "https://i.pravatar.cc/240?img=68",
		VideoThumbnailURL: "https://picsum.photos/seed/minjun/640/360",
		IntroVideoURL:     "https://example.com/video/minjun",
		About:             "I'm a certified Korean teacher in Seoul with students in a dozen countries, including a growing group in Uzbekistan.\n\nWhether you're learning for K-content, for work with Korean companies, or to study in Korea, we start with solid Hangul and pronunciation so bad habits never set in.",
		TeachingStyle:     "Immersive but supported. I speak mostly Korean from early on, with English when it saves time. Shared slides for every grammar point and a flashcard deck after each lesson.",
		Teaches:           []seedLang{{"ko", "Korean", "native"}},
		AlsoSpeaks:        []seedLang{{"en", "English", "c1"}},
		Focus:             []string{"TOPIK", "Speaking", "Hangul & pronunciation"},
		Experience: []seedExp{
			{"Certified Korean language instructor", "Sejong Institute", "2017 – present"},
		},
	},
	{
		Slug: "mehmet-demir", DisplayName: "Mehmet Demir",
		Headline:    "Relaxed Turkish conversation — bring a coffee, we'll just talk",
		Kind:        "community",
		CountryCode: "TR", CountryName: "Türkiye", City: "Istanbul", Timezone: "Europe/Istanbul",
		PriceMinor: 6_000_000, TrialMinor: minor(2_000_000),
		Rating: 4.7, ReviewCount: 58, LessonsCompleted: 940, StudentCount: 80, ResponseTimeHours: 5, Accepting: true,
		AvatarURL:         "https://i.pravatar.cc/240?img=59",
		VideoThumbnailURL: "https://picsum.photos/seed/mehmet/640/360",
		IntroVideoURL:     "https://example.com/video/mehmet",
		About:             "I'm a graphic designer in Istanbul and I love that Uzbek and Turkish are close enough to get a real conversation going in the first lesson.\n\nThese are casual speaking sessions. Bring a topic or let me pick one. I'll point out the false friends between our languages so they stop tripping you up.",
		TeachingStyle:     "Casual and patient. We talk, I correct what matters, and I keep a running list of new words that I send you afterwards.",
		Teaches:           []seedLang{{"tr", "Turkish", "native"}},
		AlsoSpeaks:        []seedLang{{"en", "English", "b2"}, {"ru", "Russian", "b1"}},
		Focus:             []string{"Conversational", "Travel", "Uzbek speakers"},
		Experience: []seedExp{
			{"Conversation tutor", "Independent", "2022 – present"},
		},
	},
	{
		Slug: "kamola-sattorova", DisplayName: "Kamola Sattorova",
		Headline:    "Uzbek for people who just moved to Tashkent and want to stop pointing at menus",
		Kind:        "professional",
		CountryCode: "UZ", CountryName: "Uzbekistan", City: "Tashkent", Timezone: "Asia/Tashkent",
		PriceMinor: 7_500_000, TrialMinor: minor(2_500_000),
		Rating: 4.9, ReviewCount: 76, LessonsCompleted: 1340, StudentCount: 85, ResponseTimeHours: 2, Accepting: true,
		AvatarURL:         "https://i.pravatar.cc/240?img=24",
		VideoThumbnailURL: "https://picsum.photos/seed/kamola/640/360",
		IntroVideoURL:     "https://example.com/video/kamola",
		About:             "I have a degree in Uzbek philology and I've spent the last six years teaching the language to diplomats, NGO staff, and people who married into an Uzbek family.\n\nWe start with the survival phrases you need this week — the bazaar, the taxi, your neighbours — and build grammar around them so it never feels abstract.",
		TeachingStyle:     "Practical and warm. Lots of role-play from real situations you'll face. I teach the spoken Tashkent variety first and point out where the textbook written form differs.",
		Teaches:           []seedLang{{"uz", "Uzbek", "native"}},
		AlsoSpeaks:        []seedLang{{"ru", "Russian", "native"}, {"en", "English", "c1"}},
		Focus:             []string{"Beginners", "Everyday Uzbek", "Newcomers to Tashkent"},
		Experience: []seedExp{
			{"Uzbek language instructor", "Internews / diplomatic missions, Tashkent", "2019 – present"},
			{"B.A., Uzbek Philology", "National University of Uzbekistan", "2013 – 2017"},
		},
	},
	{
		Slug: "bekzod-ergashev", DisplayName: "Bekzod Ergashev",
		Headline:    "Reconnect with Uzbek — for anyone who understood their grandparents but never spoke back",
		Kind:        "community",
		CountryCode: "UZ", CountryName: "Uzbekistan", City: "Andijan", Timezone: "Asia/Tashkent",
		PriceMinor: 5_000_000, TrialMinor: minor(1_500_000),
		Rating: 4.7, ReviewCount: 44, LessonsCompleted: 720, StudentCount: 60, ResponseTimeHours: 7, Accepting: true,
		AvatarURL:         "https://i.pravatar.cc/240?img=13",
		VideoThumbnailURL: "https://picsum.photos/seed/bekzod/640/360",
		IntroVideoURL:     "https://example.com/video/bekzod",
		About:             "I'm a history teacher in Andijan. Most of my online students grew up abroad in Uzbek families and can follow a conversation but freeze when it's their turn.\n\nNo grammar drills unless you ask. We talk about family, food, and where your relatives are from, and your speaking comes back faster than you'd think.",
		TeachingStyle:     "Gentle and conversational. I speak slowly, repeat a lot, and never make you feel behind. I'll write down the words you reach for in Russian or English so you have them next time.",
		Teaches:           []seedLang{{"uz", "Uzbek", "native"}},
		AlsoSpeaks:        []seedLang{{"ru", "Russian", "c1"}, {"en", "English", "b1"}},
		Focus:             []string{"Conversational", "Heritage speakers", "Fergana dialect"},
		Experience: []seedExp{
			{"History teacher", "School No. 24, Andijan", "2015 – present"},
			{"Uzbek conversation tutor", "Independent", "2021 – present"},
		},
	},
	{
		// Sits in the admin moderation queue on a fresh DB (status = 'pending'):
		// a complete profile that is NOT public until an admin approves it.
		// Demo account: malika@example.com / "password".
		Slug: "malika-abdurakhmonova", DisplayName: "Malika Abdurakhmonova",
		Headline: "Business English for Uzbek professionals — emails, calls, and negotiations that land",
		Kind:     "professional", Status: "pending",
		CountryCode: "UZ", CountryName: "Uzbekistan", City: "Tashkent", Timezone: "Asia/Tashkent",
		PriceMinor: 7_000_000, TrialMinor: minor(2_500_000),
		Rating: 0, ReviewCount: 0, LessonsCompleted: 0, StudentCount: 0, ResponseTimeHours: 6, Accepting: true,
		AvatarURL:         "https://i.pravatar.cc/240?img=32",
		VideoThumbnailURL: "https://picsum.photos/seed/malika/640/360",
		IntroVideoURL:     "https://example.com/video/malika",
		About:             "I spent eight years in corporate finance in Tashkent and Almaty, most of it working in English with regional teams.\n\nI coach professionals who already have solid English but want it to sound sharper at work — clearer emails, more confident calls, and the phrasing that makes a negotiation go your way.",
		TeachingStyle:     "We work from your real material: an email you need to send, a presentation next week, a call you're dreading. Every lesson ends with something you can use the next morning.",
		Teaches:           []seedLang{{"en", "English", "c1"}},
		AlsoSpeaks:        []seedLang{{"uz", "Uzbek", "native"}, {"ru", "Russian", "c2"}},
		Focus:             []string{"Business English", "Email writing", "Negotiation"},
		Experience: []seedExp{
			{"Finance manager", "Regional FMCG group, Tashkent / Almaty", "2016 – 2024"},
			{"Business English coach", "Independent", "2023 – present"},
		},
	},
}
