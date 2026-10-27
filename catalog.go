package main

import (
	"encoding/json"
	"strings"
)

type FacetDef struct {
	Slug  string `json:"slug"`
	Label string `json:"label"`
}

type TagDef struct {
	Slug     string   `json:"slug"`
	Label    string   `json:"label"`
	Facet    string   `json:"facet"`
	Synonyms []string `json:"-"`
}

type MoodDef struct {
	Slug        string         `json:"slug"`
	Title       string         `json:"title"`
	Subtitle    string         `json:"subtitle"`
	Description string         `json:"description"`
	Cover       string         `json:"cover"`
	Weights     map[string]int `json:"-"`
	Tags        []string       `json:"tags"`
	Count       int            `json:"count,omitempty"`
}

type HotelMeta struct {
	Insured        bool     `json:"insured"`
	Guides         bool     `json:"guides"`
	Soundproof     bool     `json:"soundproof"`
	Parking        bool     `json:"parking"`
	Pool           bool     `json:"pool"`
	Kitchen        bool     `json:"kitchen"`
	Farm           bool     `json:"farm"`
	Discount       int      `json:"discount,omitempty"`
	WeeklyPrice    int      `json:"weeklyPrice,omitempty"`
	Lat            float64  `json:"lat"`
	Lng            float64  `json:"lng"`
	FoodMinutes    int      `json:"foodMinutes,omitempty"`
	MetroMinutes   int      `json:"metroMinutes,omitempty"`
	BeachMinutes   int      `json:"beachMinutes,omitempty"`
	ParkMinutes    int      `json:"parkMinutes,omitempty"`
	ServicesMinutes int     `json:"servicesMinutes,omitempty"`
	Stars          int      `json:"stars,omitempty"`
	TagSlugs       []string `json:"tagSlugs"`
	Highlight      string   `json:"highlight,omitempty"`
	Winter         bool     `json:"winter,omitempty"`
	NorthernLights bool     `json:"northernLights,omitempty"`
}

type FacetValue struct {
	Slug  string `json:"slug"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type FacetGroup struct {
	Slug   string       `json:"slug"`
	Label  string       `json:"label"`
	Values []FacetValue `json:"values"`
}

var FacetDefs = []FacetDef{
	{Slug: "mood", Label: "Настроение"},
	{Slug: "nature", Label: "Природа"},
	{Slug: "season", Label: "Сезон"},
	{Slug: "infra", Label: "Инфраструктура"},
	{Slug: "near", Label: "Рядом с отелем"},
	{Slug: "price", Label: "Цена и выгода"},
	{Slug: "feature", Label: "Особенности"},
	{Slug: "access", Label: "Доступность"},
}

var TagCatalog = []TagDef{
	{Slug: "tishina", Label: "Тишина", Facet: "mood", Synonyms: []string{"тих", "тишин", "спокой", "безмолв"}},
	{Slug: "otkisaem", Label: "Откисаем", Facet: "mood", Synonyms: []string{"откис", "отдых", "релакс", "ленив"}},
	{Slug: "zakaty", Label: "Закаты", Facet: "mood", Synonyms: []string{"закат", "заря", "вечерн"}},
	{Slug: "svoya-atmosfera", Label: "Своя атмосфера", Facet: "mood", Synonyms: []string{"атмосфер", "небаналь", "малы город", "камерн"}},
	{Slug: "teatraly", Label: "Для театралов", Facet: "mood", Synonyms: []string{"театр", "балет", "опер", "сцен"}},
	{Slug: "istoriya", Label: "Отели с историей", Facet: "mood", Synonyms: []string{"истори", "старин", "усадьб", "особняк"}},
	{Slug: "dlya-dvoih", Label: "Для двоих", Facet: "mood", Synonyms: []string{"двоих", "пар", "романт", "свидан"}},
	{Slug: "semya", Label: "Семейный отдых", Facet: "mood", Synonyms: []string{"семь", "дет", "ребен", "ребён"}},
	{Slug: "romantika", Label: "Романтика", Facet: "mood", Synonyms: []string{"романт", "медов", "любов"}},
	{Slug: "priklyucheniya", Label: "Приключения", Facet: "mood", Synonyms: []string{"приключ", "актив", "поход", "трек"}},
	{Slug: "kofe", Label: "Кофе", Facet: "mood", Synonyms: []string{"кофе", "латте", "капучино", "эспрессо", "фильтр"}},
	{Slug: "zavtraki", Label: "Завтраки", Facet: "mood", Synonyms: []string{"завтрак", "бранч", "утр"}},
	{Slug: "na-pogulyat", Label: "На погулять", Facet: "mood", Synonyms: []string{"погул", "гулять", "прогулк", "пешком", "city walk"}},

	{Slug: "gory", Label: "Горы", Facet: "nature", Synonyms: []string{"гор", "хребет", "вершин"}},
	{Slug: "reki", Label: "Реки", Facet: "nature", Synonyms: []string{"рек", "набереж", "канал"}},
	{Slug: "ozera", Label: "Озёра", Facet: "nature", Synonyms: []string{"озер", "озёр", "водоем"}},
	{Slug: "les", Label: "В лесу", Facet: "nature", Synonyms: []string{"лес", "елов", "соснов", "бор"}},
	{Slug: "parki", Label: "Парки", Facet: "nature", Synonyms: []string{"парк", "сад", "сквер"}},
	{Slug: "more", Label: "Море", Facet: "nature", Synonyms: []string{"мор", "океан", "побереж"}},
	{Slug: "plyazh", Label: "Пляж", Facet: "nature", Synonyms: []string{"пляж", "берег", "песок"}},
	{Slug: "polyarnoe-siyanie", Label: "Полярное сияние", Facet: "nature", Synonyms: []string{"сияни", "полярн", "аврор"}},
	{Slug: "zvezdy", Label: "Звёздное небо", Facet: "nature", Synonyms: []string{"звезд", "звёзд", "небо", "телескоп"}},
	{Slug: "nebo", Label: "Небо и сияние", Facet: "nature", Synonyms: []string{"небо", "сияни", "аврор", "закат", "звезд"}},

	{Slug: "zima", Label: "Зима есть", Facet: "season", Synonyms: []string{"зим", "лыж", "снег", "каток"}},
	{Slug: "ne-zima", Label: "Зимы нет", Facet: "season", Synonyms: []string{"тепл", "лето", "круглый год", "без зимы", "не зима"}},
	{Slug: "belye-nochi", Label: "Белые ночи", Facet: "season", Synonyms: []string{"белые ночи", "белых ноч"}},

	{Slug: "bassein", Label: "Бассейн", Facet: "infra", Synonyms: []string{"бассейн", "плава"}},
	{Slug: "spa", Label: "SPA", Facet: "infra", Synonyms: []string{"спа", "spa", "массаж", "баня", "саун"}},
	{Slug: "parkovka", Label: "Парковка", Facet: "infra", Synonyms: []string{"парков", "стоянк", "авто"}},
	{Slug: "guides", Label: "Услуги гидов", Facet: "infra", Synonyms: []string{"гид", "экскурсовод", "проводник"}},
	{Slug: "transfer", Label: "Трансфер", Facet: "infra", Synonyms: []string{"трансфер", "аэропорт"}},
	{Slug: "kuhnya", Label: "Кухня в номере", Facet: "infra", Synonyms: []string{"кухн", "апартамент", "готов"}},
	{Slug: "detskaya", Label: "Детская комната", Facet: "infra", Synonyms: []string{"детск", "игров"}},
	{Slug: "restoran", Label: "Ресторан", Facet: "infra", Synonyms: []string{"ресторан", "ужин", "обед"}},

	{Slug: "eda", Label: "Еда рядом", Facet: "near", Synonyms: []string{"еда", "поесть", "ресторан рядом", "кафе"}},
	{Slug: "servisy", Label: "Магазины и сервисы", Facet: "near", Synonyms: []string{"магазин", "сервис", "аптек", "супермаркет"}},
	{Slug: "metro", Label: "Метро рядом", Facet: "near", Synonyms: []string{"метро", "транспорт"}},
	{Slug: "akvapark", Label: "Рядом с аквапарком", Facet: "near", Synonyms: []string{"аквапарк", "горк"}},
	{Slug: "narodnaya-kuhnya", Label: "Народная кухня", Facet: "near", Synonyms: []string{"народн", "локальн", "национальн", "гастроном"}},
	{Slug: "dostoprimechatelnosti", Label: "Достопримечательности", Facet: "near", Synonyms: []string{"достопримеч", "кремл", "музей", "собор", "дворц"}},
	{Slug: "ekskursii", Label: "Экскурсии рядом", Facet: "near", Synonyms: []string{"экскурс", "тур", "маршрут"}},
	{Slug: "torgovye-centry", Label: "ТЦ рядом", Facet: "near", Synonyms: []string{"тц", "торгов", "шопинг"}},

	{Slug: "skidka", Label: "Скидка", Facet: "price", Synonyms: []string{"скидк", "распродаж", "акци", "дешев"}},
	{Slug: "kosmos-ceny", Label: "Космос-цены", Facet: "price", Synonyms: []string{"космос", "премиум", "люкс"}},
	{Slug: "budget", Label: "Бюджетно", Facet: "price", Synonyms: []string{"бюджет", "эконом", "недорог"}},
	{Slug: "week-exact", Label: "Ровно на неделю", Facet: "price", Synonyms: []string{"неделя", "7 ночей", "на неделю"}},

	{Slug: "shumoizolyaciya", Label: "Шумоизоляция", Facet: "feature", Synonyms: []string{"шумоизол", "звукоизол", "тихие номер"}},
	{Slug: "vid-na-more", Label: "Вид на море", Facet: "feature", Synonyms: []string{"вид на мор", "океан из окна"}},
	{Slug: "panorama", Label: "Панорамные окна", Facet: "feature", Synonyms: []string{"панорам", "окна в пол"}},
	{Slug: "istoricheskoe-zdanie", Label: "Историческое здание", Facet: "feature", Synonyms: []string{"историческ", "старый дом", "особняк", "усадьб"}},
	{Slug: "ferma", Label: "Отпуск на ферме", Facet: "feature", Synonyms: []string{"ферм", "дерев", "сельск"}},
	{Slug: "dizayn", Label: "Дизайн", Facet: "feature", Synonyms: []string{"дизайн", "интерьер", "архитектур"}},
	{Slug: "butik", Label: "Бутик-отель", Facet: "feature", Synonyms: []string{"бутик", "камерн"}},
	{Slug: "ekoteh", Label: "Эко-отель", Facet: "feature", Synonyms: []string{"эко", "зелен", "природ"}},

	{Slug: "geo", Label: "Рядом со мной", Facet: "access", Synonyms: []string{"рядом со мной", "геолокац", "рядом"}},
	{Slug: "s-zhivotnymi", Label: "Можно с животными", Facet: "access", Synonyms: []string{"животн", "собак", "кошк", "питом"}},
	{Slug: "bezbarernaya", Label: "Безбарьерная среда", Facet: "access", Synonyms: []string{"безбарьер", "инвалид", "доступн"}},
}

var tagBySlug = func() map[string]TagDef {
	result := make(map[string]TagDef, len(TagCatalog))
	for _, tag := range TagCatalog {
		result[tag.Slug] = tag
	}
	return result
}()

var MoodCatalog = []MoodDef{
	{
		Slug: "tishina", Title: "Тишина и природа", Subtitle: "Проснуться от света, а не от города",
		Description: "Леса, парки, звукоизоляция и ощущение, что время замедлилось.",
		Cover:       "/assets/images/hotel-10.jpg",
		Tags:        []string{"tishina", "shumoizolyaciya", "les", "parki", "svoya-atmosfera"},
		Weights:     map[string]int{"tishina": 5, "shumoizolyaciya": 4, "les": 3, "parki": 2, "svoya-atmosfera": 2, "gory": 1},
	},
	{
		Slug: "more", Title: "Все оттенки океана", Subtitle: "Море, пляж и закаты",
		Description: "Номера с видом на воду, бассейн и вечерние прогулки по набережной.",
		Cover:       "/assets/images/hotel-11.jpg",
		Tags:        []string{"more", "plyazh", "zakaty", "vid-na-more", "bassein"},
		Weights:     map[string]int{"more": 5, "plyazh": 4, "zakaty": 3, "vid-na-more": 3, "bassein": 2},
	},
	{
		Slug: "zima", Title: "Зимняя сказка", Subtitle: "Снег, горы и сияние",
		Description: "Горнолыжные склоны, тёплый спа и небо, которое светится.",
		Cover:       "/assets/images/altai.jpg",
		Tags:        []string{"zima", "gory", "polyarnoe-siyanie", "spa", "zvezdy"},
		Weights:     map[string]int{"zima": 5, "gory": 3, "polyarnoe-siyanie": 4, "spa": 2, "zvezdy": 2},
	},
	{
		Slug: "istoriya", Title: "Отели с историей", Subtitle: "Жить внутри архитектуры",
		Description: "Особняки, доходные дома и усадьбы с собственной биографией.",
		Cover:       "/assets/images/hotel-06.jpg",
		Tags:        []string{"istoriya", "istoricheskoe-zdanie", "dostoprimechatelnosti", "teatraly"},
		Weights:     map[string]int{"istoriya": 5, "istoricheskoe-zdanie": 4, "dostoprimechatelnosti": 2, "teatraly": 2},
	},
	{
		Slug: "otkisaem", Title: "Откисаем", Subtitle: "Спа, баня и ничего не делать",
		Description: "Массаж, бассейн, поздний завтрак и полное право никуда не спешить.",
		Cover:       "/assets/images/hotel-02.jpg",
		Tags:        []string{"spa", "bassein", "otkisaem", "zavtraki"},
		Weights:     map[string]int{"spa": 5, "bassein": 4, "otkisaem": 4, "zavtraki": 1},
	},
	{
		Slug: "gastro", Title: "Вкус места", Subtitle: "Народная кухня и кофе",
		Description: "Локальные продукты, гастрономические ужины и кофейни без толпы.",
		Cover:       "/assets/images/hotel-07.jpg",
		Tags:        []string{"narodnaya-kuhnya", "eda", "kofe", "zavtraki"},
		Weights:     map[string]int{"narodnaya-kuhnya": 5, "eda": 3, "kofe": 3, "zavtraki": 2, "restoran": 2},
	},
	{
		Slug: "semya", Title: "Семейные каникулы", Subtitle: "Всем составом",
		Description: "Детская комната, аквапарк рядом и номера, где хватает места.",
		Cover:       "/assets/images/hotel-12.jpg",
		Tags:        []string{"semya", "akvapark", "detskaya", "bassein", "plyazh"},
		Weights:     map[string]int{"semya": 5, "akvapark": 4, "detskaya": 3, "bassein": 2, "plyazh": 2},
	},
	{
		Slug: "dvoih", Title: "Только для двоих", Subtitle: "Романтика без пафоса",
		Description: "Камерные номера, закаты и ужины, которые хочется растянуть.",
		Cover:       "/assets/images/hotel-03.jpg",
		Tags:        []string{"dlya-dvoih", "romantika", "zakaty", "spa"},
		Weights:     map[string]int{"dlya-dvoih": 5, "romantika": 4, "zakaty": 3, "spa": 2, "kofe": 1},
	},
	{
		Slug: "nebo", Title: "Ближе к небу", Subtitle: "Звёзды, сияние и закаты",
		Description: "Панорамные окна, телескопы и места, где небо становится главным героем.",
		Cover:       "/assets/images/altai.jpg",
		Tags:        []string{"zvezdy", "polyarnoe-siyanie", "zakaty", "panorama", "nebo"},
		Weights:     map[string]int{"zvezdy": 5, "polyarnoe-siyanie": 5, "zakaty": 3, "panorama": 3, "nebo": 4},
	},
	{
		Slug: "pogulyat", Title: "На погулять", Subtitle: "Малые города и длинные прогулки",
		Description: "Парки, набережные, старые улицы и маршруты, которые не хочется заканчивать.",
		Cover:       "/assets/images/kaliningrad.jpg",
		Tags:        []string{"na-pogulyat", "parki", "dostoprimechatelnosti", "istoriya", "svoya-atmosfera"},
		Weights:     map[string]int{"na-pogulyat": 5, "parki": 4, "dostoprimechatelnosti": 4, "istoriya": 2, "svoya-atmosfera": 3},
	},
}

var moodBySlug = func() map[string]MoodDef {
	result := make(map[string]MoodDef, len(MoodCatalog))
	for _, mood := range MoodCatalog {
		result[mood.Slug] = mood
	}
	return result
}()

func decodeHotelMeta(raw string) HotelMeta {
	var meta HotelMeta
	if strings.TrimSpace(raw) == "" {
		return meta
	}
	_ = json.Unmarshal([]byte(raw), &meta)
	return meta
}

func encodeHotelMeta(meta HotelMeta) string {
	if meta.TagSlugs == nil {
		meta.TagSlugs = []string{}
	}
	data, _ := json.Marshal(meta)
	return string(data)
}

func TagLabel(slug string) string {
	if tag, ok := tagBySlug[slug]; ok {
		return tag.Label
	}
	return slug
}

func TagsToLabels(slugs []string) []string {
	labels := make([]string, 0, len(slugs))
	for _, slug := range slugs {
		labels = append(labels, TagLabel(slug))
	}
	return labels
}

func NormalizeTags(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			slug := strings.ToLower(strings.TrimSpace(part))
			if slug == "" || seen[slug] {
				continue
			}
			if _, ok := tagBySlug[slug]; !ok {
				slug = slugByLabel(slug)
			}
			if slug == "" {
				continue
			}
			if seen[slug] {
				continue
			}
			seen[slug] = true
			result = append(result, slug)
		}
	}
	return result
}

func slugByLabel(value string) string {
	for slug, tag := range tagBySlug {
		if strings.EqualFold(strings.TrimSpace(tag.Label), value) {
			return slug
		}
	}
	return ""
}

func InferTags(text string) []string {
	lower := strings.ToLower(text)
	matched := make([]string, 0)
	seen := map[string]bool{}
	for _, tag := range TagCatalog {
		found := false
		for _, synonym := range tag.Synonyms {
			if stemMatches(lower, synonym) {
				found = true
				break
			}
		}
		if !found && stemMatches(lower, strings.ToLower(tag.Label)) {
			found = true
		}
		if found && !seen[tag.Slug] {
			seen[tag.Slug] = true
			matched = append(matched, tag.Slug)
		}
	}
	return matched
}

func stemMatches(text, stem string) bool {
	stem = strings.ToLower(strings.TrimSpace(stem))
	if stem == "" {
		return false
	}
	if len([]rune(stem)) > 4 {
		return strings.Contains(text, stem)
	}
	for _, token := range strings.FieldsFunc(text, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= 'а' && r <= 'я') && r != 'ё'
	}) {
		if !strings.HasPrefix(token, stem) {
			continue
		}
		switch stem {
		case "гор":
			if strings.HasPrefix(token, "город") || strings.HasPrefix(token, "горн") && !strings.HasPrefix(token, "горнолыж") {
				continue
			}
		case "мор":
			if strings.HasPrefix(token, "мороз") || strings.HasPrefix(token, "морск") {
				continue
			}
		case "рек":
			if strings.HasPrefix(token, "рекоменд") || strings.HasPrefix(token, "рекорд") {
				continue
			}
		case "лес":
			if strings.HasPrefix(token, "лестн") {
				continue
			}
		case "спа":
			if strings.HasPrefix(token, "спальн") {
				continue
			}
		case "тур":
			if strings.HasPrefix(token, "турнир") {
				continue
			}
		}
		return true
	}
	return false
}

func InferMood(text string) string {
	lower := strings.ToLower(text)
	bestSlug := ""
	bestScore := 0
	for _, mood := range MoodCatalog {
		score := 0
		if strings.Contains(lower, strings.ToLower(mood.Title)) {
			score += 5
		}
		for _, slug := range mood.Tags {
			tag := tagBySlug[slug]
			for _, synonym := range tag.Synonyms {
				if stemMatches(lower, synonym) {
					score++
					break
				}
			}
		}
		if score > bestScore {
			bestScore = score
			bestSlug = mood.Slug
		}
	}
	if bestScore < 2 {
		return ""
	}
	return bestSlug
}

func MoodScore(tagSlugs []string, mood MoodDef) int {
	have := make(map[string]bool, len(tagSlugs))
	for _, slug := range tagSlugs {
		have[slug] = true
	}
	totalWeight := 0
	score := 0
	for slug, weight := range mood.Weights {
		totalWeight += weight
		if have[slug] {
			score += weight
		}
	}
	if score == 0 || totalWeight == 0 {
		return 0
	}
	percent := 55 + int(float64(score)/float64(totalWeight)*43)
	if percent > 98 {
		percent = 98
	}
	return percent
}

func buildFacets(hotels []HotelCard) []FacetGroup {
	counts := map[string]int{}
	for _, hotel := range hotels {
		for _, slug := range hotel.Meta.TagSlugs {
			counts[slug]++
		}
		if hotel.Meta.Insured {
			counts["insured"]++
		}
		if hotel.Meta.Guides {
			counts["guides"]++
		}
	}
	groups := make([]FacetGroup, 0, len(FacetDefs))
	for _, facet := range FacetDefs {
		values := make([]FacetValue, 0)
		for _, tag := range TagCatalog {
			if tag.Facet != facet.Slug {
				continue
			}
			count := counts[tag.Slug]
			if count == 0 {
				continue
			}
			values = append(values, FacetValue{Slug: tag.Slug, Label: tag.Label, Count: count})
		}
		if len(values) > 0 {
			groups = append(groups, FacetGroup{Slug: facet.Slug, Label: facet.Label, Values: values})
		}
	}
	return groups
}

func MoodsWithCounts(hotels []HotelCard) []MoodDef {
	moods := make([]MoodDef, 0, len(MoodCatalog))
	for _, mood := range MoodCatalog {
		count := 0
		for _, hotel := range hotels {
			if MoodScore(hotel.Meta.TagSlugs, mood) > 0 {
				count++
			}
		}
		copyMood := mood
		copyMood.Count = count
		moods = append(moods, copyMood)
	}
	return moods
}