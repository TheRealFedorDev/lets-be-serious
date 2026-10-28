package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const seedContentVersion = "7"

type seedDestination struct {
	Slug        string
	City        string
	Country     string
	Tagline     string
	Description string
	Image       string
	PriceFrom   int
	Tags        []string
	Aliases     []string
}

type seedHotel struct {
	DestinationSlug  string
	Slug             string
	Name             string
	ShortDescription string
	Description      string
	Address          string
	Image            string
	Gallery          []string
	Rating           float64
	Reviews          int
	Price            int
	Tags             []string
	Amenities        []string
	Featured         bool
	Meta             HotelMeta
}

type seedPOI struct {
	DestinationSlug string
	Name            string
	Category        string
	Description     string
	Address         string
	Distance        string
	Minutes         int
	Tags            []string
	PriceLevel      int
	Lat             float64
	Lng             float64
	Schematic       bool
}

func mkMeta(lat, lng float64, insured, guides, soundproof, parking, pool bool, discount, food, metro, beach int) HotelMeta {
	return HotelMeta{
		Lat: lat, Lng: lng,
		Insured: insured, Guides: guides, Soundproof: soundproof,
		Parking: parking, Pool: pool, Discount: discount,
		FoodMinutes: food, MetroMinutes: metro, BeachMinutes: beach,
	}
}

func seedBaseData(ctx context.Context, store *Store) error {
	_, _ = store.db.ExecContext(ctx,
		`UPDATE preferences SET accent = ?, menu_tint = ? WHERE accent = ? OR menu_tint = ?`,
		"#1F4BDB", "#FFFFFF", "#D45A3A", "#FBF7EF",
	)
	var version string
	_ = store.db.QueryRowContext(ctx, `SELECT value FROM seed_meta WHERE key = 'content_version'`).Scan(&version)
	if version == seedContentVersion {
		return nil
	}

	destinations := []seedDestination{
		{
			Slug: "moscow", City: "Москва", Country: "Россия",
			Tagline: "Большой город с тихими дворами", Image: "/assets/images/moscow.jpg", PriceFrom: 7400,
			Description: "Архитектура, кофе и вечерние прогулки — здесь легко переключиться с ритма мегаполиса на свой собственный.",
			Tags:        []string{"архитектура", "кофе", "вечерние прогулки"},
			Aliases:     []string{"москва", "москве", "москву", "moscow"},
		},
		{
			Slug: "saint-petersburg", City: "Санкт-Петербург", Country: "Россия",
			Tagline: "Белые ночи и тихие дворы", Image: "/assets/images/saint-petersburg.jpg", PriceFrom: 7600,
			Description: "Витражные фасады, вода и медленные завтраки. Город, в который хочется возвращаться без повода.",
			Tags:        []string{"история", "дизайн", "завтраки"},
			Aliases:     []string{"санкт-петербург", "петербург", "петербурге", "питер", "спб", "saint petersburg"},
		},
		{
			Slug: "sochi", City: "Сочи", Country: "Россия",
			Tagline: "Море, пальмы и медленные завтраки", Image: "/assets/images/sochi.jpg", PriceFrom: 14200,
			Description: "Утро начинается с моря, день — с гор, вечер — с длинной набережной. Отдых без лишнего расписания.",
			Tags:        []string{"море", "спа", "горы"},
			Aliases:     []string{"сочи", "sochi"},
		},
		{
			Slug: "kazan", City: "Казань", Country: "Россия",
			Tagline: "Тысяча лет на одном берегу", Image: "/assets/images/kazan.jpg", PriceFrom: 9400,
			Description: "Кремль, гастрономия и современная архитектура в одном маршруте. Город, где легко совмещать культуру и отдых.",
			Tags:        []string{"гастрономия", "культура", "центр"},
			Aliases:     []string{"казань", "казани", "казанью", "kazan"},
		},
		{
			Slug: "kaliningrad", City: "Калининград", Country: "Россия",
			Tagline: "Балтийский воздух и кирпичная готика", Image: "/assets/images/kaliningrad.jpg", PriceFrom: 8900,
			Description: "Морской бриз, тихие улицы и лучшие завтраки с видом на воду. Небольшое путешествие с большим послевкусием.",
			Tags:        []string{"балтика", "тишина", "завтраки"},
			Aliases:     []string{"калининград", "калининграде", "kaliningrad"},
		},
		{
			Slug: "altai", City: "Алтай", Country: "Россия",
			Tagline: "Горы, тишина и звёзды", Image: "/assets/images/altai.jpg", PriceFrom: 12600,
			Description: "Деревянные дома, панорамные окна и воздух, в котором слышно только ветер. Для тех, кто хочет выдохнуть.",
			Tags:        []string{"горы", "природа", "тишина"},
			Aliases:     []string{"алтай", "алтае", "алтаю", "горный алтай", "altai"},
		},
		{
			Slug: "yekaterinburg", City: "Екатеринбург", Country: "Россия",
			Tagline: "Конструктивизм и горы рядом", Image: "/assets/images/hotel-07.jpg", PriceFrom: 7200,
			Description: "Город-конструктор: строгая геометрия, локальный кофе и Урал в получасе от центра.",
			Tags:        []string{"архитектура", "кофе", "горы"},
			Aliases:     []string{"екатеринбург", "екатеринбурге", "екб", "yekaterinburg", "ekaterinburg"},
		},
		{
			Slug: "nizhny-novgorod", City: "Нижний Новгород", Country: "Россия",
			Tagline: "Стрелка двух рек", Image: "/assets/images/hotel-05.jpg", PriceFrom: 6900,
			Description: "Закаты над Волгой, кремль на холме и улицы, по которым хочется гулять до темноты.",
			Tags:        []string{"реки", "закаты", "история"},
			Aliases:     []string{"нижний новгород", "нижнем новгороде", "нижний", "nizhny novgorod"},
		},
		{
			Slug: "irkutsk", City: "Иркутск", Country: "Россия",
			Tagline: "Байкал начинается здесь", Image: "/assets/images/altai.jpg", PriceFrom: 7100,
			Description: "Деревянные кружева, озеро в часе езды и маршруты, после которых меняется масштаб.",
			Tags:        []string{"озёра", "лес", "природа"},
			Aliases:     []string{"иркутск", "иркутске", "байкал", "irkutsk", "baikal"},
		},
		{
			Slug: "vladivostok", City: "Владивосток", Country: "Россия",
			Tagline: "Океан и сопки", Image: "/assets/images/hotel-11.jpg", PriceFrom: 9600,
			Description: "Море во всех направлениях, мосты на сопках и лучшие закаты на Дальнем Востоке.",
			Tags:        []string{"море", "закаты", "морепродукты"},
			Aliases:     []string{"владивосток", "владивостоке", "vladivostok"},
		},
		{
			Slug: "murmansk", City: "Мурманск", Country: "Россия",
			Tagline: "Северное сияние и океан", Image: "/assets/images/hotel-10.jpg", PriceFrom: 8700,
			Description: "Полярная ночь, снег и небо, которое иногда светится. Место, где тишина звучит громче города.",
			Tags:        []string{"полярное сияние", "зима", "горы"},
			Aliases:     []string{"мурманск", "мурманске", "murrmansk", "murmansk", "кировск"},
		},
		{
			Slug: "suzdal", City: "Суздаль", Country: "Россия",
			Tagline: "Малый город с большой историей", Image: "/assets/images/hotel-06.jpg", PriceFrom: 8400,
			Description: "Кремль, деревянные дома и ощущение, что время здесь идёт чуть медленнее.",
			Tags:        []string{"история", "тишина", "ферма"},
			Aliases:     []string{"суздаль", "суздале", "suzdal"},
		},
	}

	hotels := []seedHotel{
		{
			DestinationSlug: "moscow", Slug: "senses-moscow", Name: "The Senses Moscow",
			ShortDescription: "Бутик-отель с тихим садом на крыше и завтраками до полудня.",
			Address:          "Столярный переулок, 6", Image: "/assets/images/hotel-01.jpg",
			Rating: 9.6, Reviews: 1284, Price: 18400, Featured: true,
			Tags:      []string{"tishina", "dizayn", "zavtraki", "spa", "parkovka", "shumoizolyaciya", "kofe", "metro"},
			Amenities: []string{"Wi-Fi 6E", "SPA", "Ресторан", "Бар", "Фитнес", "Завтрак до 12:00", "Сауна", "Парковка"},
			Meta:      mkMeta(55.761, 37.610, true, true, true, true, false, 10, 3, 5, 0),
		},
		{
			DestinationSlug: "moscow", Slug: "kvartal-47", Name: "Kvartal 47",
			ShortDescription: "Городской отель для кофе, архитектуры и поздних прогулок.",
			Address:          "Малая Бронная, 47", Image: "/assets/images/hotel-04.jpg",
			Rating: 9.2, Reviews: 862, Price: 9800, Featured: true,
			Tags:      []string{"kofe", "dizayn", "metro", "eda", "torgovye-centry", "dlya-dvoih"},
			Amenities: []string{"Wi-Fi", "Кофейня", "Бар", "Библиотека", "Терраса", "Завтрак"},
			Meta:      mkMeta(55.763, 37.594, false, false, true, false, false, 8, 2, 3, 0),
		},
		{
			DestinationSlug: "moscow", Slug: "patriarshy-house", Name: "Patriarshy House",
			ShortDescription: "Тихие номера в двух шагах от Патриарших и большие завтраки.",
			Address:          "Малая Бронная, 18", Image: "/assets/images/hotel-02.jpg",
			Rating: 9.4, Reviews: 731, Price: 15200,
			Tags: []string{"tishina", "dizayn", "zavtraki", "metro", "shumoizolyaciya", "dlya-dvoih"},
			Meta: mkMeta(55.764, 37.592, true, false, true, true, false, 12, 2, 4, 0),
		},
		{
			DestinationSlug: "moscow", Slug: "gorky-loft", Name: "Gorky Loft",
			ShortDescription: "Лофт с кофейней на первом этаже и парком через дорогу.",
			Address:          "Крымский Вал, 9", Image: "/assets/images/hotel-08.jpg",
			Rating: 9.0, Reviews: 489, Price: 8800,
			Tags: []string{"kofe", "dizayn", "eda", "metro", "budget", "parki"},
			Meta: mkMeta(55.729, 37.601, false, false, false, false, false, 18, 3, 7, 0),
		},
		{
			DestinationSlug: "moscow", Slug: "romanov-court", Name: "Romanov Court",
			ShortDescription: "Особняк с историей, театральной программой и спа.",
			Address:          "Пречистенка, 24", Image: "/assets/images/hotel-03.jpg",
			Rating: 9.3, Reviews: 612, Price: 22400,
			Tags: []string{"istoriya", "istoricheskoe-zdanie", "spa", "teatraly", "dostoprimechatelnosti", "kosmos-ceny"},
			Meta: mkMeta(55.739, 37.593, true, true, true, true, false, 5, 4, 8, 0),
		},
		{
			DestinationSlug: "saint-petersburg", Slug: "neva-atelier", Name: "NEVA Atelier",
			ShortDescription: "Светлый лофт-отель с витражами и видом на каналы.",
			Address:          "Набережная реки Мойки, 38", Image: "/assets/images/hotel-05.jpg",
			Rating: 9.4, Reviews: 1106, Price: 13600, Featured: true,
			Tags:      []string{"istoriya", "dizayn", "reki", "teatraly", "dostoprimechatelnosti", "kofe"},
			Amenities: []string{"Wi-Fi", "Завтрак", "Бар", "Библиотека", "Прокат велосипедов", "Консьерж"},
			Meta:      mkMeta(59.932, 30.318, true, true, false, false, false, 9, 3, 8, 0),
		},
		{
			DestinationSlug: "saint-petersburg", Slug: "moyka-house", Name: "Двор на Мойке",
			ShortDescription: "Камерный отель в тихом дворе с историей и лучшими завтраками.",
			Address:          "Набережная реки Мойки, 62", Image: "/assets/images/hotel-06.jpg",
			Rating: 9.7, Reviews: 648, Price: 16200,
			Tags:      []string{"tishina", "istoriya", "zavtraki", "shumoizolyaciya", "dlya-dvoih", "parki"},
			Amenities: []string{"Wi-Fi", "Завтрак", "Сад", "Чайная", "Консьерж", "Трансфер"},
			Meta:      mkMeta(59.929, 30.330, true, false, true, false, false, 7, 4, 10, 0),
		},
		{
			DestinationSlug: "saint-petersburg", Slug: "fontanka-12", Name: "Fontanka 12",
			ShortDescription: "Доходный дом на Фонтанке с высокими окнами и театральной афишей.",
			Address:          "Набережная Фонтанки, 12", Image: "/assets/images/hotel-07.jpg",
			Rating: 9.1, Reviews: 524, Price: 11900,
			Tags: []string{"istoriya", "reki", "kofe", "teatraly", "metro"},
			Meta: mkMeta(59.937, 30.335, false, true, false, false, false, 11, 3, 6, 0),
		},
		{
			DestinationSlug: "saint-petersburg", Slug: "baltic-boutique", Name: "Baltic Boutique",
			ShortDescription: "Небольшой бутик-отель с идеальной звукоизоляцией и поздними завтраками.",
			Address:          "Гражданская, 8", Image: "/assets/images/hotel-01.jpg",
			Rating: 9.5, Reviews: 402, Price: 17800, Featured: true,
			Tags: []string{"butik", "tishina", "zavtraki", "shumoizolyaciya", "dlya-dvoih", "spa"},
			Meta: mkMeta(59.936, 30.312, true, true, true, false, false, 6, 3, 7, 0),
		},
		{
			DestinationSlug: "saint-petersburg", Slug: "nevsky-grand", Name: "Nevsky Grand",
			ShortDescription: "Просторный отель на Невском для города без машины.",
			Address:          "Невский проспект, 88", Image: "/assets/images/hotel-04.jpg",
			Rating: 8.8, Reviews: 1330, Price: 7600,
			Tags: []string{"budget", "metro", "dostoprimechatelnosti", "eda", "torgovye-centry"},
			Meta: mkMeta(59.932, 30.360, false, false, false, false, false, 20, 2, 2, 0),
		},
		{
			DestinationSlug: "sochi", Slug: "marea-resort", Name: "Marea Resort",
			ShortDescription: "Курортный отель у моря с инфинити-бассейном и спа.",
			Address:          "Приморская набережная, 14", Image: "/assets/images/hotel-11.jpg",
			Rating: 9.3, Reviews: 2311, Price: 21400, Featured: true,
			Tags:      []string{"more", "spa", "bassein", "plyazh", "vid-na-more", "semya"},
			Amenities: []string{"Wi-Fi", "Бассейн", "SPA", "Пляж", "Ресторан", "Бар", "Фитнес", "Парковка"},
			Meta:      mkMeta(43.581, 39.721, true, true, false, true, true, 15, 5, 9, 2),
		},
		{
			DestinationSlug: "sochi", Slug: "cedar-sea", Name: "Cedar & Sea",
			ShortDescription: "Еловый лес, море и семейные апартаменты с кухней.",
			Address:          "Курортный проспект, 108", Image: "/assets/images/hotel-12.jpg",
			Rating: 9.5, Reviews: 876, Price: 16800,
			Tags:      []string{"more", "tishina", "semya", "les", "bassein", "detskaya"},
			Amenities: []string{"Wi-Fi", "Кухня", "Детская комната", "Бассейн", "Пляж", "Парковка", "Завтрак"},
			Meta:      mkMeta(43.596, 39.731, false, false, true, true, true, 12, 6, 10, 7),
		},
		{
			DestinationSlug: "sochi", Slug: "sunset-pier", Name: "Sunset Pier",
			ShortDescription: "Номера с видом на воду и веранда, где встречают закат.",
			Address:          "Черноморская, 3", Image: "/assets/images/hotel-03.jpg",
			Rating: 9.4, Reviews: 1094, Price: 19500, Featured: true,
			Tags: []string{"more", "plyazh", "zakaty", "vid-na-more", "romantika", "restoran"},
			Meta: mkMeta(43.579, 39.718, true, true, false, true, false, 10, 3, 8, 1),
		},
		{
			DestinationSlug: "sochi", Slug: "aqua-family", Name: "Aqua Family",
			ShortDescription: "Семейный отель рядом с аквапарком и детской комнатой.",
			Address:          "Виноградная, 21", Image: "/assets/images/hotel-09.jpg",
			Rating: 9.0, Reviews: 1642, Price: 14200,
			Tags: []string{"semya", "akvapark", "bassein", "detskaya", "plyazh", "parkovka"},
			Meta: mkMeta(43.570, 39.742, false, false, false, true, true, 14, 4, 12, 9),
		},
		{
			DestinationSlug: "kazan", Slug: "kazan-house", Name: "Kazan House",
			ShortDescription: "Современный отель в центре с сильной гастрономической программой.",
			Address:          "Профсоюзная, 12", Image: "/assets/images/hotel-07.jpg",
			Rating: 9.1, Reviews: 512, Price: 10400, Featured: true,
			Tags:      []string{"istoriya", "dizayn", "narodnaya-kuhnya", "dostoprimechatelnosti", "eda", "restoran"},
			Amenities: []string{"Wi-Fi", "Ресторан", "Бар", "Терраса", "Завтрак", "Консьерж"},
			Meta:      mkMeta(55.788, 49.122, false, true, false, false, false, 10, 2, 6, 0),
		},
		{
			DestinationSlug: "kazan", Slug: "volga-loft", Name: "Volga Loft",
			ShortDescription: "Лофт с видом на реку и большими окнами для неспешных выходных.",
			Address:          "Набережная озера Кабан, 4", Image: "/assets/images/hotel-08.jpg",
			Rating: 8.9, Reviews: 389, Price: 8600,
			Tags:      []string{"reki", "dizayn", "budget", "kuhnya", "parkovka", "panorama"},
			Amenities: []string{"Wi-Fi", "Коворкинг", "Кухня", "Парковка", "Завтрак"},
			Meta:      mkMeta(55.779, 49.115, false, false, false, true, false, 18, 4, 9, 0),
		},
		{
			DestinationSlug: "kazan", Slug: "kremlin-spa", Name: "Kremlin SPA",
			ShortDescription: "Отель с бассейном и спа в пяти минутах от Кремля.",
			Address:          "Кремлёвская, 8", Image: "/assets/images/hotel-02.jpg",
			Rating: 9.3, Reviews: 921, Price: 13800,
			Tags: []string{"spa", "bassein", "dostoprimechatelnosti", "otkisaem", "restoran", "parkovka"},
			Meta: mkMeta(55.797, 49.106, true, true, true, true, true, 9, 3, 7, 0),
		},
		{
			DestinationSlug: "kazan", Slug: "tatar-yard", Name: "Tatar Yard",
			ShortDescription: "Двор с национальной кухней и семейными номерами.",
			Address:          "Баумана, 36", Image: "/assets/images/hotel-05.jpg",
			Rating: 9.2, Reviews: 604, Price: 9400,
			Tags: []string{"narodnaya-kuhnya", "eda", "istoriya", "kofe", "semya"},
			Meta: mkMeta(55.790, 49.120, false, true, false, false, false, 13, 2, 5, 0),
		},
		{
			DestinationSlug: "kaliningrad", Slug: "amber-yard", Name: "Amber Yard",
			ShortDescription: "Тёплый отель с внутренним двором и балтийскими завтраками.",
			Address:          "Эпроновская, 11", Image: "/assets/images/hotel-09.jpg",
			Rating: 9.2, Reviews: 443, Price: 11200,
			Tags:      []string{"istoriya", "tishina", "zavtraki", "more", "eda", "parkovka"},
			Amenities: []string{"Wi-Fi", "Завтрак", "Двор", "Бар", "Парковка", "Сауна"},
			Meta:      mkMeta(54.706, 20.510, true, false, true, true, false, 11, 3, 9, 12),
		},
		{
			DestinationSlug: "kaliningrad", Slug: "baltic-dune", Name: "Baltic Dune",
			ShortDescription: "Дюны, сосны и море в десяти минутах от номера.",
			Address:          "Верхнеозёрная, 6", Image: "/assets/images/hotel-12.jpg",
			Rating: 9.4, Reviews: 517, Price: 13100, Featured: true,
			Tags: []string{"more", "plyazh", "tishina", "parkovka", "vid-na-more", "les"},
			Meta: mkMeta(54.720, 20.480, true, true, true, true, false, 8, 5, 11, 6),
		},
		{
			DestinationSlug: "kaliningrad", Slug: "kant-house", Name: "Kant House",
			ShortDescription: "Небольшой отель в историческом доме у острова Канта.",
			Address:          "Канта, 14", Image: "/assets/images/hotel-06.jpg",
			Rating: 9.0, Reviews: 288, Price: 8900,
			Tags: []string{"istoriya", "istoricheskoe-zdanie", "kofe", "dostoprimechatelnosti", "eda"},
			Meta: mkMeta(54.712, 20.508, false, true, false, false, false, 15, 2, 8, 15),
		},
		{
			DestinationSlug: "altai", Slug: "altai-ridge", Name: "Altai Ridge",
			ShortDescription: "Панорамные дома в горах с камином, спа и звёздным небом.",
			Address:          "Чемальский тракт, 24", Image: "/assets/images/hotel-10.jpg",
			Rating: 9.6, Reviews: 327, Price: 15600, Featured: true,
			Tags:      []string{"gory", "les", "zvezdy", "spa", "parkovka", "ekoteh", "zima"},
			Amenities: []string{"Wi-Fi", "Камин", "SPA", "Завтрак", "Терраса", "Трансфер", "Парковка"},
			Meta:      mkMeta(51.402, 86.002, true, true, false, true, false, 10, 8, 0, 0),
		},
		{
			DestinationSlug: "altai", Slug: "cedar-creek", Name: "Cedar Creek",
			ShortDescription: "Дома у реки с панорамными окнами и телескопом.",
			Address:          "Катунская, 3", Image: "/assets/images/hotel-01.jpg",
			Rating: 9.7, Reviews: 214, Price: 17400,
			Tags: []string{"les", "reki", "zvezdy", "tishina", "ekoteh", "spa"},
			Meta: mkMeta(51.395, 85.988, true, true, true, true, false, 7, 10, 0, 0),
		},
		{
			DestinationSlug: "altai", Slug: "farm-retreat", Name: "Farm Retreat",
			ShortDescription: "Ферма с деревянными домами, животными и завтраками из печи.",
			Address:          "Береговая, 7", Image: "/assets/images/hotel-09.jpg",
			Rating: 9.3, Reviews: 176, Price: 12600,
			Tags: []string{"ferma", "les", "semya", "s-zhivotnymi", "zavtraki"},
			Meta: mkMeta(51.410, 86.020, false, true, false, true, false, 12, 12, 0, 0),
		},
		{
			DestinationSlug: "yekaterinburg", Slug: "constructivist", Name: "Constructivist",
			ShortDescription: "Дизайн-отель в памятнике конструктивизма.",
			Address:          "Малышева, 22", Image: "/assets/images/hotel-04.jpg",
			Rating: 9.2, Reviews: 486, Price: 8400,
			Tags: []string{"dizayn", "istoriya", "kofe", "metro", "budget"},
			Meta: mkMeta(56.837, 60.602, false, true, true, false, false, 16, 3, 4, 0),
		},
		{
			DestinationSlug: "yekaterinburg", Slug: "ural-panorama", Name: "Ural Panorama",
			ShortDescription: "Панорамный отель с видом на город и Урал.",
			Address:          "Горького, 45", Image: "/assets/images/hotel-08.jpg",
			Rating: 9.4, Reviews: 703, Price: 11800, Featured: true,
			Tags: []string{"gory", "panorama", "spa", "parkovka", "shumoizolyaciya"},
			Meta: mkMeta(56.830, 60.610, true, true, true, true, false, 9, 4, 6, 0),
		},
		{
			DestinationSlug: "yekaterinburg", Slug: "iset-hotel", Name: "Iset Hotel",
			ShortDescription: "Отель на набережной Исети с кофейней и завтраками.",
			Address:          "Ленина, 5", Image: "/assets/images/hotel-02.jpg",
			Rating: 9.0, Reviews: 391, Price: 7200,
			Tags: []string{"reki", "eda", "kofe", "metro", "skidka"},
			Meta: mkMeta(56.840, 60.615, false, false, false, false, false, 22, 2, 3, 0),
		},
		{
			DestinationSlug: "nizhny-novgorod", Slug: "strelka-house", Name: "Strelka House",
			ShortDescription: "Закаты над Волгой из каждого номера.",
			Address:          "Нижневолжская набережная, 11", Image: "/assets/images/hotel-11.jpg",
			Rating: 9.5, Reviews: 618, Price: 12400, Featured: true,
			Tags: []string{"zakaty", "reki", "dizayn", "zavtraki", "dlya-dvoih", "panorama"},
			Meta: mkMeta(56.330, 44.002, true, true, true, false, false, 8, 3, 7, 0),
		},
		{
			DestinationSlug: "nizhny-novgorod", Slug: "kremlin-view", Name: "Kremlin View",
			ShortDescription: "Семейный отель рядом с кремлём и канатной дорогой.",
			Address:          "Большая Покровская, 18", Image: "/assets/images/hotel-05.jpg",
			Rating: 9.1, Reviews: 844, Price: 9800,
			Tags: []string{"istoriya", "dostoprimechatelnosti", "eda", "parkovka", "semya"},
			Meta: mkMeta(56.324, 44.010, false, true, false, true, false, 14, 2, 5, 0),
		},
		{
			DestinationSlug: "nizhny-novgorod", Slug: "volga-breeze", Name: "Volga Breeze",
			ShortDescription: "Недорогой отель у реки с кофейней на первом этаже.",
			Address:          "Рождественская, 24", Image: "/assets/images/hotel-07.jpg",
			Rating: 8.9, Reviews: 512, Price: 6900,
			Tags: []string{"budget", "reki", "kofe", "metro", "skidka"},
			Meta: mkMeta(56.328, 43.995, false, false, false, false, false, 21, 3, 6, 0),
		},
		{
			DestinationSlug: "irkutsk", Slug: "baikal-gate", Name: "Baikal Gate",
			ShortDescription: "База для поездок на Байкал с гидами и трансфером.",
			Address:          "Байкальская, 30", Image: "/assets/images/hotel-10.jpg",
			Rating: 9.5, Reviews: 472, Price: 13900,
			Tags: []string{"ozera", "les", "priklyucheniya", "ekskursii", "guides", "parkovka"},
			Meta: mkMeta(52.280, 104.280, true, true, false, true, false, 9, 4, 7, 0),
		},
		{
			DestinationSlug: "irkutsk", Slug: "listvyanka-lodge", Name: "Listvyanka Lodge",
			ShortDescription: "Лодж у Байкала с панорамными окнами и баней.",
			Address:          "Седова, 2", Image: "/assets/images/hotel-12.jpg",
			Rating: 9.6, Reviews: 338, Price: 16200, Featured: true,
			Tags: []string{"ozera", "tishina", "zvezdy", "ekoteh", "spa", "les"},
			Meta: mkMeta(51.860, 104.870, true, true, true, true, false, 7, 9, 0, 0),
		},
		{
			DestinationSlug: "irkutsk", Slug: "irkutsk-passage", Name: "Irkutsk Passage",
			ShortDescription: "Городской отель в историческом центре.",
			Address:          "Карла Маркса, 14", Image: "/assets/images/hotel-04.jpg",
			Rating: 8.9, Reviews: 296, Price: 7100,
			Tags: []string{"istoriya", "kofe", "eda", "metro", "budget"},
			Meta: mkMeta(52.289, 104.302, false, false, false, false, false, 19, 2, 5, 0),
		},
		{
			DestinationSlug: "vladivostok", Slug: "ocean-ridge", Name: "Ocean Ridge",
			ShortDescription: "Панорама океана, спа и ресторан на верхнем этаже.",
			Address:          "Светланская, 51", Image: "/assets/images/hotel-11.jpg",
			Rating: 9.6, Reviews: 688, Price: 18600, Featured: true,
			Tags: []string{"more", "vid-na-more", "zakaty", "spa", "restoran", "panorama"},
			Meta: mkMeta(43.118, 131.882, true, true, true, true, true, 8, 3, 6, 4),
		},
		{
			DestinationSlug: "vladivostok", Slug: "golden-horn", Name: "Golden Horn",
			ShortDescription: "Отель над бухтой с завтраками из локальных продуктов.",
			Address:          "Алеутская, 17", Image: "/assets/images/hotel-07.jpg",
			Rating: 9.2, Reviews: 941, Price: 11200,
			Tags: []string{"more", "panorama", "eda", "metro", "skidka", "kofe"},
			Meta: mkMeta(43.116, 131.889, false, true, false, false, false, 16, 2, 4, 8),
		},
		{
			DestinationSlug: "vladivostok", Slug: "russky-breeze", Name: "Russky Breeze",
			ShortDescription: "Семейный отель с бассейном рядом с набережной.",
			Address:          "Набережная Цесаревича, 5", Image: "/assets/images/hotel-09.jpg",
			Rating: 9.0, Reviews: 405, Price: 9600,
			Tags: []string{"plyazh", "semya", "parkovka", "bassein", "akvapark", "more"},
			Meta: mkMeta(43.113, 131.900, false, false, false, true, true, 13, 4, 9, 5),
		},
		{
			DestinationSlug: "murmansk", Slug: "aurora-lodge", Name: "Aurora Lodge",
			ShortDescription: "Охота за северным сиянием с гидом и тёплым спа.",
			Address:          "Полярные Зори, 4", Image: "/assets/images/hotel-10.jpg",
			Rating: 9.6, Reviews: 352, Price: 14900, Featured: true,
			Tags: []string{"polyarnoe-siyanie", "zima", "zvezdy", "tishina", "spa", "guides"},
			Meta: mkMeta(68.960, 33.078, true, true, true, true, false, 6, 5, 0, 0),
		},
		{
			DestinationSlug: "murmansk", Slug: "arctic-pier", Name: "Arctic Pier",
			ShortDescription: "Вид на океан и полярную ночь из панорамных окон.",
			Address:          "Морская, 12", Image: "/assets/images/hotel-03.jpg",
			Rating: 9.2, Reviews: 511, Price: 10800,
			Tags: []string{"more", "zima", "kofe", "restoran", "parkovka", "panorama"},
			Meta: mkMeta(68.955, 33.070, true, false, true, true, false, 12, 3, 8, 3),
		},
		{
			DestinationSlug: "murmansk", Slug: "severny-dom", Name: "Severny Dom",
			ShortDescription: "Деревянный дом с сауной и завтраками у камина.",
			Address:          "Северная, 8", Image: "/assets/images/hotel-06.jpg",
			Rating: 9.0, Reviews: 264, Price: 8700,
			Tags: []string{"zima", "les", "semya", "s-zhivotnymi", "zavtraki", "tishina"},
			Meta: mkMeta(68.970, 33.090, false, true, false, true, false, 15, 6, 0, 0),
		},
		{
			DestinationSlug: "suzdal", Slug: "suzdal-estate", Name: "Suzdal Estate",
			ShortDescription: "Усадьба с яблоневым садом и тихими номерами.",
			Address:          "Кремлёвская, 9", Image: "/assets/images/hotel-06.jpg",
			Rating: 9.7, Reviews: 431, Price: 15800, Featured: true,
			Tags: []string{"istoriya", "istoricheskoe-zdanie", "tishina", "zavtraki", "dlya-dvoih", "parki"},
			Meta: mkMeta(56.418, 40.447, true, true, true, true, false, 7, 5, 0, 0),
		},
		{
			DestinationSlug: "suzdal", Slug: "farm-suzdal", Name: "Farm Suzdal",
			ShortDescription: "Ферма с животными, народной кухней и домами у пруда.",
			Address:          "Пушкарская, 21", Image: "/assets/images/hotel-09.jpg",
			Rating: 9.4, Reviews: 287, Price: 11200,
			Tags: []string{"ferma", "semya", "s-zhivotnymi", "narodnaya-kuhnya", "parkovka", "les"},
			Meta: mkMeta(56.425, 40.440, false, true, false, true, false, 10, 4, 0, 0),
		},
		{
			DestinationSlug: "suzdal", Slug: "kamenka-house", Name: "Kamenka House",
			ShortDescription: "Камерный дом с собственной атмосферой и кофейней.",
			Address:          "Ленина, 14", Image: "/assets/images/hotel-02.jpg",
			Rating: 9.1, Reviews: 198, Price: 8400,
			Tags: []string{"svoya-atmosfera", "istoriya", "kofe", "eda", "skidka", "tishina"},
			Meta: mkMeta(56.421, 40.452, false, false, true, false, false, 18, 3, 0, 0),
		},
	}

	pois := basePOIs()
	pois = append(pois, newCityPOIs()...)
	pois = append(pois, excursionPOIs()...)
	pois = append(pois, attractionPOIs()...)
	pois = applyPOICoords(pois)

	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for index, destination := range destinations {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO destinations
			 (slug, city, country, tagline, description, image, price_from, tags_json, aliases_json, sort_order)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(slug) DO UPDATE SET
			   city = excluded.city, country = excluded.country, tagline = excluded.tagline,
			   description = excluded.description, image = excluded.image,
			   price_from = excluded.price_from, tags_json = excluded.tags_json,
			   aliases_json = excluded.aliases_json, sort_order = excluded.sort_order`,
			destination.Slug, destination.City, destination.Country, destination.Tagline,
			destination.Description, destination.Image, destination.PriceFrom,
			encodeStrings(destination.Tags), encodeStrings(destination.Aliases), index,
		); err != nil {
			return err
		}
	}

	destinationIDs := map[string]int64{}
	rows, err := tx.QueryContext(ctx, `SELECT slug, id FROM destinations`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var slug string
		var id int64
		if err := rows.Scan(&slug, &id); err != nil {
			rows.Close()
			return err
		}
		destinationIDs[slug] = id
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for index, hotel := range hotels {
		gallery := hotel.Gallery
		if len(gallery) == 0 {
			gallery = []string{
				hotel.Image,
				fmt.Sprintf("/assets/images/hotel-%02d.jpg", (index+1)%12+1),
				fmt.Sprintf("/assets/images/hotel-%02d.jpg", (index+5)%12+1),
			}
		}
		meta := hotel.Meta
		meta.TagSlugs = enrichTags(hotel.DestinationSlug, hotel.Tags)
		meta.WeeklyPrice = int(math.Round(float64(hotel.Price)*6.3/100)) * 100
		if meta.WeeklyPrice <= 0 {
			meta.WeeklyPrice = hotel.Price * 6
		}
		meta.ServicesMinutes = meta.MetroMinutes
		if meta.ServicesMinutes == 0 {
			meta.ServicesMinutes = 5 + index%7
		} else {
			meta.ServicesMinutes += 2
		}
		if meta.ParkMinutes == 0 {
			meta.ParkMinutes = 4 + index%6
		}
		switch {
		case hotel.Price >= 18000:
			meta.Stars = 5
		case hotel.Price >= 11000:
			meta.Stars = 4
		default:
			meta.Stars = 3
		}
		meta.Winter = containsString(meta.TagSlugs, "zima")
		meta.NorthernLights = containsString(meta.TagSlugs, "polyarnoe-siyanie")
		amenities := hotel.Amenities
		if len(amenities) == 0 {
			amenities = amenitiesFor(meta.TagSlugs, meta)
		}
		description := hotel.Description
		if description == "" {
			description = hotel.ShortDescription + " До главных точек города — короткая прогулка, а команда отеля подскажет места, куда не доходят путеводители."
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO hotels
			 (destination_id, slug, name, short_description, description, address, image, gallery_json,
			  rating, reviews_count, price_per_night, vibe_tags_json, amenities_json, featured, sort_order, meta_json)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(slug) DO UPDATE SET
			   destination_id = excluded.destination_id, name = excluded.name,
			   short_description = excluded.short_description, description = excluded.description,
			   address = excluded.address, image = excluded.image, gallery_json = excluded.gallery_json,
			   rating = excluded.rating, reviews_count = excluded.reviews_count,
			   price_per_night = excluded.price_per_night, vibe_tags_json = excluded.vibe_tags_json,
			   amenities_json = excluded.amenities_json, featured = excluded.featured,
			   sort_order = excluded.sort_order, meta_json = excluded.meta_json`,
			destinationIDs[hotel.DestinationSlug], hotel.Slug, hotel.Name, hotel.ShortDescription,
			description, hotel.Address, hotel.Image, encodeStrings(gallery),
			hotel.Rating, hotel.Reviews, hotel.Price, encodeStrings(TagsToLabels(hotel.Tags)),
			encodeStrings(amenities), boolInt(hotel.Featured), index, encodeHotelMeta(meta),
		); err != nil {
			return err
		}
	}

	hotelRows, err := tx.QueryContext(ctx, `SELECT slug, id, price_per_night FROM hotels`)
	if err != nil {
		return err
	}
	type hotelRef struct {
		id    int64
		price int
	}
	hotelRefs := map[string]hotelRef{}
	for hotelRows.Next() {
		var slug string
		var ref hotelRef
		if err := hotelRows.Scan(&slug, &ref.id, &ref.price); err != nil {
			hotelRows.Close()
			return err
		}
		hotelRefs[slug] = ref
	}
	if err := hotelRows.Close(); err != nil {
		return err
	}

	for index, hotel := range hotels {
		ref := hotelRefs[hotel.Slug]
		var roomCount int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM rooms WHERE hotel_id = ?`, ref.id).Scan(&roomCount); err != nil {
			return err
		}
		if roomCount > 0 {
			continue
		}
		gallery := hotel.Gallery
		if len(gallery) == 0 {
			gallery = []string{hotel.Image}
		}
		if err := insertRooms(ctx, tx, ref.id, ref.price, gallery); err != nil {
			return err
		}
		_ = index
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM pois`); err != nil {
		return err
	}
	for index, poi := range pois {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO pois
			 (destination_slug, name, category, description, address, distance, minutes, tags_json, price_level, sort_order, lat, lng, schematic)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			poi.DestinationSlug, poi.Name, poi.Category, poi.Description,
			poi.Address, poi.Distance, poi.Minutes, encodeStrings(poi.Tags), poi.PriceLevel, index,
			poi.Lat, poi.Lng, boolInt(poi.Schematic),
		); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO seed_meta (key, value) VALUES ('content_version', ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		seedContentVersion,
	); err != nil {
		return err
	}

	return tx.Commit()
}

func enrichTags(destination string, tags []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(tags)+4)
	add := func(slug string) {
		if slug == "" || seen[slug] {
			return
		}
		if _, ok := tagBySlug[slug]; !ok {
			return
		}
		seen[slug] = true
		result = append(result, slug)
	}
	for _, tag := range tags {
		add(tag)
	}

	has := func(slug string) bool { return seen[slug] }
	if destination == "sochi" || destination == "vladivostok" {
		add("ne-zima")
	}
	if has("parki") || has("dostoprimechatelnosti") || has("istoriya") || has("reki") || destination == "suzdal" || destination == "kaliningrad" {
		add("na-pogulyat")
	}
	if has("zvezdy") || has("polyarnoe-siyanie") || has("zakaty") || has("panorama") {
		add("nebo")
	}
	if has("metro") || has("eda") || has("torgovye-centry") || has("kofe") {
		add("servisy")
	}
	if destination == "altai" || destination == "murmansk" {
		add("zima")
	}
	return result
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func amenitiesFor(tags []string, meta HotelMeta) []string {
	have := map[string]bool{}
	for _, tag := range tags {
		have[tag] = true
	}
	amenities := []string{"Wi-Fi", "Завтрак", "Консьерж"}
	add := func(condition bool, value string) {
		if condition {
			amenities = append(amenities, value)
		}
	}
	add(have["bassein"], "Бассейн")
	add(have["spa"], "SPA")
	add(have["restoran"], "Ресторан")
	add(have["bar"], "Бар")
	add(have["kuhnya"], "Кухня")
	add(have["detskaya"], "Детская комната")
	add(have["parkovka"], "Парковка")
	add(have["guides"], "Услуги гида")
	add(have["transfer"], "Трансфер")
	add(have["torgovye-centry"], "Рядом ТЦ")
	add(have["s-zhivotnymi"], "Можно с животными")
	return amenities
}

func basePOIs() []seedPOI {
	return []seedPOI{
		{DestinationSlug: "moscow", Name: "Kilo Coffee", Category: "coffee", Description: "Тихая спешелти-кофейня с фильтром, сезонным меню и столиками у окна. До 11:00 здесь почти нет туристов.", Address: "Столярный переулок, 6", Distance: "6 мин пешком", Minutes: 6, Tags: []string{"кофе", "тишина"}, PriceLevel: 2},
		{DestinationSlug: "moscow", Name: "Кооператив «Чёрный»", Category: "coffee", Description: "Локальная обжарка, большие окна и места у подоконника.", Address: "Никитский бульвар, 12", Distance: "12 мин пешком", Minutes: 12, Tags: []string{"кофе", "обжарка"}, PriceLevel: 2},
		{DestinationSlug: "moscow", Name: "Северяне", Category: "food", Description: "Сезонная русская кухня и спокойный свет. Бронируйте стол у окна.", Address: "Столешников переулок, 7", Distance: "8 мин пешком", Minutes: 8, Tags: []string{"ужин", "локальная кухня"}, PriceLevel: 3},
		{DestinationSlug: "moscow", Name: "Парк Горького", Category: "walk", Description: "Прогулка вдоль Москвы-реки, сад и длинная набережная.", Address: "Крымский Вал, 9", Distance: "14 мин на такси", Minutes: 14, Tags: []string{"прогулка", "парк"}, PriceLevel: 1},
		{DestinationSlug: "saint-petersburg", Name: "Coffee Room", Category: "coffee", Description: "Классическая петербургская кофейня с собственной обжаркой.", Address: "Невский проспект, 88", Distance: "5 мин пешком", Minutes: 5, Tags: []string{"кофе", "тишина"}, PriceLevel: 2},
		{DestinationSlug: "saint-petersburg", Name: "Летний сад", Category: "walk", Description: "Тихая прогулка среди старых деревьев и скульптур.", Address: "Летний сад", Distance: "11 мин пешком", Minutes: 11, Tags: []string{"прогулка", "парк"}, PriceLevel: 1},
		{DestinationSlug: "saint-petersburg", Name: "Эрмитаж", Category: "culture", Description: "Главный музей города. Начните с залов на втором этаже.", Address: "Дворцовая площадь, 2", Distance: "12 мин пешком", Minutes: 12, Tags: []string{"музей", "культура"}, PriceLevel: 2},
		{DestinationSlug: "saint-petersburg", Name: "Больше кофе", Category: "coffee", Description: "Небольшая кофейня у канала с фильтром и каскарой.", Address: "Набережная реки Мойки, 42", Distance: "7 мин пешком", Minutes: 7, Tags: []string{"кофе", "спешелти"}, PriceLevel: 2},
		{DestinationSlug: "sochi", Name: "Кофе и море", Category: "coffee", Description: "Маленькая кофейня на набережной с видом на воду.", Address: "Приморская набережная, 8", Distance: "3 мин пешком", Minutes: 3, Tags: []string{"кофе", "море"}, PriceLevel: 2},
		{DestinationSlug: "sochi", Name: "Синий сад", Category: "food", Description: "Сезонные морепродукты, овощи с огня и длинные завтраки.", Address: "Черноморская, 3", Distance: "8 мин пешком", Minutes: 8, Tags: []string{"завтрак", "морепродукты"}, PriceLevel: 3},
		{DestinationSlug: "sochi", Name: "Приморская набережная", Category: "walk", Description: "Шесть километров у моря: пальмы, велодорожка и закаты.", Address: "Приморская набережная", Distance: "2 мин пешком", Minutes: 2, Tags: []string{"море", "закат"}, PriceLevel: 1},
		{DestinationSlug: "sochi", Name: "Скайпарк", Category: "culture", Description: "Смотровая площадка и мосты над ущельем.", Address: "Красная Поляна", Distance: "35 мин на такси", Minutes: 35, Tags: []string{"активность", "горы"}, PriceLevel: 3},
		{DestinationSlug: "kazan", Name: "Big Coffee", Category: "coffee", Description: "Просторная кофейня в центре с обжаркой на месте.", Address: "Профсоюзная, 15", Distance: "4 мин пешком", Minutes: 4, Tags: []string{"кофе", "работа"}, PriceLevel: 2},
		{DestinationSlug: "kazan", Name: "Артель", Category: "food", Description: "Современная татарская кухня: эчпочмак, кыстыбый и локальные продукты.", Address: "Баумана, 36", Distance: "9 мин пешком", Minutes: 9, Tags: []string{"гастрономия", "локальная кухня"}, PriceLevel: 2},
		{DestinationSlug: "kazan", Name: "Набережная Кабана", Category: "walk", Description: "Вода, огни и длинная прогулочная линия.", Address: "Озеро Кабан", Distance: "6 мин пешком", Minutes: 6, Tags: []string{"прогулка", "вид"}, PriceLevel: 1},
		{DestinationSlug: "kazan", Name: "Казанский Кремль", Category: "culture", Description: "Белая стена, мечеть и музеи внутри.", Address: "Кремль", Distance: "12 мин пешком", Minutes: 12, Tags: []string{"культура", "история"}, PriceLevel: 1},
		{DestinationSlug: "kaliningrad", Name: "Coffee Cube", Category: "coffee", Description: "Минималистичная кофейня с очень тихим внутренним двориком.", Address: "Черняховского, 6", Distance: "5 мин пешком", Minutes: 5, Tags: []string{"кофе", "тишина"}, PriceLevel: 2},
		{DestinationSlug: "kaliningrad", Name: "Балтийский завтрак", Category: "food", Description: "Копчёная рыба, сыры и свежий хлеб.", Address: "Эпроновская, 8", Distance: "4 мин пешком", Minutes: 4, Tags: []string{"завтрак", "балтика"}, PriceLevel: 2},
		{DestinationSlug: "kaliningrad", Name: "Рыбная деревня", Category: "walk", Description: "Набережная с видом на реку и городские крыши.", Address: "Рыбная деревня", Distance: "10 мин пешком", Minutes: 10, Tags: []string{"прогулка", "река"}, PriceLevel: 1},
		{DestinationSlug: "kaliningrad", Name: "Кафедральный собор", Category: "culture", Description: "Главный символ города на острове Канта.", Address: "Остров Канта", Distance: "9 мин пешком", Minutes: 9, Tags: []string{"культура", "история"}, PriceLevel: 1},
		{DestinationSlug: "altai", Name: "Кофе у гор", Category: "coffee", Description: "Небольшая кофейня с видом на хребет и печью.", Address: "Чемальский тракт, 18", Distance: "4 мин пешком", Minutes: 4, Tags: []string{"кофе", "горы"}, PriceLevel: 2},
		{DestinationSlug: "altai", Name: "Алтайская кухня", Category: "food", Description: "Мясо на огне, кедровые орехи и локальные травы.", Address: "Чемальский тракт, 22", Distance: "6 мин пешком", Minutes: 6, Tags: []string{"ужин", "локальная кухня"}, PriceLevel: 2},
		{DestinationSlug: "altai", Name: "Тропа здоровья", Category: "walk", Description: "Лёгкий маршрут вдоль реки со смотровыми площадками.", Address: "Чемальский район", Distance: "8 мин пешком", Minutes: 8, Tags: []string{"прогулка", "горы"}, PriceLevel: 1},
		{DestinationSlug: "altai", Name: "Спа у реки", Category: "spa", Description: "Массаж с кедровым маслом и баня с видом на воду.", Address: "Чемальский тракт, 30", Distance: "7 мин пешком", Minutes: 7, Tags: []string{"спа", "баня"}, PriceLevel: 3},
	}
}

func newCityPOIs() []seedPOI {
	return []seedPOI{
		{DestinationSlug: "yekaterinburg", Name: "Coffee Project", Category: "coffee", Description: "Локальная обжарка в цехе с бетонными стенами.", Address: "Малышева, 8", Distance: "4 мин пешком", Minutes: 4, Tags: []string{"кофе", "дизайн"}, PriceLevel: 2},
		{DestinationSlug: "yekaterinburg", Name: "Шарташ", Category: "walk", Description: "Озеро, сосны и длинная прогулочная тропа.", Address: "Озеро Шарташ", Distance: "18 мин на такси", Minutes: 18, Tags: []string{"озеро", "парк"}, PriceLevel: 1},
		{DestinationSlug: "yekaterinburg", Name: "Ельцин Центр", Category: "culture", Description: "Музей, галереи и городская площадь.", Address: "Ельцина, 3", Distance: "9 мин пешком", Minutes: 9, Tags: []string{"музей", "история"}, PriceLevel: 2},
		{DestinationSlug: "yekaterinburg", Name: "Уральские пельмени", Category: "food", Description: "Локальная кухня и большие порции.", Address: "Ленина, 24", Distance: "6 мин пешком", Minutes: 6, Tags: []string{"народная кухня"}, PriceLevel: 2},
		{DestinationSlug: "nizhny-novgorod", Name: "Стрелка", Category: "walk", Description: "Слияние Оки и Волги, лучшие закаты города.", Address: "Стрелка", Distance: "7 мин пешком", Minutes: 7, Tags: []string{"закат", "река"}, PriceLevel: 1},
		{DestinationSlug: "nizhny-novgorod", Name: "Кофе и точка", Category: "coffee", Description: "Маленькая кофейня с фильтром и видом на кремль.", Address: "Рождественская, 8", Distance: "5 мин пешком", Minutes: 5, Tags: []string{"кофе", "вид"}, PriceLevel: 2},
		{DestinationSlug: "nizhny-novgorod", Name: "Нижегородский кремль", Category: "culture", Description: "Стены, башни и панорама города.", Address: "Кремль", Distance: "10 мин пешком", Minutes: 10, Tags: []string{"история", "культура"}, PriceLevel: 1},
		{DestinationSlug: "nizhny-novgorod", Name: "Пельменная", Category: "food", Description: "Народная кухня и домашние пельмени.", Address: "Большая Покровская, 30", Distance: "4 мин пешком", Minutes: 4, Tags: []string{"народная кухня"}, PriceLevel: 1},
		{DestinationSlug: "irkutsk", Name: "Байкал рядом", Category: "excursion", Description: "Однодневная поездка к озеру с гидом.", Address: "Листвянка", Distance: "60 мин на такси", Minutes: 60, Tags: []string{"байкал", "экскурсия"}, PriceLevel: 3},
		{DestinationSlug: "irkutsk", Name: "Кофейня «Молоко»", Category: "coffee", Description: "Тихая кофейня в деревянном доме.", Address: "Карла Маркса, 6", Distance: "3 мин пешком", Minutes: 3, Tags: []string{"кофе", "тишина"}, PriceLevel: 2},
		{DestinationSlug: "irkutsk", Name: "130-й квартал", Category: "walk", Description: "Деревянные кружева, рестораны и прогулочная улица.", Address: "130-й квартал", Distance: "8 мин пешком", Minutes: 8, Tags: []string{"прогулка", "история"}, PriceLevel: 1},
		{DestinationSlug: "irkutsk", Name: "Бурятская кухня", Category: "food", Description: "Позы, буузы и локальные травы.", Address: "Седова, 12", Distance: "7 мин пешком", Minutes: 7, Tags: []string{"народная кухня"}, PriceLevel: 2},
		{DestinationSlug: "vladivostok", Name: "Токаревский маяк", Category: "walk", Description: "Коса, море и самый фотогеничный закат.", Address: "Токаревский маяк", Distance: "14 мин на такси", Minutes: 14, Tags: []string{"море", "закат"}, PriceLevel: 1},
		{DestinationSlug: "vladivostok", Name: "Five O'Clock", Category: "coffee", Description: "Кофейня с видом на бухту и локальной обжаркой.", Address: "Светланская, 44", Distance: "4 мин пешком", Minutes: 4, Tags: []string{"кофе", "вид"}, PriceLevel: 2},
		{DestinationSlug: "vladivostok", Name: "Морской вокзал", Category: "culture", Description: "Отправная точка морских прогулок.", Address: "Нижнепортовая, 1", Distance: "9 мин пешком", Minutes: 9, Tags: []string{"море", "прогулка"}, PriceLevel: 1},
		{DestinationSlug: "vladivostok", Name: "Дальневосточная кухня", Category: "food", Description: "Гребешок, краб и локальные морепродукты.", Address: "Алеутская, 20", Distance: "5 мин пешком", Minutes: 5, Tags: []string{"морепродукты", "ужин"}, PriceLevel: 3},
		{DestinationSlug: "murmansk", Name: "Северное сияние", Category: "excursion", Description: "Ночная охота за сиянием с гидом и фотографом.", Address: "Мурманская область", Distance: "90 мин на такси", Minutes: 90, Tags: []string{"сияние", "экскурсия"}, PriceLevel: 3},
		{DestinationSlug: "murmansk", Name: "Териберка", Category: "walk", Description: "Северный океан, скалы и деревянные дома.", Address: "Териберка", Distance: "120 мин на такси", Minutes: 120, Tags: []string{"море", "природа"}, PriceLevel: 2},
		{DestinationSlug: "murmansk", Name: "Кофе на Полярной", Category: "coffee", Description: "Тёплая кофейня в полярную ночь.", Address: "Полярные Зори, 8", Distance: "3 мин пешком", Minutes: 3, Tags: []string{"кофе", "тишина"}, PriceLevel: 2},
		{DestinationSlug: "murmansk", Name: "Арктическая кухня", Category: "food", Description: "Оленина, рыба и северные ягоды.", Address: "Ленина, 20", Distance: "6 мин пешком", Minutes: 6, Tags: []string{"народная кухня"}, PriceLevel: 3},
		{DestinationSlug: "suzdal", Name: "Спасо-Евфимиев монастырь", Category: "culture", Description: "Белые стены, колокольные звоны и тишина.", Address: "Спасо-Евфимиев монастырь", Distance: "11 мин пешком", Minutes: 11, Tags: []string{"история", "культура"}, PriceLevel: 2},
		{DestinationSlug: "suzdal", Name: "Торговая площадь", Category: "walk", Description: "Медовые ряды, травяной чай и прогулка вдоль Каменки.", Address: "Торговая площадь", Distance: "6 мин пешком", Minutes: 6, Tags: []string{"прогулка", "история"}, PriceLevel: 1},
		{DestinationSlug: "suzdal", Name: "Кофейня на Ленина", Category: "coffee", Description: "Домашняя выпечка и фильтр в старом доме.", Address: "Ленина, 5", Distance: "4 мин пешком", Minutes: 4, Tags: []string{"кофе", "тишина"}, PriceLevel: 2},
		{DestinationSlug: "suzdal", Name: "Фермерский рынок", Category: "food", Description: "Сыры, мёд и локальные продукты.", Address: "Торговая площадь, 1", Distance: "5 мин пешком", Minutes: 5, Tags: []string{"ферма", "локальная кухня"}, PriceLevel: 1},
	}
}

func excursionPOIs() []seedPOI {
	type excursion struct {
		City        string
		Name        string
		Description string
		Distance    string
		Minutes     int
	}
	list := []excursion{
		{"moscow", "Обзорная прогулка по центру", "Маршрут по переулкам, дворам и смотровым точкам с местным гидом.", "20 мин пешком", 20},
		{"moscow", "Гастротур по локальным местам", "Пять остановок: кофе, рынок, пекарня и два ресторана.", "15 мин пешком", 15},
		{"saint-petersburg", "Крыши и дворы Петербурга", "Прогулка по крышам с видом на каналы и Исаакий.", "18 мин пешком", 18},
		{"saint-petersburg", "Ночная прогулка по каналам", "Разводные мосты и вода в свете фонарей.", "25 мин пешком", 25},
		{"sochi", "Закат в горах", "Подъём к смотровой площадке и встреча заката.", "40 мин на такси", 40},
		{"sochi", "Морская прогулка", "Катер вдоль берега с остановкой для купания.", "12 мин пешком", 12},
		{"kazan", "Гастрономический маршрут", "Эчпочмак, кыстыбый и современная татарская кухня.", "10 мин пешком", 10},
		{"kazan", "Кремль и Старо-Татарская слобода", "История города за два часа с гидом.", "14 мин пешком", 14},
		{"kaliningrad", "Куршская коса", "Дюны, сосны и море в одной поездке.", "60 мин на такси", 60},
		{"kaliningrad", "Город Канта", "Пешеходная прогулка по острову и старым кварталам.", "12 мин пешком", 12},
		{"altai", "Трек к смотровой", "Лёгкий горный маршрут с проводником.", "30 мин пешком", 30},
		{"altai", "Ночное небо с телескопом", "Звёзды, Млечный Путь и горячий чай.", "10 мин пешком", 10},
		{"yekaterinburg", "Конструктивизм", "Маршрут по памятникам авангарда с архитектором.", "16 мин пешком", 16},
		{"nizhny-novgorod", "Закат на Стрелке", "Прогулка к слиянию Оки и Волги.", "8 мин пешком", 8},
		{"irkutsk", "Байкал за один день", "Листвянка, музей и прогулка у воды.", "60 мин на такси", 60},
		{"vladivostok", "Острова и маяки", "Морская прогулка к Токаревскому маяку.", "20 мин на такси", 20},
		{"murmansk", "Охота за сиянием", "Ночной выезд с гидом и фотографом.", "90 мин на такси", 90},
		{"suzdal", "Суздаль пешком", "Кремль, монастыри и деревянные дома с историком.", "9 мин пешком", 9},
	}
	pois := make([]seedPOI, 0, len(list))
	for _, item := range list {
		pois = append(pois, seedPOI{
			DestinationSlug: item.City,
			Name:            item.Name,
			Category:        "excursion",
			Description:     item.Description,
			Address:         "Центр города",
			Distance:        item.Distance,
			Minutes:         item.Minutes,
			Tags:            []string{"экскурсия", "гид"},
			PriceLevel:      2,
		})
	}
	return pois
}

func insertRooms(ctx context.Context, tx *sql.Tx, hotelID int64, price int, gallery []string) error {
	roomTypes := []struct {
		Name        string
		Description string
		Factor      float64
		Capacity    int
		Bed         string
		Size        int
		Amenities   []string
		Units       int
	}{
		{
			Name: "Studio", Description: "Компактный номер с большой кроватью и продуманным светом.",
			Factor: 1, Capacity: 2, Bed: "King", Size: 28,
			Amenities: []string{"Wi-Fi", "Кондиционер", "Кофе-станция", "Душ"}, Units: 6,
		},
		{
			Name: "Deluxe", Description: "Больше пространства, зона отдыха и вид на город.",
			Factor: 1.35, Capacity: 3, Bed: "King + диван", Size: 40,
			Amenities: []string{"Wi-Fi", "Кондиционер", "Ванна", "Мини-бар", "Вид на город"}, Units: 4,
		},
		{
			Name: "Suite", Description: "Отдельная гостиная, панорамные окна и поздний выезд.",
			Factor: 1.9, Capacity: 4, Bed: "King + sofa bed", Size: 58,
			Amenities: []string{"Wi-Fi", "Гостиная", "Ванна", "Мини-бар", "Late check-out"}, Units: 2,
		},
	}

	for index, room := range roomTypes {
		roomPrice := int(math.Round(float64(price)*room.Factor/100)) * 100
		image := gallery[index%len(gallery)]
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO rooms
			 (hotel_id, name, description, price, capacity, bed, size, amenities_json, image, total_units)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			hotelID, room.Name, room.Description, roomPrice, room.Capacity, room.Bed, room.Size,
			encodeStrings(room.Amenities), image, room.Units,
		); err != nil {
			return err
		}
	}
	return nil
}

func ensureDemoUser(ctx context.Context, store *Store, cfg Config) error {
	user, err := store.FindUserByEmail(ctx, cfg.DemoEmail)
	if errors.Is(err, ErrNotFound) {
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(cfg.DemoPassword), bcrypt.DefaultCost)
		if hashErr != nil {
			return hashErr
		}
		user, err = store.CreateUser(ctx, cfg.DemoName, cfg.DemoEmail, string(hash))
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	if _, err := store.Preferences(ctx, user.ID); err != nil {
		return err
	}
	_ = store.SavePreferences(ctx, user.ID, Preferences{Accent: defaultAccent, MenuTint: defaultMenuTint})

	var bookingCount int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bookings WHERE user_id = ?`, user.ID).Scan(&bookingCount); err != nil {
		return err
	}
	if bookingCount == 0 {
		if err := seedDemoTrips(ctx, store, user); err != nil {
			return err
		}
	}

	var favoriteCount int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM favorites WHERE user_id = ?`, user.ID).Scan(&favoriteCount); err != nil {
		return err
	}
	if favoriteCount == 0 {
		var hotelID int64
		if err := store.db.QueryRowContext(ctx, `SELECT id FROM hotels WHERE slug = 'marea-resort'`).Scan(&hotelID); err == nil {
			_, _ = store.db.ExecContext(ctx,
				`INSERT INTO favorites (user_id, hotel_id, created_at) VALUES (?, ?, ?)`,
				user.ID, hotelID, time.Now().UTC().Format(time.RFC3339),
			)
		}
	}
	return nil
}

func seedDemoTrips(ctx context.Context, store *Store, user User) error {
	type demoBooking struct {
		HotelSlug string
		RoomName  string
		CheckIn   time.Time
		CheckOut  time.Time
		Guests    int
		Code      string
	}
	now := time.Now().UTC()
	bookings := []demoBooking{
		{HotelSlug: "senses-moscow", RoomName: "Deluxe", CheckIn: now.AddDate(0, 0, 20), CheckOut: now.AddDate(0, 0, 23), Guests: 2, Code: "ODN-DEMO1"},
		{HotelSlug: "marea-resort", RoomName: "Studio", CheckIn: now.AddDate(0, -2, 0), CheckOut: now.AddDate(0, -2, 4), Guests: 2, Code: "ODN-DEMO2"},
	}

	for _, booking := range bookings {
		var hotelID, roomID int64
		var price int
		err := store.db.QueryRowContext(ctx,
			`SELECT h.id, r.id, r.price
			 FROM hotels h JOIN rooms r ON r.hotel_id = h.id
			 WHERE h.slug = ? AND r.name = ?`,
			booking.HotelSlug, booking.RoomName,
		).Scan(&hotelID, &roomID, &price)
		if err != nil {
			return fmt.Errorf("seed demo trip: %w", err)
		}
		nights := int(booking.CheckOut.Sub(booking.CheckIn).Hours() / 24)
		_, err = store.db.ExecContext(ctx,
			`INSERT INTO bookings
			 (code, user_id, hotel_id, room_id, check_in, check_out, guests, nights, total, status,
			  guest_name, guest_email, guest_phone, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'confirmed', ?, ?, ?, ?)`,
			booking.Code, user.ID, hotelID, roomID,
			booking.CheckIn.Format("2006-01-02"), booking.CheckOut.Format("2006-01-02"),
			booking.Guests, nights, price*nights, user.Name, user.Email, "+7 999 120-44-08",
			now.AddDate(0, 0, -7).Format(time.RFC3339),
		)
		if err != nil {
			return fmt.Errorf("seed demo trip: %w", err)
		}
	}
	return nil
}

func encodeStrings(values []string) string {
	data, _ := json.Marshal(values)
	return string(data)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

var _ = strconv.Itoa
var _ = strings.TrimSpace
