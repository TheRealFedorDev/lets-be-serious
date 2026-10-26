package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"golang.org/x/crypto/bcrypt"
)

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
}

type seedPOI struct {
	DestinationSlug string
	Name            string
	Category        string
	Description     string
	Address         string
	Distance        string
	Tags            []string
	PriceLevel      int
}

func seedBaseData(ctx context.Context, store *Store) error {
	var count int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM destinations`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	destinations := []seedDestination{
		{
			Slug: "moscow", City: "Москва", Country: "Россия",
			Tagline:     "Большой город с тихими дворами",
			Description: "Архитектура, кофе и вечерние прогулки — здесь легко переключиться с ритма мегаполиса на свой собственный.",
			Image:       "/assets/images/moscow.jpg", PriceFrom: 9800,
			Tags:    []string{"архитектура", "кофе", "вечерние прогулки"},
			Aliases: []string{"москва", "москве", "москву", "moscow"},
		},
		{
			Slug: "saint-petersburg", City: "Санкт-Петербург", Country: "Россия",
			Tagline:     "Белые ночи и тихие дворы",
			Description: "Витражные фасады, вода и медленные завтраки. Город, в который хочется возвращаться без повода.",
			Image:       "/assets/images/saint-petersburg.jpg", PriceFrom: 13600,
			Tags:    []string{"история", "дизайн", "завтраки"},
			Aliases: []string{"санкт-петербург", "петербург", "петербурге", "питер", "спб", "saint petersburg"},
		},
		{
			Slug: "sochi", City: "Сочи", Country: "Россия",
			Tagline:     "Море, пальмы и медленные завтраки",
			Description: "Утро начинается с моря, день — с гор, вечер — с длинной набережной. Отдых без лишнего расписания.",
			Image:       "/assets/images/sochi.jpg", PriceFrom: 16800,
			Tags:    []string{"море", "спа", "горы"},
			Aliases: []string{"сочи", "sochi"},
		},
		{
			Slug: "kazan", City: "Казань", Country: "Россия",
			Tagline:     "Тысяча лет на одном берегу",
			Description: "Кремль, гастрономия и современная архитектура в одном маршруте. Город, где легко совмещать культуру и отдых.",
			Image:       "/assets/images/kazan.jpg", PriceFrom: 8600,
			Tags:    []string{"гастрономия", "культура", "центр"},
			Aliases: []string{"казань", "казани", "казанью", "kazan"},
		},
		{
			Slug: "kaliningrad", City: "Калининград", Country: "Россия",
			Tagline:     "Балтийский воздух и кирпичная готика",
			Description: "Морской бриз, тихие улицы и лучшие завтраки с видом на воду. Небольшое путешествие с большим послевкусием.",
			Image:       "/assets/images/kaliningrad.jpg", PriceFrom: 11200,
			Tags:    []string{"балтика", "тишина", "завтраки"},
			Aliases: []string{"калининград", "калининграде", "kaliningrad"},
		},
		{
			Slug: "altai", City: "Алтай", Country: "Россия",
			Tagline:     "Горы, тишина и звёзды",
			Description: "Деревянные дома, панорамные окна и воздух, в котором слышно только ветер. Для тех, кто хочет выдохнуть.",
			Image:       "/assets/images/altai.jpg", PriceFrom: 15600,
			Tags:    []string{"горы", "природа", "тишина"},
			Aliases: []string{"алтай", "алтае", "алтаю", "горный алтай", "altai"},
		},
	}

	hotels := []seedHotel{
		{
			DestinationSlug: "moscow", Slug: "senses-moscow", Name: "The Senses Moscow",
			ShortDescription: "Бутик-отель с тихими номерами, садом на крыше и завтраками до полудня.",
			Description:      "Пять минут от Столярного переулка и при этом полная тишина внутри. В номерах — натуральные материалы, продуманный свет и звукоизоляция, которой хватит для глубокого сна. Утром — завтрак в саду на крыше, вечером — бар с локальными виноделами.",
			Address:          "Столярный переулок, 6", Image: "/assets/images/hotel-01.jpg",
			Gallery: []string{"/assets/images/hotel-01.jpg", "/assets/images/hotel-02.jpg", "/assets/images/hotel-03.jpg"},
			Rating:  9.6, Reviews: 1284, Price: 18400,
			Tags:      []string{"дизайн", "тишина", "завтраки"},
			Amenities: []string{"Wi-Fi 6E", "SPA", "Ресторан", "Бар", "Фитнес", "Завтрак до 12:00", "Сауна", "Парковка"},
			Featured:  true,
		},
		{
			DestinationSlug: "moscow", Slug: "kvartal-47", Name: "Kvartal 47",
			ShortDescription: "Городской отель для тех, кто любит кофе, архитектуру и поздние прогулки.",
			Description:      "Дом в тихом квартале между Патриаршими и Тверской. В лобби — кофейня с собственной обжаркой, на верхнем этаже — небольшая библиотека и терраса. Номера компактные, но очень удобные: идеально для пары или короткой городской поездки.",
			Address:          "Малая Бронная, 47", Image: "/assets/images/hotel-04.jpg",
			Gallery: []string{"/assets/images/hotel-04.jpg", "/assets/images/hotel-07.jpg", "/assets/images/hotel-08.jpg"},
			Rating:  9.2, Reviews: 862, Price: 9800,
			Tags:      []string{"центр", "кофе", "для пар"},
			Amenities: []string{"Wi-Fi", "Кофейня", "Бар", "Библиотека", "Терраса", "Завтрак"},
			Featured:  true,
		},
		{
			DestinationSlug: "saint-petersburg", Slug: "neva-atelier", Name: "NEVA Atelier",
			ShortDescription: "Светлый лофт-отель с витражами и видом на каналы.",
			Description:      "Бывший доходный дом, в котором сохранили лестницу, витражи и высокие окна. Номера собраны из натурального дуба, льна и стекла. До Эрмитажа — 12 минут пешком, до ближайшей кофейни — три.",
			Address:          "Набережная реки Мойки, 38", Image: "/assets/images/hotel-05.jpg",
			Gallery: []string{"/assets/images/hotel-05.jpg", "/assets/images/hotel-06.jpg", "/assets/images/hotel-03.jpg"},
			Rating:  9.4, Reviews: 1106, Price: 13600,
			Tags:      []string{"дизайн", "центр", "витражи"},
			Amenities: []string{"Wi-Fi", "Завтрак", "Бар", "Библиотека", "Прокат велосипедов", "Консьерж"},
			Featured:  true,
		},
		{
			DestinationSlug: "saint-petersburg", Slug: "moyka-house", Name: "Двор на Мойке",
			ShortDescription: "Камерный отель в тихом дворе с историей и лучшими завтраками.",
			Description:      "Всего 14 номеров, собственный сад и ощущение, что вы живёте в гостях у очень внимательного петербуржца. Утром подают сырники и сезонные фрукты, вечером — чай с видом на двор-колодец.",
			Address:          "Набережная реки Мойки, 62", Image: "/assets/images/hotel-06.jpg",
			Gallery: []string{"/assets/images/hotel-06.jpg", "/assets/images/hotel-02.jpg", "/assets/images/hotel-09.jpg"},
			Rating:  9.7, Reviews: 648, Price: 16200,
			Tags:      []string{"тишина", "история", "завтраки"},
			Amenities: []string{"Wi-Fi", "Завтрак", "Сад", "Чайная", "Консьерж", "Трансфер"},
			Featured:  false,
		},
		{
			DestinationSlug: "sochi", Slug: "marea-resort", Name: "Marea Resort",
			ShortDescription: "Курортный отель у моря с инфинити-бассейном и спа.",
			Description:      "Тёплый камень, пальмы и панорама моря из каждого номера. В Marea нет суеты: завтрак подают до 12:00, бассейн работает допоздна, а спа-комплекс выходит прямо на набережную.",
			Address:          "Приморская набережная, 14", Image: "/assets/images/hotel-11.jpg",
			Gallery: []string{"/assets/images/hotel-11.jpg", "/assets/images/hotel-12.jpg", "/assets/images/hotel-04.jpg"},
			Rating:  9.3, Reviews: 2311, Price: 21400,
			Tags:      []string{"море", "спа", "бассейн"},
			Amenities: []string{"Wi-Fi", "Бассейн", "SPA", "Пляж", "Ресторан", "Бар", "Фитнес", "Парковка"},
			Featured:  true,
		},
		{
			DestinationSlug: "sochi", Slug: "cedar-sea", Name: "Cedar & Sea",
			ShortDescription: "Еловый лес, море и семейные апартаменты с кухней.",
			Description:      "Отель на склоне между лесом и морем. Большие семейные номера с кухней, детская комната и тихая терраса, где завтрак превращается в полноценное утро. До пляжа — 7 минут по тенистой дороге.",
			Address:          "Курортный проспект, 108", Image: "/assets/images/hotel-12.jpg",
			Gallery: []string{"/assets/images/hotel-12.jpg", "/assets/images/hotel-03.jpg", "/assets/images/hotel-06.jpg"},
			Rating:  9.5, Reviews: 876, Price: 16800,
			Tags:      []string{"море", "тишина", "семья"},
			Amenities: []string{"Wi-Fi", "Кухня", "Детская комната", "Бассейн", "Пляж", "Парковка", "Завтрак"},
			Featured:  false,
		},
		{
			DestinationSlug: "kazan", Slug: "kazan-house", Name: "Kazan House",
			ShortDescription: "Современный отель в центре с сильной гастрономической программой.",
			Description:      "Кирпич, дерево и татарские орнаменты в деталях. На первом этаже — ресторан с локальной кухней и завтраками, на крыше — вид на Кремль. Удобно для первого знакомства с городом.",
			Address:          "Профсоюзная, 12", Image: "/assets/images/hotel-07.jpg",
			Gallery: []string{"/assets/images/hotel-07.jpg", "/assets/images/hotel-08.jpg", "/assets/images/hotel-05.jpg"},
			Rating:  9.1, Reviews: 512, Price: 10400,
			Tags:      []string{"центр", "гастрономия", "дизайн"},
			Amenities: []string{"Wi-Fi", "Ресторан", "Бар", "Терраса", "Завтрак", "Консьерж"},
			Featured:  true,
		},
		{
			DestinationSlug: "kazan", Slug: "volga-loft", Name: "Volga Loft",
			ShortDescription: "Лофт с видом на реку и большими окнами для неспешных выходных.",
			Description:      "Просторные лофты с панорамными окнами и видом на Волгу. Внутри минимум декора и максимум света. Хороший выбор для тех, кто путешествует с ноутбуком и ценит пространство.",
			Address:          "Набережная озера Кабан, 4", Image: "/assets/images/hotel-08.jpg",
			Gallery: []string{"/assets/images/hotel-08.jpg", "/assets/images/hotel-09.jpg", "/assets/images/hotel-04.jpg"},
			Rating:  8.9, Reviews: 389, Price: 8600,
			Tags:      []string{"лофт", "вид на реку", "бюджет"},
			Amenities: []string{"Wi-Fi", "Коворкинг", "Кухня", "Парковка", "Завтрак"},
			Featured:  false,
		},
		{
			DestinationSlug: "kaliningrad", Slug: "amber-yard", Name: "Amber Yard",
			ShortDescription: "Тёплый отель с внутренним двором и балтийскими завтраками.",
			Description:      "Кирпичный дом с собственным двором и большой коллекцией современного искусства. Утром — балтийские сыры, копчёная рыба и свежий хлеб. До Рыбной деревни — 10 минут пешком.",
			Address:          "Эпроновская, 11", Image: "/assets/images/hotel-09.jpg",
			Gallery: []string{"/assets/images/hotel-09.jpg", "/assets/images/hotel-10.jpg", "/assets/images/hotel-02.jpg"},
			Rating:  9.2, Reviews: 443, Price: 11200,
			Tags:      []string{"балтика", "тишина", "завтраки"},
			Amenities: []string{"Wi-Fi", "Завтрак", "Двор", "Бар", "Парковка", "Сауна"},
			Featured:  false,
		},
		{
			DestinationSlug: "altai", Slug: "altai-ridge", Name: "Altai Ridge",
			ShortDescription: "Панорамные дома в горах с камином, спа и звёздным небом.",
			Description:      "Отдельные дома с панорамным остеклением, камином и террасой. Утром — завтрак в номере, днём — горные маршруты, вечером — спа и телескоп. Здесь не нужен будильник: свет разбудит сам.",
			Address:          "Чемальский тракт, 24", Image: "/assets/images/hotel-10.jpg",
			Gallery: []string{"/assets/images/hotel-10.jpg", "/assets/images/hotel-01.jpg", "/assets/images/hotel-06.jpg"},
			Rating:  9.6, Reviews: 327, Price: 15600,
			Tags:      []string{"горы", "природа", "спа"},
			Amenities: []string{"Wi-Fi", "Камин", "SPA", "Завтрак", "Терраса", "Трансфер", "Парковка"},
			Featured:  true,
		},
	}

	pois := []seedPOI{
		{DestinationSlug: "moscow", Name: "Kilo Coffee", Category: "coffee", Description: "Тихая спешелти-кофейня с фильтром, сезонным меню и столиками у окна. До 11:00 здесь почти нет туристов.", Address: "Столярный переулок, 6", Distance: "6 мин пешком", Tags: []string{"кофе", "тишина", "без толпы"}, PriceLevel: 2},
		{DestinationSlug: "moscow", Name: "Кооператив «Чёрный»", Category: "coffee", Description: "Локальная обжарка, большие окна и места у подоконника. Хорошо для медленного утра с книгой.", Address: "Никитский бульвар, 12", Distance: "12 мин пешком", Tags: []string{"кофе", "обжарка", "центр"}, PriceLevel: 2},
		{DestinationSlug: "moscow", Name: "Surf Coffee Патриаршие", Category: "coffee", Description: "Спешелти-кофе и много мест у окна. Приходите до 10:30, пока на Патриарших ещё тихо.", Address: "Малая Бронная, 24", Distance: "9 мин пешком", Tags: []string{"кофе", "патриаршие", "утро"}, PriceLevel: 2},
		{DestinationSlug: "moscow", Name: "Северяне", Category: "food", Description: "Сезонная русская кухня и спокойный свет. Бронируйте стол у окна — вечером здесь особенно хорошо.", Address: "Столешников переулок, 7", Distance: "8 мин пешком", Tags: []string{"ужин", "локальная кухня", "для двоих"}, PriceLevel: 3},
		{DestinationSlug: "moscow", Name: "Парк Горького", Category: "walk", Description: "Прогулка вдоль Москвы-реки, сад и длинная набережная. Лучше всего заходить со стороны Крымского Вала.", Address: "Крымский Вал, 9", Distance: "14 мин на такси", Tags: []string{"прогулка", "парк", "река"}, PriceLevel: 1},
		{DestinationSlug: "saint-petersburg", Name: "Coffee Room", Category: "coffee", Description: "Классическая петербургская кофейня с собственной обжаркой и тихими столиками на втором этаже.", Address: "Невский проспект, 88", Distance: "5 мин пешком", Tags: []string{"кофе", "тишина", "центр"}, PriceLevel: 2},
		{DestinationSlug: "saint-petersburg", Name: "Больше кофе", Category: "coffee", Description: "Небольшая кофейня у канала с фильтром, каскарой и очень внимательными бариста.", Address: "Набережная реки Мойки, 42", Distance: "7 мин пешком", Tags: []string{"кофе", "спешелти"}, PriceLevel: 2},
		{DestinationSlug: "saint-petersburg", Name: "Летний сад", Category: "walk", Description: "Тихая прогулка среди старых деревьев и скульптур. Утром здесь почти никого.", Address: "Летний сад", Distance: "11 мин пешком", Tags: []string{"прогулка", "парк", "тишина"}, PriceLevel: 1},
		{DestinationSlug: "saint-petersburg", Name: "Эрмитаж", Category: "culture", Description: "Главный музей города. Начните с залов на втором этаже и оставьте время на тихий внутренний двор.", Address: "Дворцовая площадь, 2", Distance: "12 мин пешком", Tags: []string{"музей", "культура", "архитектура"}, PriceLevel: 2},
		{DestinationSlug: "sochi", Name: "Кофе и море", Category: "coffee", Description: "Маленькая кофейня на набережной с видом на воду. Берите фильтр и место у открытого окна.", Address: "Приморская набережная, 8", Distance: "3 мин пешком", Tags: []string{"кофе", "море", "вид"}, PriceLevel: 2},
		{DestinationSlug: "sochi", Name: "Синий сад", Category: "food", Description: "Сезонные морепродукты, овощи с огня и длинные завтраки до 14:00.", Address: "Черноморская, 3", Distance: "8 мин пешком", Tags: []string{"завтрак", "морепродукты", "терраса"}, PriceLevel: 3},
		{DestinationSlug: "sochi", Name: "Приморская набережная", Category: "walk", Description: "Шесть километров у моря: пальмы, велодорожка и места, где можно сесть прямо у воды.", Address: "Приморская набережная", Distance: "2 мин пешком", Tags: []string{"море", "прогулка", "закат"}, PriceLevel: 1},
		{DestinationSlug: "sochi", Name: "Скайпарк", Category: "culture", Description: "Смотровая площадка и мосты над ущельем. Лучшее время — за час до заката.", Address: "Красная Поляна", Distance: "35 мин на такси", Tags: []string{"активность", "виды", "горы"}, PriceLevel: 3},
		{DestinationSlug: "kazan", Name: "Big Coffee", Category: "coffee", Description: "Просторная кофейня в центре с обжаркой на месте и большим столом для ноутбука.", Address: "Профсоюзная, 15", Distance: "4 мин пешком", Tags: []string{"кофе", "работа", "центр"}, PriceLevel: 2},
		{DestinationSlug: "kazan", Name: "Артель", Category: "food", Description: "Современная татарская кухня: эчпочмак, кыстыбый и локальные сезонные продукты.", Address: "Баумана, 36", Distance: "9 мин пешком", Tags: []string{"гастрономия", "локальная кухня"}, PriceLevel: 2},
		{DestinationSlug: "kazan", Name: "Набережная Кабана", Category: "walk", Description: "Вода, огни и длинная прогулочная линия. Особенно хорошо после заката.", Address: "Озеро Кабан", Distance: "6 мин пешком", Tags: []string{"прогулка", "вечер", "вид"}, PriceLevel: 1},
		{DestinationSlug: "kazan", Name: "Казанский Кремль", Category: "culture", Description: "Белая стена, мечеть и музеи внутри. На прогулку закладывайте минимум два часа.", Address: "Кремль", Distance: "12 мин пешком", Tags: []string{"культура", "архитектура", "история"}, PriceLevel: 1},
		{DestinationSlug: "kaliningrad", Name: "Coffee Cube", Category: "coffee", Description: "Минималистичная кофейня с локальной обжаркой и очень тихим внутренним двориком.", Address: "Улица Черняховского, 6", Distance: "5 мин пешком", Tags: []string{"кофе", "тишина", "двор"}, PriceLevel: 2},
		{DestinationSlug: "kaliningrad", Name: "Балтийский завтрак", Category: "food", Description: "Копчёная рыба, сыры и свежий хлеб. Завтраки подают до полудня.", Address: "Эпроновская, 8", Distance: "4 мин пешком", Tags: []string{"завтрак", "балтика", "локальное"}, PriceLevel: 2},
		{DestinationSlug: "kaliningrad", Name: "Рыбная деревня", Category: "walk", Description: "Набережная с видом на реку и городские крыши. Хороший маршрут на вечер.", Address: "Рыбная деревня", Distance: "10 мин пешком", Tags: []string{"прогулка", "река", "вид"}, PriceLevel: 1},
		{DestinationSlug: "kaliningrad", Name: "Кафедральный собор", Category: "culture", Description: "Главный символ города на острове Канта. Внутри — орган и очень тихие залы.", Address: "Остров Канта", Distance: "9 мин пешком", Tags: []string{"культура", "история", "орган"}, PriceLevel: 1},
		{DestinationSlug: "altai", Name: "Кофе у гор", Category: "coffee", Description: "Небольшая кофейня с видом на хребет и печью. Лучшее место для утреннего фильтра.", Address: "Чемальский тракт, 18", Distance: "4 мин пешком", Tags: []string{"кофе", "горы", "тишина"}, PriceLevel: 2},
		{DestinationSlug: "altai", Name: "Алтайская кухня", Category: "food", Description: "Мясо на огне, кедровые орехи и локальные травы. Ужин подают у большого окна.", Address: "Чемальский тракт, 22", Distance: "6 мин пешком", Tags: []string{"ужин", "локальная кухня"}, PriceLevel: 2},
		{DestinationSlug: "altai", Name: "Тропа здоровья", Category: "walk", Description: "Лёгкий маршрут вдоль реки с несколькими смотровыми площадками. Подходит для любого уровня.", Address: "Чемальский район", Distance: "8 мин пешком", Tags: []string{"прогулка", "горы", "природа"}, PriceLevel: 1},
		{DestinationSlug: "altai", Name: "Спа у реки", Category: "spa", Description: "Массаж с кедровым маслом и баня с видом на воду. Лучше бронировать за день.", Address: "Чемальский тракт, 30", Distance: "7 мин пешком", Tags: []string{"спа", "массаж", "баня"}, PriceLevel: 3},
	}

	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	destinationIDs := make(map[string]int64)
	for index, destination := range destinations {
		result, err := tx.ExecContext(ctx,
			`INSERT INTO destinations
			 (slug, city, country, tagline, description, image, price_from, tags_json, aliases_json, sort_order)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			destination.Slug, destination.City, destination.Country, destination.Tagline,
			destination.Description, destination.Image, destination.PriceFrom,
			encodeStrings(destination.Tags), encodeStrings(destination.Aliases), index,
		)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		destinationIDs[destination.Slug] = id
	}

	for index, hotel := range hotels {
		result, err := tx.ExecContext(ctx,
			`INSERT INTO hotels
			 (destination_id, slug, name, short_description, description, address, image, gallery_json,
			  rating, reviews_count, price_per_night, vibe_tags_json, amenities_json, featured, sort_order)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			destinationIDs[hotel.DestinationSlug], hotel.Slug, hotel.Name, hotel.ShortDescription,
			hotel.Description, hotel.Address, hotel.Image, encodeStrings(hotel.Gallery),
			hotel.Rating, hotel.Reviews, hotel.Price, encodeStrings(hotel.Tags),
			encodeStrings(hotel.Amenities), boolInt(hotel.Featured), index,
		)
		if err != nil {
			return err
		}
		hotelID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		if err := insertRooms(ctx, tx, hotelID, hotel); err != nil {
			return err
		}
	}

	for index, poi := range pois {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO pois
			 (destination_slug, name, category, description, address, distance, tags_json, price_level, sort_order)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			poi.DestinationSlug, poi.Name, poi.Category, poi.Description,
			poi.Address, poi.Distance, encodeStrings(poi.Tags), poi.PriceLevel, index,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func insertRooms(ctx context.Context, tx *sql.Tx, hotelID int64, hotel seedHotel) error {
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
		price := int(math.Round(float64(hotel.Price)*room.Factor/100)) * 100
		image := hotel.Gallery[index%len(hotel.Gallery)]
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO rooms
			 (hotel_id, name, description, price, capacity, bed, size, amenities_json, image, total_units)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			hotelID, room.Name, room.Description, price, room.Capacity, room.Bed, room.Size,
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
